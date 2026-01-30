package system_tests

import (
	"context"
	rabbithole "github.com/michaelklishin/rabbit-hole/v3"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	topology "github.com/rabbitmq/messaging-topology-operator/api/v1beta1"
)

var _ = Describe("schema replication", func() {

	var (
		endpointsSecret corev1.Secret
		namespace       = MustHaveEnv("NAMESPACE")
		ctx             = context.Background()
		replication     = &topology.SchemaReplication{}
	)

	AfterEach(func() {
		Expect(k8sClient.Delete(ctx, &endpointsSecret, &client.DeleteOptions{})).To(Succeed())
	})

	BeforeEach(func() {
		endpointsSecret = corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "endpoints-secret",
				Namespace: namespace,
			},
			Type: corev1.SecretTypeOpaque,
			Data: map[string][]byte{
				"username": []byte("some-username"),
				"password": []byte("some-password"),
			},
		}
		Expect(k8sClient.Create(ctx, &endpointsSecret, &client.CreateOptions{})).To(Succeed())
		replication = &topology.SchemaReplication{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "replication",
				Namespace: namespace,
			},
			Spec: topology.SchemaReplicationSpec{
				Endpoints: "abc.endpoints.local:5672,efg.endpoints.local:1234",
				UpstreamSecret: &corev1.LocalObjectReference{
					Name: "endpoints-secret",
				},
				RabbitmqClusterReference: topology.RabbitmqClusterReference{
					Name: rmq.Name,
				},
			},
		}
	})

	It("works", func() {
		SetDefaultEventuallyPollingInterval(2 * time.Second)
		SetDefaultEventuallyTimeout(30 * time.Second)
		getRabbitGlobalParams := func() ([]rabbithole.GlobalRuntimeParameter, error) {
			return rabbitClient.ListGlobalParameters()
		}

		By("setting schema replication upstream global parameters successfully")
		Expect(k8sClient.Create(ctx, replication, &client.CreateOptions{})).To(Succeed())
		DeferCleanup(func() {
			// leaving a cleanup step in case the test fails, so that it does not leave behind resources
			// In the happy path, the schemareplication object is deleted, and the following command is a no-op
			_ = k8sClient.Delete(ctx, replication, &client.DeleteOptions{})
		})
		Eventually(getRabbitGlobalParams).Should(ContainElement(And(
			HaveField("Name", "schema_definition_sync_upstream"),
			HaveField("Value", And(
				HaveKeyWithValue("endpoints", ContainElements("abc.endpoints.local:5672", "efg.endpoints.local:1234")),
				HaveKeyWithValue("username", "some-username"),
				HaveKeyWithValue("password", "some-password"),
			)),
		)))

		By("updating status condition 'Ready'")
		updatedReplication := topology.SchemaReplication{}

		Eventually(func() []topology.Condition {
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: replication.Name, Namespace: replication.Namespace}, &updatedReplication)).To(Succeed())
			return updatedReplication.Status.Conditions
		}, waitUpdatedStatusCondition).Should(HaveLen(1), "Schema Replication status condition should be present")

		readyCondition := updatedReplication.Status.Conditions[0]
		Expect(string(readyCondition.Type)).To(Equal("Ready"))
		Expect(readyCondition.Status).To(Equal(corev1.ConditionTrue))
		Expect(readyCondition.Reason).To(Equal("SuccessfulCreateOrUpdate"))
		Expect(readyCondition.LastTransitionTime).NotTo(BeZero())

		By("setting correct finalizer")
		Expect(updatedReplication.ObjectMeta.Finalizers).To(ConsistOf("deletion.finalizers.schemareplications.rabbitmq.com"))

		By("setting status.observedGeneration")
		Expect(updatedReplication.Status.ObservedGeneration).To(Equal(updatedReplication.GetGeneration()))

		By("not allowing updates on rabbitmqClusterReference")
		updateTest := topology.SchemaReplication{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: replication.Name, Namespace: replication.Namespace}, &updateTest)).To(Succeed())
		updateTest.Spec.RabbitmqClusterReference.Name = "new-cluster"
		Expect(k8sClient.Update(ctx, &updateTest).Error()).To(ContainSubstring("spec.rabbitmqClusterReference: Forbidden: update on rabbitmqClusterReference is forbidden"))

		By("unsetting schema replication upstream global parameters on deletion")
		Expect(k8sClient.Delete(ctx, replication)).To(Succeed())
		Eventually(getRabbitGlobalParams).ShouldNot(ContainElement(HaveField("Name", "schema_definition_sync_upstream")))
	})
})
