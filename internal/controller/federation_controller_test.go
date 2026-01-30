package controller_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/rabbitmq/cluster-operator/v2/api/v1beta1"
	"github.com/rabbitmq/messaging-topology-operator/internal/controller"
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

var _ = Describe("federation-controller", func() {
	var (
		federation     topology.Federation
		federationName string
		federationMgr  ctrl.Manager
		managerCtx     context.Context
		managerCancel  context.CancelFunc
		k8sClient      runtimeClient.Client
	)

	initialiseManager := func(keyValPair ...string) {
		var sel labels.Selector
		if len(keyValPair) == 2 {
			var err error
			sel, err = labels.Parse(fmt.Sprintf("%s == %s", keyValPair[0], keyValPair[1]))
			Expect(err).NotTo(HaveOccurred())
		}

		var err error
		federationMgr, err = ctrl.NewManager(testEnv.Config, ctrl.Options{
			Metrics: server.Options{
				BindAddress: "0", // To avoid MacOS firewall pop-up every time you run this suite
			},
			Cache: cache.Options{
				DefaultNamespaces: map[string]cache.Config{federationNamespace: {
					LabelSelector: sel,
				}},
				ByObject: map[runtimeClient.Object]cache.ByObject{
					// Not sure why, but restricting the NS to the test-ns fails the tests :shrug:
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
			Expect(federationMgr.Start(ctx)).To(Succeed())
		}(managerCtx)

		k8sClient = federationMgr.GetClient()

		Expect((&controller.TopologyReconciler{
			Client:                federationMgr.GetClient(),
			Type:                  &topology.Federation{},
			Scheme:                federationMgr.GetScheme(),
			Recorder:              fakeRecorder,
			RabbitmqClientFactory: fakeRabbitMQClientFactory,
			ReconcileFunc:         &controller.FederationReconciler{Client: federationMgr.GetClient()},
		}).SetupWithManager(federationMgr)).To(Succeed())
	}

	initialiseFederation := func() {
		federation = topology.Federation{
			ObjectMeta: metav1.ObjectMeta{
				Name:      federationName,
				Namespace: federationNamespace,
			},
			Spec: topology.FederationSpec{
				Name:      "my-federation-upstream",
				Vhost:     "/test",
				UriSecret: &corev1.LocalObjectReference{Name: "federation-uri"},
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
		// case with testenv and kubernetes controller.
		<-time.After(time.Second)
	})

	When("creation", func() {
		AfterEach(func() {
			Expect(k8sClient.Delete(ctx, &federation)).To(Succeed())
		})

		When("the RabbitMQ Client returns a HTTP error response", func() {
			BeforeEach(func() {
				federationName = "test-federation-http-error"
				fakeRabbitMQClient.PutFederationUpstreamReturns(&http.Response{
					Status:     "418 I'm a teapot",
					StatusCode: 418,
				}, errors.New("some HTTP error"))
				initialiseFederation()
				federation.Labels = map[string]string{"test": "test-federation-http-error"}
				initialiseManager("test", "test-federation-http-error")
			})

			It("sets the status condition to indicate a failure to reconcile", func() {
				Expect(k8sClient.Create(ctx, &federation)).To(Succeed())
				Eventually(func() []topology.Condition {
					_ = k8sClient.Get(
						ctx,
						types.NamespacedName{Name: federation.Name, Namespace: federation.Namespace},
						&federation,
					)

					return federation.Status.Conditions
				}).
					Within(statusEventsUpdateTimeout).
					WithPolling(time.Second).
					Should(ContainElement(MatchFields(IgnoreExtras, Fields{
						"Type":    Equal(topology.ConditionType("Ready")),
						"Reason":  Equal("FailedCreateOrUpdate"),
						"Status":  Equal(corev1.ConditionFalse),
						"Message": ContainSubstring("some HTTP error"),
					})))
			})
		})

		When("the RabbitMQ Client returns a Go error response", func() {
			BeforeEach(func() {
				federationName = "test-federation-go-error"
				fakeRabbitMQClient.PutFederationUpstreamReturns(nil, errors.New("some go failure here"))
				initialiseFederation()
				federation.Labels = map[string]string{"test": "test-federation-go-error"}
				initialiseManager("test", "test-federation-go-error")
			})

			It("sets the status condition to indicate a failure to reconcile", func() {
				Expect(k8sClient.Create(ctx, &federation)).To(Succeed())
				Eventually(func() []topology.Condition {
					_ = k8sClient.Get(
						ctx,
						types.NamespacedName{Name: federation.Name, Namespace: federation.Namespace},
						&federation,
					)

					return federation.Status.Conditions
				}).
					Within(statusEventsUpdateTimeout).
					WithPolling(time.Second).
					Should(ContainElement(MatchFields(IgnoreExtras, Fields{
						"Type":    Equal(topology.ConditionType("Ready")),
						"Reason":  Equal("FailedCreateOrUpdate"),
						"Status":  Equal(corev1.ConditionFalse),
						"Message": ContainSubstring("some go failure here"),
					})))
			})
		})
	})

	When("deletion", func() {
		JustBeforeEach(func() {
			// Must use a JustBeforeEach to extract this common behaviour
			// JustBeforeEach runs AFTER all BeforeEach have completed
			fakeRabbitMQClient.PutFederationUpstreamReturns(&http.Response{
				Status:     "201 Created",
				StatusCode: http.StatusCreated,
			}, nil)
			Expect(k8sClient.Create(ctx, &federation)).To(Succeed())
			Eventually(func() []topology.Condition {
				_ = k8sClient.Get(
					ctx,
					types.NamespacedName{Name: federation.Name, Namespace: federation.Namespace},
					&federation,
				)

				return federation.Status.Conditions
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
				federationName = "delete-federation-http-error"
				fakeRabbitMQClient.DeleteFederationUpstreamReturns(&http.Response{
					Status:     "502 Bad Gateway",
					StatusCode: http.StatusBadGateway,
					Body:       io.NopCloser(bytes.NewBufferString("Hello World")),
				}, nil)
				initialiseFederation()
				federation.Labels = map[string]string{"test": "delete-federation-http-error"}
				initialiseManager("test", "delete-federation-http-error")
			})

			It("raises an event to indicate a failure to delete", func() {
				Expect(k8sClient.Delete(ctx, &federation)).To(Succeed())
				Consistently(func() bool {
					err := k8sClient.Get(ctx, types.NamespacedName{Name: federation.Name, Namespace: federation.Namespace}, &topology.Federation{})
					return apierrors.IsNotFound(err)
				}).
					Within(statusEventsUpdateTimeout).
					WithPolling(time.Second).
					Should(BeFalse())
				Expect(observedEvents()).To(ContainElement("Warning FailedDelete failed to delete federation"))
			})
		})

		When("the RabbitMQ Client returns a Go error response", func() {
			BeforeEach(func() {
				federationName = "delete-federation-go-error"
				fakeRabbitMQClient.DeleteFederationUpstreamReturns(nil, errors.New("some error"))
				initialiseFederation()
				federation.Labels = map[string]string{"test": "delete-federation-go-error"}
				initialiseManager("test", "delete-federation-go-error")
			})

			It("publishes a 'warning' event", func() {
				Expect(k8sClient.Delete(ctx, &federation)).To(Succeed())
				Consistently(func() bool {
					err := k8sClient.Get(ctx, types.NamespacedName{Name: federation.Name, Namespace: federation.Namespace}, &topology.Federation{})
					return apierrors.IsNotFound(err)
				}).
					Within(statusEventsUpdateTimeout).
					WithPolling(time.Second).
					Should(BeFalse())
				Expect(observedEvents()).To(ContainElement("Warning FailedDelete failed to delete federation"))
			})
		})
	})

	When("the Federation has DeletionPolicy set to retain", func() {
		BeforeEach(func() {
			federationName = "federation-with-retain-policy"
			fakeRabbitMQClient.DeleteFederationUpstreamReturns(&http.Response{
				Status:     "200 OK",
				StatusCode: http.StatusOK,
			}, nil)
			fakeRabbitMQClient.PutFederationUpstreamReturns(&http.Response{StatusCode: http.StatusCreated, Status: "201 Created"}, nil)
			initialiseFederation()
			federation.Labels = map[string]string{"test": "federation-with-retain-policy"}
			initialiseManager("test", "federation-with-retain-policy")
		})

		It("deletes the k8s resource but preserves the federation in RabbitMQ server", func() {
			federation.Spec.DeletionPolicy = "retain"
			Expect(k8sClient.Create(ctx, &federation)).To(Succeed())
			Eventually(fakeRabbitMQClient.PutFederationUpstreamCallCount).
				WithPolling(time.Second).
				Within(time.Second*3).
				Should(BeNumerically(">=", 1), "Expected to call RMQ API to create federation")

			Expect(k8sClient.Delete(ctx, &federation)).To(Succeed())
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: federation.Name, Namespace: federation.Namespace}, &federation)
				return apierrors.IsNotFound(err)
			}).
				Within(statusEventsUpdateTimeout).
				WithPolling(time.Second).
				Should(BeTrue(), "Federation should not be found")

			Expect(fakeRabbitMQClient.DeleteFederationUpstreamCallCount()).To(Equal(0), "Expected Federation to be deleted and no calls to RMQ API")
		})
	})
})
