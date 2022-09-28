package controllers_test

import (
	"github.com/rabbitmq/messaging-topology-operator/controllers"
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	topology "github.com/rabbitmq/messaging-topology-operator/api/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("TopologyReconciler", func() {
	var (
		commonRabbitmqClusterRef = topology.RabbitmqClusterReference{
			Name:      "example-rabbit",
			Namespace: "default",
		}
		commonHttpCreatedResponse = &http.Response{
			Status:     "201 Created",
			StatusCode: http.StatusCreated,
		}
		commonHttpDeletedResponse = &http.Response{
			Status:     "204 No Content",
			StatusCode: http.StatusNoContent,
		}
	)

	When("k8s domain is configured", func() {
		It("sets the domain name in the URI to connect to RabbitMQ", func() {
			Expect((&controllers.TopologyReconciler{
				Client:                  mgr.GetClient(),
				Type:                    &topology.Queue{},
				Scheme:                  mgr.GetScheme(),
				Recorder:                fakeRecorder,
				RabbitmqClientFactory:   fakeRabbitMQClientFactory,
				ReconcileFunc:           &controllers.QueueReconciler{},
				KubernetesClusterDomain: ".some-domain.com",
			}).SetupWithManager(mgr)).To(Succeed())

			queue := &topology.Queue{
				ObjectMeta: metav1.ObjectMeta{Name: "ab-queue", Namespace: "default"},
				Spec:       topology.QueueSpec{RabbitmqClusterReference: commonRabbitmqClusterRef},
			}
			fakeRabbitMQClient.DeclareQueueReturns(commonHttpCreatedResponse, nil)
			fakeRabbitMQClient.DeleteQueueReturns(commonHttpDeletedResponse, nil)
			Expect(client.Create(ctx, queue)).To(Succeed())

			Eventually(func() int {
				return len(fakeRabbitMQClientFactoryArgsForCall)
			}, 5).Should(BeNumerically(">", 0))

			credentials, _, _ := FakeRabbitMQClientFactoryArgsForCall(0)
			uri, found := credentials["uri"]
			Expect(found).To(BeTrue(), "expected to find key 'uri'")
			Expect(uri).To(BeEquivalentTo("https://example-rabbit.default.svc.some-domain.com:15671"))
		})
	})

	When("domain name is not set", func() {
		It("uses internal short name", func() {
			Expect((&controllers.TopologyReconciler{
				Client:                mgr.GetClient(),
				Type:                  &topology.Queue{},
				Scheme:                mgr.GetScheme(),
				Recorder:              fakeRecorder,
				RabbitmqClientFactory: fakeRabbitMQClientFactory,
				ReconcileFunc:         &controllers.QueueReconciler{},
			}).SetupWithManager(mgr)).To(Succeed())

			queue := &topology.Queue{
				ObjectMeta: metav1.ObjectMeta{Name: "bb-queue", Namespace: "default"},
				Spec:       topology.QueueSpec{RabbitmqClusterReference: commonRabbitmqClusterRef},
			}
			fakeRabbitMQClient.DeclareQueueReturns(commonHttpCreatedResponse, nil)
			fakeRabbitMQClient.DeleteQueueReturns(commonHttpDeletedResponse, nil)
			Expect(client.Create(ctx, queue)).To(Succeed())

			Eventually(func() int {
				return len(fakeRabbitMQClientFactoryArgsForCall)
			}, 5).Should(BeNumerically(">", 0))

			credentials, _, _ := FakeRabbitMQClientFactoryArgsForCall(0)
			uri, found := credentials["uri"]
			Expect(found).To(BeTrue(), "expected to find key 'uri'")
			Expect(uri).To(BeEquivalentTo("https://example-rabbit.default.svc:15671"))
		})
	})
})
