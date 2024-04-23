package controllers_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/rabbitmq/messaging-topology-operator/controllers"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	runtimeClient "sigs.k8s.io/controller-runtime/pkg/client"
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

var _ = Describe("exchange-controller", func() {
	var (
		exchange      topology.Exchange
		exchangeName  string
		exchangeMgr   ctrl.Manager
		managerCtx    context.Context
		managerCancel context.CancelFunc
		k8sClient     runtimeClient.Client
	)

	BeforeEach(func() {
		var err error
		exchangeMgr, err = ctrl.NewManager(testEnv.Config, ctrl.Options{
			Metrics: server.Options{
				BindAddress: "0", // To avoid MacOS firewall pop-up every time you run this suite
			},
			Cache: cache.Options{
				DefaultNamespaces: map[string]cache.Config{exchangeNamespace: {}},
			},
			Logger: GinkgoLogr,
		})
		Expect(err).ToNot(HaveOccurred())

		managerCtx, managerCancel = context.WithCancel(context.Background())
		go func(ctx context.Context) {
			defer GinkgoRecover()
			Expect(exchangeMgr.Start(ctx)).To(Succeed())
		}(managerCtx)

		k8sClient = exchangeMgr.GetClient()

		Expect((&controllers.TopologyReconciler{
			Client:                exchangeMgr.GetClient(),
			Type:                  &topology.Exchange{},
			Scheme:                exchangeMgr.GetScheme(),
			Recorder:              fakeRecorder,
			RabbitmqClientFactory: fakeRabbitMQClientFactory,
			ReconcileFunc:         &controllers.ExchangeReconciler{},
		}).SetupWithManager(exchangeMgr)).To(Succeed())
	})

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

	JustBeforeEach(func() {
		// this will be executed after all BeforeEach have run
		exchange = topology.Exchange{
			ObjectMeta: metav1.ObjectMeta{
				Name:      exchangeName,
				Namespace: exchangeNamespace,
			},
			Spec: topology.ExchangeSpec{
				RabbitmqClusterReference: topology.RabbitmqClusterReference{
					Name: "example-rabbit",
				},
			},
		}
	})

	Context("creation", func() {
		When("the RabbitMQ Client returns a HTTP error response", func() {
			BeforeEach(func() {
				exchangeName = "test-http-error"
				fakeRabbitMQClient.DeclareExchangeReturns(&http.Response{
					Status:     "418 I'm a teapot",
					StatusCode: 418,
				}, errors.New("a failure"))
			})

			It("sets the status condition", func() {
				Expect(k8sClient.Create(ctx, &exchange)).To(Succeed())
				EventuallyWithOffset(1, func() []topology.Condition {
					_ = k8sClient.Get(
						ctx,
						types.NamespacedName{Name: exchange.Name, Namespace: exchange.Namespace},
						&exchange,
					)

					return exchange.Status.Conditions
				}, statusEventsUpdateTimeout, 1*time.Second).Should(ContainElement(MatchFields(IgnoreExtras, Fields{
					"Type":    Equal(topology.ConditionType("Ready")),
					"Reason":  Equal("FailedCreateOrUpdate"),
					"Status":  Equal(corev1.ConditionFalse),
					"Message": ContainSubstring("a failure"),
				})))
			})
		})

		When("the RabbitMQ Client returns a Go error response", func() {
			BeforeEach(func() {
				exchangeName = "test-go-error"
				fakeRabbitMQClient.DeclareExchangeReturns(nil, errors.New("a go failure"))
			})

			It("sets the status condition to indicate a failure to reconcile", func() {
				Expect(k8sClient.Create(ctx, &exchange)).To(Succeed())
				EventuallyWithOffset(1, func() []topology.Condition {
					_ = k8sClient.Get(
						ctx,
						types.NamespacedName{Name: exchange.Name, Namespace: exchange.Namespace},
						&exchange,
					)

					return exchange.Status.Conditions
				}, statusEventsUpdateTimeout, 1*time.Second).Should(ContainElement(MatchFields(IgnoreExtras, Fields{
					"Type":    Equal(topology.ConditionType("Ready")),
					"Reason":  Equal("FailedCreateOrUpdate"),
					"Status":  Equal(corev1.ConditionFalse),
					"Message": ContainSubstring("a go failure"),
				})))
			})
		})
	})
	Context("LastTransitionTime", func() {
		BeforeEach(func() {
			exchangeName = "test-last-transition-time"
			fakeRabbitMQClient.DeclareExchangeReturns(&http.Response{
				Status:     "201 Created",
				StatusCode: http.StatusCreated,
			}, nil)
		})
		It("changes only if status changes", func() {
			By("setting LastTransitionTime when transitioning to status Ready=true")
			Expect(k8sClient.Create(ctx, &exchange)).To(Succeed())
			EventuallyWithOffset(1, func() []topology.Condition {
				_ = k8sClient.Get(
					ctx,
					types.NamespacedName{Namespace: exchange.Namespace, Name: exchange.Name},
					&exchange,
				)
				return exchange.Status.Conditions
			}, statusEventsUpdateTimeout, 1*time.Second).Should(ConsistOf(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(topology.ConditionType("Ready")),
				"Status": Equal(corev1.ConditionTrue),
			})))
			lastTransitionTime := exchange.Status.Conditions[0].LastTransitionTime
			Expect(lastTransitionTime.IsZero()).To(BeFalse())

			By("not touching LastTransitionTime when staying in status Ready=true")
			fakeRabbitMQClient.DeclareExchangeReturns(&http.Response{
				Status:     "204 No Content",
				StatusCode: http.StatusNoContent,
			}, nil)
			exchange.Labels = map[string]string{"k1": "v1"}
			Expect(k8sClient.Update(ctx, &exchange)).To(Succeed())
			ConsistentlyWithOffset(1, func() []topology.Condition {
				_ = k8sClient.Get(
					ctx,
					types.NamespacedName{Namespace: exchange.Namespace, Name: exchange.Name},
					&exchange,
				)
				return exchange.Status.Conditions
			}, "3s").Should(ConsistOf(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(topology.ConditionType("Ready")),
				"Status": Equal(corev1.ConditionTrue),
			})))
			Expect(exchange.Status.Conditions[0].LastTransitionTime.Time).To(BeTemporally("==", lastTransitionTime.Time))

			By("updating LastTransitionTime when transitioning to status Ready=false")
			fakeRabbitMQClient.DeclareExchangeReturns(&http.Response{
				Status:     "500 Internal Server Error",
				StatusCode: http.StatusInternalServerError,
			}, errors.New("something went wrong"))
			exchange.Labels = map[string]string{"k1": "v2"}
			Expect(k8sClient.Update(ctx, &exchange)).To(Succeed())
			EventuallyWithOffset(1, func() []topology.Condition {
				_ = k8sClient.Get(
					ctx,
					types.NamespacedName{Namespace: exchange.Namespace, Name: exchange.Name},
					&exchange,
				)
				return exchange.Status.Conditions
			}, statusEventsUpdateTimeout, 1*time.Second).Should(ConsistOf(MatchFields(IgnoreExtras, Fields{
				"Type":    Equal(topology.ConditionType("Ready")),
				"Status":  Equal(corev1.ConditionFalse),
				"Reason":  Equal("FailedCreateOrUpdate"),
				"Message": Equal("something went wrong"),
			})))
			Expect(exchange.Status.Conditions[0].LastTransitionTime.Time).To(BeTemporally(">", lastTransitionTime.Time))
		})
	})

	Context("deletion", func() {
		JustBeforeEach(func() {
			fakeRabbitMQClient.DeclareExchangeReturns(&http.Response{
				Status:     "201 Created",
				StatusCode: http.StatusCreated,
			}, nil)
			Expect(k8sClient.Create(ctx, &exchange)).To(Succeed())
			EventuallyWithOffset(1, func() []topology.Condition {
				_ = k8sClient.Get(
					ctx,
					types.NamespacedName{Name: exchange.Name, Namespace: exchange.Namespace},
					&exchange,
				)

				return exchange.Status.Conditions
			}, statusEventsUpdateTimeout, 1*time.Second).Should(ContainElement(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(topology.ConditionType("Ready")),
				"Reason": Equal("SuccessfulCreateOrUpdate"),
				"Status": Equal(corev1.ConditionTrue),
			})))
		})

		When("the RabbitMQ Client returns a HTTP error response", func() {
			BeforeEach(func() {
				exchangeName = "delete-exchange-http-error"
				fakeRabbitMQClient.DeleteExchangeReturns(&http.Response{
					Status:     "502 Bad Gateway",
					StatusCode: http.StatusBadGateway,
					Body:       io.NopCloser(bytes.NewBufferString("Hello World")),
				}, nil)
			})

			It("publishes a 'warning' event", func() {
				Expect(k8sClient.Delete(ctx, &exchange)).To(Succeed())
				Consistently(func() bool {
					err := k8sClient.Get(ctx, types.NamespacedName{Name: exchange.Name, Namespace: exchange.Namespace}, &topology.Exchange{})
					return apierrors.IsNotFound(err)
				}, statusEventsUpdateTimeout).Should(BeFalse())
				Expect(observedEvents()).To(ContainElement("Warning FailedDelete failed to delete exchange"))
			})
		})

		When("the RabbitMQ Client returns a Go error response", func() {
			BeforeEach(func() {
				exchangeName = "delete-go-error"
				fakeRabbitMQClient.DeleteExchangeReturns(nil, errors.New("some error"))
			})

			It("publishes a 'warning' event", func() {
				Expect(k8sClient.Delete(ctx, &exchange)).To(Succeed())
				Consistently(func() bool {
					err := k8sClient.Get(ctx, types.NamespacedName{Name: exchange.Name, Namespace: exchange.Namespace}, &topology.Exchange{})
					return apierrors.IsNotFound(err)
				}, statusEventsUpdateTimeout).Should(BeFalse())
				Expect(observedEvents()).To(ContainElement("Warning FailedDelete failed to delete exchange"))
			})
		})
	})
})
