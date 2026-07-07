package controller_test

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/rabbitmq/messaging-topology-operator/internal/controller"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	runtimeClient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/config"
	"sigs.k8s.io/controller-runtime/pkg/metrics/server"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	rabbitmqv1beta1 "github.com/rabbitmq/cluster-operator/v2/api/v1beta1"
	topology "github.com/rabbitmq/messaging-topology-operator/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("TopologyReconciler", func() {
	const (
		name = "example-rabbit"
	)

	var (
		commonRabbitmqClusterRef = topology.RabbitmqClusterReference{
			Name:      name,
			Namespace: topologyNamespace,
		}
		commonHttpCreatedResponse = &http.Response{
			Status:     "201 Created",
			StatusCode: http.StatusCreated,
		}
		commonHttpDeletedResponse = &http.Response{
			Status:     "204 No Content",
			StatusCode: http.StatusNoContent,
		}
		topologyMgr   ctrl.Manager
		managerCtx    context.Context
		managerCancel context.CancelFunc
		k8sClient     runtimeClient.Client
	)

	BeforeEach(func() {
		var err error
		topologyMgr, err = ctrl.NewManager(testEnv.Config, ctrl.Options{
			Metrics: server.Options{
				BindAddress: "0", // To avoid MacOS firewall pop-up every time you run this suite
			},
			Cache: cache.Options{
				DefaultNamespaces: map[string]cache.Config{topologyNamespace: {}},
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
			Expect(topologyMgr.Start(ctx)).To(Succeed())
		}(managerCtx)

		k8sClient = topologyMgr.GetClient()
	})

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

	When("k8s domain is configured", func() {
		It("sets the domain name in the URI to connect to RabbitMQ", func() {
			Expect((&controller.TopologyReconciler{
				Client:                  topologyMgr.GetClient(),
				APIReader:               topologyMgr.GetAPIReader(),
				Type:                    &topology.Queue{},
				Scheme:                  topologyMgr.GetScheme(),
				Recorder:                fakeRecorder,
				RabbitmqClientFactory:   fakeRabbitMQClientFactory,
				ReconcileFunc:           &controller.QueueReconciler{},
				KubernetesClusterDomain: ".some-domain.com",
			}).SetupWithManager(topologyMgr)).To(Succeed())

			queue := &topology.Queue{
				ObjectMeta: metav1.ObjectMeta{Name: "ab-queue", Namespace: topologyNamespace},
				Spec:       topology.QueueSpec{RabbitmqClusterReference: commonRabbitmqClusterRef},
			}
			fakeRabbitMQClient.DeclareQueueReturns(commonHttpCreatedResponse, nil)
			fakeRabbitMQClient.DeleteQueueReturns(commonHttpDeletedResponse, nil)
			Expect(k8sClient.Create(ctx, queue)).To(Succeed())

			Eventually(func() int {
				return len(fakeRabbitMQClientFactoryArgsForCall)
			}, 5).Should(BeNumerically(">", 0))

			credentials, _, _ := FakeRabbitMQClientFactoryArgsForCall(0)
			expected := fmt.Sprintf("https://%s.%s.svc.some-domain.com:15671", name, topologyNamespace)
			Expect(credentials).Should(HaveKeyWithValue("uri", expected))
		})
	})

	When("domain name is not set", func() {
		It("uses internal short name", func() {
			Expect((&controller.TopologyReconciler{
				Client:                topologyMgr.GetClient(),
				APIReader:             topologyMgr.GetAPIReader(),
				Type:                  &topology.Queue{},
				Scheme:                topologyMgr.GetScheme(),
				Recorder:              fakeRecorder,
				RabbitmqClientFactory: fakeRabbitMQClientFactory,
				ReconcileFunc:         &controller.QueueReconciler{},
			}).SetupWithManager(topologyMgr)).To(Succeed())

			queue := &topology.Queue{
				ObjectMeta: metav1.ObjectMeta{Name: "bb-queue", Namespace: topologyNamespace},
				Spec:       topology.QueueSpec{RabbitmqClusterReference: commonRabbitmqClusterRef},
			}
			fakeRabbitMQClient.DeclareQueueReturns(commonHttpCreatedResponse, nil)
			fakeRabbitMQClient.DeleteQueueReturns(commonHttpDeletedResponse, nil)
			Expect(k8sClient.Create(ctx, queue)).To(Succeed())

			Eventually(func() int {
				return len(fakeRabbitMQClientFactoryArgsForCall)
			}, 5).Should(BeNumerically(">", 0))

			credentials, _, _ := FakeRabbitMQClientFactoryArgsForCall(0)
			expected := fmt.Sprintf("https://%s.%s.svc:15671", name, topologyNamespace)
			Expect(credentials).Should(HaveKeyWithValue("uri", expected))
		})
	})

	When("flag for plain HTTP connection is set", func() {
		It("uses http for connection", func() {
			Expect((&controller.TopologyReconciler{
				Client:                topologyMgr.GetClient(),
				APIReader:             topologyMgr.GetAPIReader(),
				Type:                  &topology.Queue{},
				Scheme:                topologyMgr.GetScheme(),
				Recorder:              fakeRecorder,
				RabbitmqClientFactory: fakeRabbitMQClientFactory,
				ReconcileFunc:         &controller.QueueReconciler{},
				ConnectUsingPlainHTTP: true,
			}).SetupWithManager(topologyMgr)).To(Succeed())

			queue := &topology.Queue{
				ObjectMeta: metav1.ObjectMeta{Name: "cb-queue", Namespace: topologyNamespace},
				Spec:       topology.QueueSpec{RabbitmqClusterReference: commonRabbitmqClusterRef},
			}
			fakeRabbitMQClient.DeclareQueueReturns(commonHttpCreatedResponse, nil)
			fakeRabbitMQClient.DeleteQueueReturns(commonHttpDeletedResponse, nil)
			Expect(k8sClient.Create(ctx, queue)).To(Succeed())

			Eventually(func() int {
				return len(fakeRabbitMQClientFactoryArgsForCall)
			}, 5).Should(BeNumerically(">", 0))

			credentials, _, _ := FakeRabbitMQClientFactoryArgsForCall(0)
			expected := fmt.Sprintf("http://%s.%s.svc:15672", name, topologyNamespace)
			Expect(credentials).Should(HaveKeyWithValue("uri", expected))
		})
	})

	When("the referenced RabbitmqCluster is scaled to zero", func() {
		var (
			cluster *rabbitmqv1beta1.RabbitmqCluster
			queue   *topology.Queue
		)

		AfterEach(func() {
			// Delete leftover objects, and wait for the Queue to actually disappear, so a
			// subsequent test's fresh manager cache doesn't re-reconcile them and pollute
			// the shared fakeRabbitMQClientFactory call log.
			if queue != nil {
				_ = k8sClient.Delete(ctx, queue)
				Eventually(func() bool {
					err := k8sClient.Get(ctx, types.NamespacedName{Name: queue.Name, Namespace: queue.Namespace}, &topology.Queue{})
					return apierrors.IsNotFound(err)
				}, 10*time.Second, 1*time.Second).Should(BeTrue())
			}
			if cluster != nil {
				_ = k8sClient.Delete(ctx, cluster)
			}
		})

		setupReconciler := func() {
			Expect((&controller.TopologyReconciler{
				Client:                topologyMgr.GetClient(),
				APIReader:             topologyMgr.GetAPIReader(),
				Type:                  &topology.Queue{},
				ListType:              &topology.QueueList{},
				Scheme:                topologyMgr.GetScheme(),
				Recorder:              fakeRecorder,
				RabbitmqClientFactory: fakeRabbitMQClientFactory,
				ReconcileFunc:         &controller.QueueReconciler{},
			}).SetupWithManager(topologyMgr)).To(Succeed())
		}

		It("sets a NotReady condition and does not declare the queue", func() {
			setupReconciler()

			cluster = &rabbitmqv1beta1.RabbitmqCluster{
				ObjectMeta: metav1.ObjectMeta{Name: "scaled-to-zero-rabbit-1", Namespace: topologyNamespace},
				Spec:       rabbitmqv1beta1.RabbitmqClusterSpec{Replicas: ptr(int32(0))},
			}
			Expect(k8sClient.Create(ctx, cluster)).To(Succeed())

			queue = &topology.Queue{
				ObjectMeta: metav1.ObjectMeta{Name: "scaled-to-zero-queue-1", Namespace: topologyNamespace},
				Spec: topology.QueueSpec{RabbitmqClusterReference: topology.RabbitmqClusterReference{
					Name:      cluster.Name,
					Namespace: topologyNamespace,
				}},
			}
			Expect(k8sClient.Create(ctx, queue)).To(Succeed())

			Eventually(func() []topology.Condition {
				_ = k8sClient.Get(ctx, types.NamespacedName{Name: queue.Name, Namespace: queue.Namespace}, queue)
				return queue.Status.Conditions
			}, 10*time.Second, 1*time.Second).Should(ContainElement(MatchFields(IgnoreExtras, Fields{
				"Type":    Equal(topology.ConditionType("Ready")),
				"Status":  Equal(corev1.ConditionFalse),
				"Message": ContainSubstring("scaled to zero"),
			})))
		})

		It("removes the finalizer on deletion without calling the broker", func() {
			setupReconciler()

			cluster = &rabbitmqv1beta1.RabbitmqCluster{
				ObjectMeta: metav1.ObjectMeta{Name: "scaled-to-zero-rabbit-2", Namespace: topologyNamespace},
			}
			Expect(createRabbitmqClusterResources(k8sClient, cluster)).To(Succeed())

			queue = &topology.Queue{
				ObjectMeta: metav1.ObjectMeta{Name: "scaled-to-zero-queue-2", Namespace: topologyNamespace},
				Spec: topology.QueueSpec{RabbitmqClusterReference: topology.RabbitmqClusterReference{
					Name:      cluster.Name,
					Namespace: topologyNamespace,
				}},
			}
			fakeRabbitMQClient.DeclareQueueReturns(commonHttpCreatedResponse, nil)
			Expect(k8sClient.Create(ctx, queue)).To(Succeed())

			Eventually(func() []topology.Condition {
				_ = k8sClient.Get(ctx, types.NamespacedName{Name: queue.Name, Namespace: queue.Namespace}, queue)
				return queue.Status.Conditions
			}, 10*time.Second, 1*time.Second).Should(ContainElement(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(topology.ConditionType("Ready")),
				"Status": Equal(corev1.ConditionTrue),
			})))

			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: cluster.Name, Namespace: cluster.Namespace}, cluster)).To(Succeed())
			cluster.Spec.Replicas = ptr(int32(0))
			Expect(k8sClient.Update(ctx, cluster)).To(Succeed())

			deleteCallsBefore := fakeRabbitMQClient.DeleteQueueCallCount()
			Expect(k8sClient.Delete(ctx, queue)).To(Succeed())

			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: queue.Name, Namespace: queue.Namespace}, &topology.Queue{})
				return apierrors.IsNotFound(err)
			}, 10*time.Second, 1*time.Second).Should(BeTrue())

			Expect(fakeRabbitMQClient.DeleteQueueCallCount()).To(Equal(deleteCallsBefore))
		})

		It("recovers once the cluster is scaled back up, via the RabbitmqCluster watch", func() {
			setupReconciler()

			cluster = &rabbitmqv1beta1.RabbitmqCluster{
				ObjectMeta: metav1.ObjectMeta{Name: "scaled-to-zero-rabbit-3", Namespace: topologyNamespace},
				Spec:       rabbitmqv1beta1.RabbitmqClusterSpec{Replicas: ptr(int32(0))},
			}
			Expect(createRabbitmqClusterResources(k8sClient, cluster)).To(Succeed())

			queue = &topology.Queue{
				ObjectMeta: metav1.ObjectMeta{Name: "scaled-to-zero-queue-3", Namespace: topologyNamespace},
				Spec: topology.QueueSpec{RabbitmqClusterReference: topology.RabbitmqClusterReference{
					Name:      cluster.Name,
					Namespace: topologyNamespace,
				}},
			}
			fakeRabbitMQClient.DeclareQueueReturns(commonHttpCreatedResponse, nil)
			fakeRabbitMQClient.DeleteQueueReturns(commonHttpDeletedResponse, nil)
			Expect(k8sClient.Create(ctx, queue)).To(Succeed())

			Eventually(func() []topology.Condition {
				_ = k8sClient.Get(ctx, types.NamespacedName{Name: queue.Name, Namespace: queue.Namespace}, queue)
				return queue.Status.Conditions
			}, 10*time.Second, 1*time.Second).Should(ContainElement(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(topology.ConditionType("Ready")),
				"Status": Equal(corev1.ConditionFalse),
			})))

			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: cluster.Name, Namespace: cluster.Namespace}, cluster)).To(Succeed())
			cluster.Spec.Replicas = ptr(int32(1))
			Expect(k8sClient.Update(ctx, cluster)).To(Succeed())

			Eventually(func() []topology.Condition {
				_ = k8sClient.Get(ctx, types.NamespacedName{Name: queue.Name, Namespace: queue.Namespace}, queue)
				return queue.Status.Conditions
			}, 10*time.Second, 1*time.Second).Should(ContainElement(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(topology.ConditionType("Ready")),
				"Status": Equal(corev1.ConditionTrue),
			})))
		})
	})
})

func ptr[T any](v T) *T {
	return &v
}
