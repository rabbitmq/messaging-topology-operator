package controllers_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/rabbitmq/cluster-operator/v2/api/v1beta1"
	"github.com/rabbitmq/messaging-topology-operator/controllers"
	"io"
	"k8s.io/apimachinery/pkg/labels"
	"net/http"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	runtimeClient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/config"
	"sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	topology "github.com/rabbitmq/messaging-topology-operator/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("bindingController", func() {
	var (
		binding       topology.Binding
		bindingName   string
		bindingMgr    ctrl.Manager
		managerCtx    context.Context
		managerCancel context.CancelFunc
		k8sClient     runtimeClient.Client
	)

	initialiseManager := func(keyValPair ...string) {
		var sel labels.Selector
		if len(keyValPair) == 2 {
			var err error
			sel, err = labels.Parse(fmt.Sprintf("%s == %s", keyValPair[0], keyValPair[1]))
			Expect(err).NotTo(HaveOccurred())
		}

		var err error
		bindingMgr, err = ctrl.NewManager(testEnv.Config, ctrl.Options{
			Metrics: server.Options{
				BindAddress: "0", // To avoid MacOS firewall pop-up every time you run this suite
			},
			Cache: cache.Options{
				DefaultNamespaces: map[string]cache.Config{bindingNamespace: {
					LabelSelector: sel,
				}},
				ByObject: map[runtimeClient.Object]cache.ByObject{
					&v1beta1.RabbitmqCluster{}: {Namespaces: map[string]cache.Config{cache.AllNamespaces: {}}},
					&corev1.Secret{}:           {Namespaces: map[string]cache.Config{cache.AllNamespaces: {}}},
					&corev1.Service{}:          {Namespaces: map[string]cache.Config{cache.AllNamespaces: {}}},
				},
			},
			Logger: GinkgoLogr,
			Controller: config.Controller{
				SkipNameValidation: &skipNameValidation,
			},
		})
		Expect(err).ToNot(HaveOccurred())

		managerCtx, managerCancel = context.WithCancel(context.Background())
		go func(ctx context.Context) {
			defer GinkgoRecover()
			Expect(bindingMgr.Start(ctx)).To(Succeed())
		}(managerCtx)

		k8sClient = bindingMgr.GetClient()

		Expect((&controllers.TopologyReconciler{
			Client:                bindingMgr.GetClient(),
			Type:                  &topology.Binding{},
			Scheme:                bindingMgr.GetScheme(),
			Recorder:              fakeRecorder,
			RabbitmqClientFactory: fakeRabbitMQClientFactory,
			ReconcileFunc:         &controllers.BindingReconciler{},
		}).SetupWithManager(bindingMgr)).To(Succeed())
	}

	initialiseBinding := func() {
		binding = topology.Binding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      bindingName,
				Namespace: bindingNamespace,
			},
			Spec: topology.BindingSpec{
				RabbitmqClusterReference: topology.RabbitmqClusterReference{
					Name: "example-rabbit",
				},
			},
		}
	}

	AfterEach(func() {
		managerCancel()
		// Sad workaround to avoid controllers racing for the reconciliation of other's
		// test cases. Without this wait, the last run test consistently fails because
		// the previous cancelled manager is just in time to reconcile the Queue of the
		// new/last test, and use the wrong/unexpected arguments in the queue declare call
		//
		// Eventual consistency is nice when you have good means of awaiting. That's not the
		// case with testenv and kubernetes controllers.
		<-time.After(time.Second)
	})

	When("creating a binding", func() {
		AfterEach(func() {
			Expect(k8sClient.Delete(ctx, &binding)).To(Succeed())
		})

		When("the RabbitMQ Client returns a HTTP error response", func() {
			BeforeEach(func() {
				bindingName = "test-binding-http-error"
				fakeRabbitMQClient.DeclareBindingReturns(&http.Response{
					Status:     "418 I'm a teapot",
					StatusCode: 418,
				}, errors.New("some HTTP error"))
				initialiseBinding()
				binding.Labels = map[string]string{"test": "test-binding-http-error"}
				initialiseManager("test", "test-binding-http-error")
			})

			It("sets the status condition to indicate a failure to reconcile", func() {
				Expect(k8sClient.Create(ctx, &binding)).To(Succeed())
				EventuallyWithOffset(1, func() []topology.Condition {
					_ = k8sClient.Get(
						ctx,
						types.NamespacedName{Name: binding.Name, Namespace: binding.Namespace},
						&binding,
					)

					return binding.Status.Conditions
				}, statusEventsUpdateTimeout, 1*time.Second).Should(ContainElement(MatchFields(IgnoreExtras, Fields{
					"Type":    Equal(topology.ConditionType("Ready")),
					"Reason":  Equal("FailedCreateOrUpdate"),
					"Status":  Equal(corev1.ConditionFalse),
					"Message": ContainSubstring("some HTTP error"),
				})))
			})
		})

		When("the RabbitMQ Client returns a Go error response", func() {
			BeforeEach(func() {
				bindingName = "test-binding-go-error"
				fakeRabbitMQClient.DeclareBindingReturns(nil, errors.New("hit a exception"))
				initialiseBinding()
				binding.Labels = map[string]string{"test": "test-binding-go-error"}
				initialiseManager("test", "test-binding-go-error")
			})

			It("sets the status condition to indicate a failure to reconcile", func() {
				Expect(k8sClient.Create(ctx, &binding)).To(Succeed())
				EventuallyWithOffset(1, func() []topology.Condition {
					_ = k8sClient.Get(
						ctx,
						types.NamespacedName{Name: binding.Name, Namespace: binding.Namespace},
						&binding,
					)

					return binding.Status.Conditions
				}, statusEventsUpdateTimeout, 1*time.Second).Should(ContainElement(MatchFields(IgnoreExtras, Fields{
					"Type":    Equal(topology.ConditionType("Ready")),
					"Reason":  Equal("FailedCreateOrUpdate"),
					"Status":  Equal(corev1.ConditionFalse),
					"Message": ContainSubstring("hit a exception"),
				})))
			})
		})
	})

	When("Deleting a binding", func() {
		JustBeforeEach(func() {
			// Must use a JustBeforeEach to extract this common behaviour
			// JustBeforeEach runs AFTER all BeforeEach have completed
			fakeRabbitMQClient.DeclareBindingReturns(&http.Response{
				Status:     "201 Created",
				StatusCode: http.StatusCreated,
			}, nil)
			Expect(k8sClient.Create(ctx, &binding)).To(Succeed())
			EventuallyWithOffset(1, func() []topology.Condition {
				_ = k8sClient.Get(
					ctx,
					types.NamespacedName{Name: binding.Name, Namespace: binding.Namespace},
					&binding,
				)

				return binding.Status.Conditions
			}, statusEventsUpdateTimeout, 1*time.Second).Should(ContainElement(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(topology.ConditionType("Ready")),
				"Reason": Equal("SuccessfulCreateOrUpdate"),
				"Status": Equal(corev1.ConditionTrue),
			})))
		})

		When("the RabbitMQ Client returns a HTTP error response", func() {
			BeforeEach(func() {
				bindingName = "delete-binding-http-error"
				fakeRabbitMQClient.DeleteBindingReturns(&http.Response{
					Status:     "502 Bad Gateway",
					StatusCode: http.StatusBadGateway,
					Body:       io.NopCloser(bytes.NewBufferString("Hello World")),
				}, nil)
				initialiseBinding()
				binding.Labels = map[string]string{"test": "delete-binding-http-error"}
				initialiseManager("test", "delete-binding-http-error")
			})

			It("raises an event to indicate a failure to delete", func() {
				Expect(k8sClient.Delete(ctx, &binding)).To(Succeed())
				Consistently(func() bool {
					err := k8sClient.Get(ctx, types.NamespacedName{Name: binding.Name, Namespace: binding.Namespace}, &topology.Binding{})
					return apierrors.IsNotFound(err)
				}, statusEventsUpdateTimeout).Should(BeFalse())
				Expect(observedEvents()).To(ContainElement("Warning FailedDelete failed to delete binding"))
			})
		})

		When("the RabbitMQ Client returns a Go error response", func() {
			BeforeEach(func() {
				bindingName = "delete-binding-go-error"
				fakeRabbitMQClient.DeleteBindingReturns(nil, errors.New("some error"))
				initialiseBinding()
				binding.Labels = map[string]string{"test": "delete-binding-go-error"}
				initialiseManager("test", "delete-binding-go-error")
			})

			It("raises an event to indicate a failure to delete", func() {
				Expect(k8sClient.Delete(ctx, &binding)).To(Succeed())
				Consistently(func() bool {
					err := k8sClient.Get(ctx, types.NamespacedName{Name: binding.Name, Namespace: binding.Namespace}, &topology.Binding{})
					return apierrors.IsNotFound(err)
				}, 5).Should(BeFalse())
				Expect(observedEvents()).To(ContainElement("Warning FailedDelete failed to delete binding"))
			})
		})
	})
})
