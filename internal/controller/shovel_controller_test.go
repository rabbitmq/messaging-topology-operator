package controller_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/rabbitmq/cluster-operator/v2/api/v1beta1"
	"io"
	"k8s.io/apimachinery/pkg/labels"
	"net/http"
	"time"

	"github.com/rabbitmq/messaging-topology-operator/internal/controller"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	runtimeClient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/config"
	"sigs.k8s.io/controller-runtime/pkg/envtest/komega"
	"sigs.k8s.io/controller-runtime/pkg/metrics/server"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	topology "github.com/rabbitmq/messaging-topology-operator/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("shovel-controller", func() {
	var (
		shovel        topology.Shovel
		shovelName    string
		shovelMgr     ctrl.Manager
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
		shovelMgr, err = ctrl.NewManager(testEnv.Config, ctrl.Options{
			Metrics: server.Options{
				BindAddress: "0", // To avoid MacOS firewall pop-up every time you run this suite
			},
			Cache: cache.Options{
				DefaultNamespaces: map[string]cache.Config{shovelNamespace: {
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
			Expect(shovelMgr.Start(ctx)).To(Succeed())
		}(managerCtx)

		k8sClient = shovelMgr.GetClient()

		Expect((&controller.TopologyReconciler{
			Client:                shovelMgr.GetClient(),
			Type:                  &topology.Shovel{},
			Scheme:                shovelMgr.GetScheme(),
			Recorder:              fakeRecorder,
			RabbitmqClientFactory: fakeRabbitMQClientFactory,
			ReconcileFunc:         &controller.ShovelReconciler{Client: shovelMgr.GetClient()},
		}).SetupWithManager(shovelMgr)).To(Succeed())
	}

	initialiseShovel := func() {
		shovel = topology.Shovel{
			ObjectMeta: metav1.ObjectMeta{
				Name:      shovelName,
				Namespace: shovelNamespace,
			},
			Spec: topology.ShovelSpec{
				Name:      "my-shovel-configuration",
				Vhost:     "/test",
				UriSecret: &corev1.LocalObjectReference{Name: "shovel-uri-secret"},
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
			Expect(k8sClient.Delete(ctx, &shovel)).To(Succeed())
		})

		When("the RabbitMQ Client returns a HTTP error response", func() {
			BeforeEach(func() {
				shovelName = "test-shovel-http-error"
				fakeRabbitMQClient.DeclareShovelReturns(&http.Response{
					Status:     "418 I'm a teapot",
					StatusCode: 418,
				}, errors.New("some HTTP error"))
				initialiseShovel()
				shovel.Labels = map[string]string{"test": "test-shovel-http-error"}
				initialiseManager("test", "test-shovel-http-error")
			})

			It("sets the status condition to indicate a failure to reconcile", func() {
				Expect(k8sClient.Create(ctx, &shovel)).To(Succeed())
				Eventually(func() []topology.Condition {
					_ = k8sClient.Get(
						ctx,
						types.NamespacedName{Name: shovel.Name, Namespace: shovel.Namespace},
						&shovel,
					)

					return shovel.Status.Conditions
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
				shovelName = "test-shovel-go-error"
				fakeRabbitMQClient.DeclareShovelReturns(nil, errors.New("a go failure"))
				initialiseShovel()
				shovel.Labels = map[string]string{"test": "test-shovel-go-error"}
				initialiseManager("test", "test-shovel-go-error")
			})

			It("sets the status condition to indicate a failure to reconcile", func() {
				Expect(k8sClient.Create(ctx, &shovel)).To(Succeed())
				Eventually(func() []topology.Condition {
					_ = k8sClient.Get(
						ctx,
						types.NamespacedName{Name: shovel.Name, Namespace: shovel.Namespace},
						&shovel,
					)

					return shovel.Status.Conditions
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

		Context("success", func() {
			BeforeEach(func() {
				shovelName = "test-shovel-success"
				fakeRabbitMQClient.DeclareShovelReturns(&http.Response{
					Status:     "201 Created",
					StatusCode: http.StatusCreated,
				}, nil)
				initialiseShovel()
				shovel.Labels = map[string]string{"test": "test-shovel-success"}
				initialiseManager("test", "test-shovel-success")
			})

			It("works", func() {
				Expect(k8sClient.Create(ctx, &shovel)).To(Succeed())
				By("setting the correct finalizer")
				Eventually(komega.Object(&shovel)).WithTimeout(2 * time.Second).Should(HaveField("ObjectMeta.Finalizers", ConsistOf("deletion.finalizers.shovels.rabbitmq.com")))

				By("sets the status condition 'Ready' to 'true'")
				// TODO rewrite this using komega.Object
				Eventually(func() []topology.Condition {
					_ = k8sClient.Get(
						ctx,
						types.NamespacedName{Name: shovel.Name, Namespace: shovel.Namespace},
						&shovel,
					)

					return shovel.Status.Conditions
				}).
					Within(statusEventsUpdateTimeout).
					WithPolling(time.Second).
					Should(ContainElement(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(topology.ConditionType("Ready")),
						"Reason": Equal("SuccessfulCreateOrUpdate"),
						"Status": Equal(corev1.ConditionTrue),
					})))
			})
		})
	})

	When("deletion", func() {
		JustBeforeEach(func() {
			// Must use a JustBeforeEach to extract this common behaviour
			// JustBeforeEach runs AFTER all BeforeEach have completed
			fakeRabbitMQClient.DeclareShovelReturns(&http.Response{
				Status:     "201 Created",
				StatusCode: http.StatusCreated,
			}, nil)
			Expect(k8sClient.Create(ctx, &shovel)).To(Succeed())
			Eventually(func() []topology.Condition {
				_ = k8sClient.Get(
					ctx,
					types.NamespacedName{Name: shovel.Name, Namespace: shovel.Namespace},
					&shovel,
				)

				return shovel.Status.Conditions
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
				shovelName = "delete-shovel-http-error"
				fakeRabbitMQClient.DeleteShovelReturns(&http.Response{
					Status:     "502 Bad Gateway",
					StatusCode: http.StatusBadGateway,
					Body:       io.NopCloser(bytes.NewBufferString("Hello World")),
				}, nil)
				initialiseShovel()
				shovel.Labels = map[string]string{"test": "delete-shovel-http-error"}
				initialiseManager("test", "delete-shovel-http-error")
			})

			It("raises an event to indicate a failure to delete", func() {
				Expect(k8sClient.Delete(ctx, &shovel)).To(Succeed())
				Consistently(func() bool {
					err := k8sClient.Get(ctx, types.NamespacedName{Name: shovel.Name, Namespace: shovel.Namespace}, &topology.Shovel{})
					return apierrors.IsNotFound(err)
				}).
					Within(statusEventsUpdateTimeout).
					WithPolling(time.Second).
					Should(BeFalse())
				Expect(observedEvents()).To(ContainElement("Warning FailedDelete failed to delete shovel"))
			})
		})

		When("the RabbitMQ Client returns a Go error response", func() {
			BeforeEach(func() {
				shovelName = "delete-shovel-go-error"
				fakeRabbitMQClient.DeleteShovelReturns(nil, errors.New("some error"))
				initialiseShovel()
				shovel.Labels = map[string]string{"test": "delete-shovel-go-error"}
				initialiseManager("test", "delete-shovel-go-error")
			})

			It("publishes a 'warning' event", func() {
				Expect(k8sClient.Delete(ctx, &shovel)).To(Succeed())
				Consistently(func() bool {
					err := k8sClient.Get(ctx, types.NamespacedName{Name: shovel.Name, Namespace: shovel.Namespace}, &topology.Shovel{})
					return apierrors.IsNotFound(err)
				}).
					Within(statusEventsUpdateTimeout).
					WithPolling(time.Second).
					Should(BeFalse())
				Expect(observedEvents()).To(ContainElement("Warning FailedDelete failed to delete shovel"))
			})
		})

		Context("success", func() {
			BeforeEach(func() {
				shovelName = "delete-shovel-success"
				fakeRabbitMQClient.DeleteShovelReturns(&http.Response{
					Status:     "204 No Content",
					StatusCode: http.StatusNoContent,
				}, nil)
				initialiseShovel()
				shovel.Labels = map[string]string{"test": "delete-shovel-success"}
				initialiseManager("test", "delete-shovel-success")
			})

			It("publishes a normal event", func() {
				Expect(k8sClient.Delete(ctx, &shovel)).To(Succeed())
				Eventually(func() bool {
					err := k8sClient.Get(ctx, types.NamespacedName{Name: shovel.Name, Namespace: shovel.Namespace}, &topology.Shovel{})
					return apierrors.IsNotFound(err)
				}).
					Within(statusEventsUpdateTimeout).
					WithPolling(time.Second).
					Should(BeTrue())
				Expect(observedEvents()).To(SatisfyAll(
					Not(ContainElement("Warning FailedDelete failed to delete shovel")),
					ContainElement("Normal SuccessfulDelete successfully deleted shovel"),
				))
			})
		})
	})

	When("the Shovel has DeletionPolicy set to retain", func() {
		BeforeEach(func() {
			shovelName = "shovel-with-retain-policy"
			fakeRabbitMQClient.DeleteShovelReturns(&http.Response{
				Status:     "200 OK",
				StatusCode: http.StatusOK,
			}, nil)
			fakeRabbitMQClient.DeclareShovelReturns(&http.Response{StatusCode: http.StatusCreated, Status: "201 Created"}, nil)
			initialiseShovel()
			shovel.Labels = map[string]string{"test": "shovel-with-retain-policy"}
			initialiseManager("test", "shovel-with-retain-policy")
		})

		It("deletes the k8s resource but preserves the shovel in RabbitMQ server", func() {
			shovel.Spec.DeletionPolicy = "retain"
			Expect(k8sClient.Create(ctx, &shovel)).To(Succeed())
			Eventually(fakeRabbitMQClient.DeclareShovelCallCount).
				WithPolling(time.Second).
				Within(time.Second*3).
				Should(BeNumerically(">=", 1), "Expected to call RMQ API to declare shovel")
			Expect(k8sClient.Delete(ctx, &shovel)).To(Succeed())

			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: shovel.Name, Namespace: shovel.Namespace}, &shovel)
				return apierrors.IsNotFound(err)
			}).
				Within(statusEventsUpdateTimeout).
				WithPolling(time.Second).
				Should(BeTrue(), "Expected shovel to not be found")

			Expect(fakeRabbitMQClient.DeleteShovelCallCount()).To(Equal(0), "Shovel object should have been deleted and no calls to RMQ API")
		})
	})
})
