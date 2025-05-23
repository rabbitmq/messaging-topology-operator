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

	"k8s.io/apimachinery/pkg/runtime"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	topology "github.com/rabbitmq/messaging-topology-operator/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("policy-controller", func() {
	var (
		policy        topology.Policy
		policyName    string
		policyMgr     ctrl.Manager
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
		policyMgr, err = ctrl.NewManager(testEnv.Config, ctrl.Options{
			Metrics: server.Options{
				BindAddress: "0", // To avoid MacOS firewall pop-up every time you run this suite
			},
			Cache: cache.Options{
				DefaultNamespaces: map[string]cache.Config{policyNamespace: {
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
			Expect(policyMgr.Start(ctx)).To(Succeed())
		}(managerCtx)

		k8sClient = policyMgr.GetClient()

		Expect((&controllers.TopologyReconciler{
			Client:                policyMgr.GetClient(),
			Type:                  &topology.Policy{},
			Scheme:                policyMgr.GetScheme(),
			Recorder:              fakeRecorder,
			RabbitmqClientFactory: fakeRabbitMQClientFactory,
			ReconcileFunc:         &controllers.PolicyReconciler{},
		}).SetupWithManager(policyMgr)).To(Succeed())
	}

	initialisePolicy := func() {
		policy = topology.Policy{
			ObjectMeta: metav1.ObjectMeta{
				Name:      policyName,
				Namespace: policyNamespace,
			},
			Spec: topology.PolicySpec{
				Definition: &runtime.RawExtension{
					Raw: []byte(`{"key":"value"}`),
				},
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

	Context("creation", func() {
		AfterEach(func() {
			Expect(k8sClient.Delete(ctx, &policy)).To(Succeed())
		})

		When("the RabbitMQ Client returns a HTTP error response", func() {
			BeforeEach(func() {
				policyName = "test-http-error"
				fakeRabbitMQClient.PutPolicyReturns(&http.Response{
					Status:     "418 I'm a teapot",
					StatusCode: 418,
				}, errors.New("a failure"))
				initialisePolicy()
				policy.Labels = map[string]string{"test": "test-http-error"}
				initialiseManager("test", policyName)
			})

			It("sets the status condition", func() {
				Expect(k8sClient.Create(ctx, &policy)).To(Succeed())
				Eventually(func() []topology.Condition {
					_ = k8sClient.Get(
						ctx,
						types.NamespacedName{Name: policy.Name, Namespace: policy.Namespace},
						&policy,
					)

					return policy.Status.Conditions
				}).
					Within(statusEventsUpdateTimeout).
					WithPolling(time.Second).
					Should(ContainElement(MatchFields(IgnoreExtras, Fields{
						"Type":    Equal(topology.ConditionType("Ready")),
						"Reason":  Equal("FailedCreateOrUpdate"),
						"Status":  Equal(corev1.ConditionFalse),
						"Message": ContainSubstring("a failure"),
					})))
			})
		})

		When("the RabbitMQ Client returns a Go error response", func() {
			BeforeEach(func() {
				policyName = "test-go-error"
				fakeRabbitMQClient.PutPolicyReturns(nil, errors.New("a go failure"))
				initialisePolicy()
				policy.Labels = map[string]string{"test": "test-go-error"}
				initialiseManager("test", policyName)
			})

			It("sets the status condition to indicate a failure to reconcile", func() {
				Expect(k8sClient.Create(ctx, &policy)).To(Succeed())
				Eventually(func() []topology.Condition {
					_ = k8sClient.Get(
						ctx,
						types.NamespacedName{Name: policy.Name, Namespace: policy.Namespace},
						&policy,
					)

					return policy.Status.Conditions
				}).
					Within(statusEventsUpdateTimeout).
					WithPolling(time.Second).
					Should(ContainElement(MatchFields(IgnoreExtras, Fields{
						"Type":    Equal(topology.ConditionType("Ready")),
						"Reason":  Equal("FailedCreateOrUpdate"),
						"Status":  Equal(corev1.ConditionFalse),
						"Message": ContainSubstring("a go failure"),
					})))
			})
		})
	})

	Context("deletion", func() {
		JustBeforeEach(func() {
			// Must use a JustBeforeEach to extract this common behaviour
			// JustBeforeEach runs AFTER all BeforeEach have completed
			fakeRabbitMQClient.PutPolicyReturns(&http.Response{
				Status:     "201 Created",
				StatusCode: http.StatusCreated,
			}, nil)
			Expect(k8sClient.Create(ctx, &policy)).To(Succeed())
			Eventually(func() []topology.Condition {
				_ = k8sClient.Get(
					ctx,
					types.NamespacedName{Name: policy.Name, Namespace: policy.Namespace},
					&policy,
				)

				return policy.Status.Conditions
			}).
				Within(statusEventsUpdateTimeout).
				WithPolling(time.Second).
				Should(ContainElement(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(topology.ConditionType("Ready")),
					"Reason": Equal("SuccessfulCreateOrUpdate"),
					"Status": Equal(corev1.ConditionTrue),
				})))
		})

		When("the RabbitMQ Client returns a HTTP error response", func() {
			BeforeEach(func() {
				policyName = "delete-policy-http-error"
				fakeRabbitMQClient.DeletePolicyReturns(&http.Response{
					Status:     "502 Bad Gateway",
					StatusCode: http.StatusBadGateway,
					Body:       io.NopCloser(bytes.NewBufferString("Hello World")),
				}, nil)
				initialisePolicy()
				policy.Labels = map[string]string{"test": "delete-policy-http-error"}
				initialiseManager("test", policyName)
			})

			It("publishes a 'warning' event", func() {
				Expect(k8sClient.Delete(ctx, &policy)).To(Succeed())
				Consistently(func() bool {
					err := k8sClient.Get(ctx, types.NamespacedName{Name: policy.Name, Namespace: policy.Namespace}, &topology.Policy{})
					return apierrors.IsNotFound(err)
				}).
					Within(statusEventsUpdateTimeout).
					Should(BeFalse())
				Expect(observedEvents()).To(ContainElement("Warning FailedDelete failed to delete policy"))
			})
		})

		When("the RabbitMQ Client returns a Go error response", func() {
			BeforeEach(func() {
				policyName = "delete-go-error"
				fakeRabbitMQClient.DeletePolicyReturns(nil, errors.New("some error"))
				initialisePolicy()
				policy.Labels = map[string]string{"test": "delete-go-error"}
				initialiseManager("test", policyName)
			})

			It("publishes a 'warning' event", func() {
				Expect(k8sClient.Delete(ctx, &policy)).To(Succeed())
				Consistently(func() bool {
					err := k8sClient.Get(ctx, types.NamespacedName{Name: policy.Name, Namespace: policy.Namespace}, &topology.Policy{})
					return apierrors.IsNotFound(err)
				}).
					Within(statusEventsUpdateTimeout).
					Should(BeFalse())
				Expect(observedEvents()).To(ContainElement("Warning FailedDelete failed to delete policy"))
			})
		})
	})
})
