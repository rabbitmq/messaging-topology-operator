package controllers_test

import (
	"context"
	"fmt"
	"github.com/rabbitmq/messaging-topology-operator/controllers"
	"net/http"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	runtimeClient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/config"
	"sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	topology "github.com/rabbitmq/messaging-topology-operator/api/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
		// case with testenv and kubernetes controllers.
		<-time.After(time.Second)
	})

	When("k8s domain is configured", func() {
		It("sets the domain name in the URI to connect to RabbitMQ", func() {
			Expect((&controllers.TopologyReconciler{
				Client:                  topologyMgr.GetClient(),
				Type:                    &topology.Queue{},
				Scheme:                  topologyMgr.GetScheme(),
				Recorder:                fakeRecorder,
				RabbitmqClientFactory:   fakeRabbitMQClientFactory,
				ReconcileFunc:           &controllers.QueueReconciler{},
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
			Expect((&controllers.TopologyReconciler{
				Client:                topologyMgr.GetClient(),
				Type:                  &topology.Queue{},
				Scheme:                topologyMgr.GetScheme(),
				Recorder:              fakeRecorder,
				RabbitmqClientFactory: fakeRabbitMQClientFactory,
				ReconcileFunc:         &controllers.QueueReconciler{},
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
			Expect((&controllers.TopologyReconciler{
				Client:                topologyMgr.GetClient(),
				Type:                  &topology.Queue{},
				Scheme:                topologyMgr.GetScheme(),
				Recorder:              fakeRecorder,
				RabbitmqClientFactory: fakeRabbitMQClientFactory,
				ReconcileFunc:         &controllers.QueueReconciler{},
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
})
