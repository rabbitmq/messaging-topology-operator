package controllers_test

import (
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	topologyV1alpha "github.com/rabbitmq/messaging-topology-operator/api/v1alpha1"
	topology "github.com/rabbitmq/messaging-topology-operator/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("SetInternalDomainName", func() {
	var (
		topologyObjects          []runtimeclient.Object
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

	BeforeEach(func() {
		// The order in which these are declared matters
		// Keep it sync with the order in which 'topologyControllers' are declared in 'suite_test.go`
		topologyObjects = []runtimeclient.Object{
			&topology.Binding{
				ObjectMeta: metav1.ObjectMeta{Name: "some-binding", Namespace: "default"},
				Spec:       topology.BindingSpec{RabbitmqClusterReference: commonRabbitmqClusterRef},
			},
			&topology.Exchange{
				ObjectMeta: metav1.ObjectMeta{Name: "some-exchange", Namespace: "default"},
				Spec:       topology.ExchangeSpec{RabbitmqClusterReference: commonRabbitmqClusterRef},
			},
			&topology.Permission{
				ObjectMeta: metav1.ObjectMeta{Name: "some-exchange", Namespace: "default"},
				Spec:       topology.PermissionSpec{RabbitmqClusterReference: commonRabbitmqClusterRef},
			},
			&topology.Policy{
				ObjectMeta: metav1.ObjectMeta{Name: "some-policy", Namespace: "default"},
				Spec: topology.PolicySpec{RabbitmqClusterReference: commonRabbitmqClusterRef,
					Definition: &runtime.RawExtension{
						Raw: []byte(`{"key":"value"}`),
					}},
			},
			&topology.Queue{
				ObjectMeta: metav1.ObjectMeta{Name: "some-queue", Namespace: "default"},
				Spec:       topology.QueueSpec{RabbitmqClusterReference: commonRabbitmqClusterRef},
			},
			&topology.User{
				ObjectMeta: metav1.ObjectMeta{Name: "some-user", Namespace: "default"},
				Spec:       topology.UserSpec{RabbitmqClusterReference: commonRabbitmqClusterRef},
			},
			&topology.Vhost{
				ObjectMeta: metav1.ObjectMeta{Name: "some-vhost", Namespace: "default"},
				Spec:       topology.VhostSpec{RabbitmqClusterReference: commonRabbitmqClusterRef},
			},
			&topology.SchemaReplication{
				ObjectMeta: metav1.ObjectMeta{Name: "some-vhost", Namespace: "default"},
				Spec:       topology.SchemaReplicationSpec{RabbitmqClusterReference: commonRabbitmqClusterRef},
			},
			&topology.Federation{
				ObjectMeta: metav1.ObjectMeta{Name: "some-federation", Namespace: "default"},
				Spec: topology.FederationSpec{RabbitmqClusterReference: commonRabbitmqClusterRef,
					UriSecret: &corev1.LocalObjectReference{Name: "federation-uri"}},
			},
			&topology.Shovel{
				ObjectMeta: metav1.ObjectMeta{Name: "some-shovel", Namespace: "default"},
				Spec: topology.ShovelSpec{RabbitmqClusterReference: commonRabbitmqClusterRef,
					UriSecret: &corev1.LocalObjectReference{Name: "shovel-uri-secret"}},
			},
			&topologyV1alpha.SuperStream{
				ObjectMeta: metav1.ObjectMeta{Name: "some-super-stream", Namespace: "default"},
				Spec:       topologyV1alpha.SuperStreamSpec{RabbitmqClusterReference: commonRabbitmqClusterRef},
			},
		}

		fakeRabbitMQClient.DeclareBindingReturns(commonHttpCreatedResponse, nil)
		fakeRabbitMQClient.DeleteBindingReturns(commonHttpDeletedResponse, nil)
		fakeRabbitMQClient.DeclareExchangeReturns(commonHttpCreatedResponse, nil)
		fakeRabbitMQClient.DeleteExchangeReturns(commonHttpDeletedResponse, nil)
		fakeRabbitMQClient.UpdatePermissionsInReturns(commonHttpCreatedResponse, nil)
		fakeRabbitMQClient.ClearPermissionsInReturns(commonHttpDeletedResponse, nil)
		fakeRabbitMQClient.PutPolicyReturns(commonHttpCreatedResponse, nil)
		fakeRabbitMQClient.DeletePolicyReturns(commonHttpDeletedResponse, nil)
		fakeRabbitMQClient.DeclareQueueReturns(commonHttpCreatedResponse, nil)
		fakeRabbitMQClient.DeleteQueueReturns(commonHttpDeletedResponse, nil)
		fakeRabbitMQClient.PutUserReturns(commonHttpCreatedResponse, nil)
		fakeRabbitMQClient.DeleteUserReturns(commonHttpDeletedResponse, nil)
		fakeRabbitMQClient.PutVhostReturns(commonHttpCreatedResponse, nil)
		fakeRabbitMQClient.DeleteVhostReturns(commonHttpDeletedResponse, nil)
		fakeRabbitMQClient.PutGlobalParameterReturns(commonHttpCreatedResponse, nil)
		fakeRabbitMQClient.DeleteGlobalParameterReturns(commonHttpDeletedResponse, nil)
		fakeRabbitMQClient.PutFederationUpstreamReturns(commonHttpCreatedResponse, nil)
		fakeRabbitMQClient.DeleteFederationUpstreamReturns(commonHttpDeletedResponse, nil)
		fakeRabbitMQClient.DeclareShovelReturns(commonHttpCreatedResponse, nil)
		fakeRabbitMQClient.DeleteShovelReturns(commonHttpDeletedResponse, nil)
	})

	It("sets the domain name in the URI to connect to RabbitMQ", func() {
		for _, controller := range topologyReconcilers {
			controller.SetInternalDomainName(".some-domain.com")
		}
		superStreamReconciler.SetInternalDomainName(".some-domain.com")

		for i, obj := range topologyObjects {
			Expect(client.Create(ctx, obj)).To(Succeed())

			// Wait until the client factory is called
			Eventually(func() int {
				return len(fakeRabbitMQClientFactoryArgsForCall)
			}, 5).Should(BeNumerically(">", i))

			credentials, _, _ := FakeRabbitMQClientFactoryArgsForCall(i)
			uri, found := credentials.Data("uri")
			Expect(found).To(BeTrue(), "expected to find key 'uri'")
			// Equals() fails here because the types dont match. BeEquivalent() is more lax, allows different type comparison
			Expect(uri).To(BeEquivalentTo("https://example-rabbit.default.svc.some-domain.com:15671"),
				"Offender: %d", i)
		}
	})

	When("domain name is not set", func() {
		It("uses internal short name", func() {
			for _, controller := range topologyReconcilers {
				controller.SetInternalDomainName("")
			}
			superStreamReconciler.SetInternalDomainName("")

			for i, obj := range topologyObjects {
				Expect(client.Create(ctx, obj)).To(Succeed())

				// Wait until the client factory is called
				Eventually(func() int {
					return len(fakeRabbitMQClientFactoryArgsForCall)
				}, 5).Should(BeNumerically(">", i))

				credentials, _, _ := FakeRabbitMQClientFactoryArgsForCall(i)
				uri, found := credentials.Data("uri")
				Expect(found).To(BeTrue(), "expected to find key 'uri'")
				// Equals() fails here because the types dont match. BeEquivalent() is more lax, allows different type comparison
				Expect(uri).To(BeEquivalentTo("https://example-rabbit.default.svc:15671"),
					"Offender: %d", i)
			}
		})
	})

	AfterEach(func() {
		for _, controller := range topologyReconcilers {
			controller.SetInternalDomainName("")
		}
		superStreamReconciler.SetInternalDomainName("")
		for _, topologyObject := range topologyObjects {
			Expect(client.Delete(ctx, topologyObject)).To(Succeed())
			Eventually(func() bool {
				dummy := topologyObject.DeepCopyObject()
				err := client.Get(ctx,
					types.NamespacedName{Name: topologyObject.GetName(), Namespace: topologyObject.GetNamespace()},
					dummy.(runtimeclient.Object))
				return apierrors.IsNotFound(err)
			}, 10, 2).Should(BeTrue(), "expected to receive a 'not found' error")
		}
	})
})
