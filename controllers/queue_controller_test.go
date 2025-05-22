package controllers_test

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

	"github.com/rabbitmq/messaging-topology-operator/controllers"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	runtimeClient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/config"
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

var _ = Describe("queue-controller", func() {
	var (
		queue         topology.Queue
		queueName     string
		queueMgr      ctrl.Manager
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
		queueMgr, err = ctrl.NewManager(testEnv.Config, ctrl.Options{
			Metrics: server.Options{
				BindAddress: "0", // To avoid MacOS firewall pop-up every time you run this suite
			},
			Cache: cache.Options{
				DefaultNamespaces: map[string]cache.Config{queueNamespace: {
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
			Expect(queueMgr.Start(ctx)).To(Succeed())
		}(managerCtx)

		k8sClient = queueMgr.GetClient()

		Expect((&controllers.TopologyReconciler{
			Client:                queueMgr.GetClient(),
			Type:                  &topology.Queue{},
			Scheme:                queueMgr.GetScheme(),
			Recorder:              fakeRecorder,
			RabbitmqClientFactory: fakeRabbitMQClientFactory,
			ReconcileFunc:         &controllers.QueueReconciler{},
		}).SetupWithManager(queueMgr)).To(Succeed())
	}

	initialiseQueue := func() {
		queue = topology.Queue{
			ObjectMeta: metav1.ObjectMeta{
				Name:      queueName,
				Namespace: queueNamespace,
			},
			Spec: topology.QueueSpec{
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
			Expect(k8sClient.Delete(ctx, &queue)).To(Succeed())
		})

		When("the RabbitMQ Client returns a HTTP error response", func() {
			BeforeEach(func() {
				queueName = "test-http-error"
				fakeRabbitMQClient.DeclareQueueReturns(&http.Response{
					Status:     "418 I'm a teapot",
					StatusCode: 418,
				}, errors.New("a failure"))
				initialiseQueue()
				queue.Labels = map[string]string{"test": "test-http-error"}
				initialiseManager("test", "test-http-error")
			})

			It("sets the status condition", func() {
				Expect(k8sClient.Create(ctx, &queue)).To(Succeed())
				Eventually(func() []topology.Condition {
					_ = k8sClient.Get(
						ctx,
						types.NamespacedName{Name: queue.Name, Namespace: queue.Namespace},
						&queue,
					)

					return queue.Status.Conditions
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
				queueName = "test-go-error"
				fakeRabbitMQClient.DeclareQueueReturns(nil, errors.New("a go failure"))
				initialiseQueue()
				queue.Labels = map[string]string{"test": "test-go-error"}
				initialiseManager("test", "test-go-error")
			})

			It("sets the status condition to indicate a failure to reconcile", func() {
				Expect(k8sClient.Create(ctx, &queue)).To(Succeed())
				Eventually(func() []topology.Condition {
					_ = k8sClient.Get(
						ctx,
						types.NamespacedName{Name: queue.Name, Namespace: queue.Namespace},
						&queue,
					)

					return queue.Status.Conditions
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
			fakeRabbitMQClient.DeclareQueueReturns(&http.Response{
				Status:     "201 Created",
				StatusCode: http.StatusCreated,
			}, nil)
			Expect(k8sClient.Create(ctx, &queue)).To(Succeed())
			Eventually(func() []topology.Condition {
				_ = k8sClient.Get(
					ctx,
					types.NamespacedName{Name: queue.Name, Namespace: queue.Namespace},
					&queue,
				)

				return queue.Status.Conditions
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
				queueName = "delete-queue-http-error"
				fakeRabbitMQClient.DeleteQueueReturns(&http.Response{
					Status:     "502 Bad Gateway",
					StatusCode: http.StatusBadGateway,
					Body:       io.NopCloser(bytes.NewBufferString("Hello World")),
				}, nil)
				initialiseQueue()
				queue.Labels = map[string]string{"test": "delete-queue-http-error"}
				initialiseManager("test", "delete-queue-http-error")
			})

			It("publishes a 'warning' event", func() {
				Expect(k8sClient.Delete(ctx, &queue)).To(Succeed())
				Consistently(func() bool {
					err := k8sClient.Get(ctx, types.NamespacedName{Name: queue.Name, Namespace: queue.Namespace}, &topology.Queue{})
					return apierrors.IsNotFound(err)
				}).
					Within(statusEventsUpdateTimeout).
					Should(BeFalse())
				Expect(observedEvents()).To(ContainElement("Warning FailedDelete failed to delete queue"))
			})
		})

		When("the RabbitMQ Client returns a Go error response", func() {
			BeforeEach(func() {
				queueName = "delete-go-error"
				fakeRabbitMQClient.DeleteQueueReturns(nil, errors.New("some error"))
				initialiseQueue()
				queue.Labels = map[string]string{"test": "delete-go-error"}
				initialiseManager("test", "delete-go-error")
			})

			It("publishes a 'warning' event", func() {
				Expect(k8sClient.Delete(ctx, &queue)).To(Succeed())
				Consistently(func() bool {
					err := k8sClient.Get(ctx, types.NamespacedName{Name: queue.Name, Namespace: queue.Namespace}, &topology.Queue{})
					return apierrors.IsNotFound(err)
				}).
					Within(statusEventsUpdateTimeout).
					Should(BeFalse())
				Expect(observedEvents()).To(ContainElement("Warning FailedDelete failed to delete queue"))
			})
		})
	})

	When("the Queue has DeletionPolicy set to retain", func() {
		BeforeEach(func() {
			queueName = "queue-with-retain-policy"
			fakeRabbitMQClient.DeleteQueueReturns(&http.Response{
				Status:     "200 OK",
				StatusCode: http.StatusOK,
			}, nil)
			initialiseQueue()
			queue.Labels = map[string]string{"test": "queue-with-retain-policy"}
			initialiseManager("test", "queue-with-retain-policy")
		})

		It("deletes the k8s resource but preserves the queue in RabbitMQ server", func() {
			queue.Spec.DeletionPolicy = "retain"
			Expect(k8sClient.Create(ctx, &queue)).To(Succeed())
			Eventually(fakeRabbitMQClient.DeclareQueueCallCount).
				WithPolling(time.Second).
				Within(time.Second*3).
				Should(BeNumerically(">=", 1), "Expected to call RMQ API to declare queue")

			Expect(k8sClient.Delete(ctx, &queue)).To(Succeed())
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: queue.Name, Namespace: queue.Namespace}, &queue)
				return apierrors.IsNotFound(err)
			}).
				Within(statusEventsUpdateTimeout).
				WithPolling(time.Second).
				Should(BeTrue(), "Queue should not be found")

			Expect(fakeRabbitMQClient.DeleteQueueCallCount()).To(Equal(0), "Expected to delete queue and no calls to RMQ API")
		})
	})
})
