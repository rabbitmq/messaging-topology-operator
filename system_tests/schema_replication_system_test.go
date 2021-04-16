package system_tests

import (
	"context"
	rabbithole "github.com/michaelklishin/rabbit-hole/v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	topology "github.com/rabbitmq/messaging-topology-operator/api/v1alpha2"
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
				"endpoints": []byte("abc.endpoints.local:5672,efg.endpoints.local:1234"),
				"username":  []byte("some-username"),
				"password":  []byte("some-password"),
			},
		}
		Expect(k8sClient.Create(ctx, &endpointsSecret, &client.CreateOptions{})).To(Succeed())
		replication = &topology.SchemaReplication{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "replication",
				Namespace: namespace,
			},
			Spec: topology.SchemaReplicationSpec{
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
		By("setting schema replication upstream global parameters successfully")
		Expect(k8sClient.Create(ctx, replication, &client.CreateOptions{})).To(Succeed())
		var allGlobalParams []rabbithole.GlobalRuntimeParameter
		Eventually(func() []rabbithole.GlobalRuntimeParameter {
			var err error
			allGlobalParams, err = rabbitClient.ListGlobalParameters()
			Expect(err).NotTo(HaveOccurred())
			return allGlobalParams
		}, 5, 2).Should(HaveLen(3)) // cluster_name and internal_cluster_id are set by default by RabbitMQ

		Expect(allGlobalParams).To(ContainElement(
			rabbithole.GlobalRuntimeParameter{
				Name: "schema_definition_sync_upstream",
				Value: map[string]interface{}{
					"endpoints": []interface{}{"abc.endpoints.local:5672", "efg.endpoints.local:1234"},
					"username":  "some-username",
					"password":  "some-password",
				},
			}))

		By("updating status condition 'Ready'")
		updatedReplication := topology.SchemaReplication{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: replication.Name, Namespace: replication.Namespace}, &updatedReplication)).To(Succeed())

		Expect(updatedReplication.Status.Conditions).To(HaveLen(1))
		readyCondition := updatedReplication.Status.Conditions[0]
		Expect(string(readyCondition.Type)).To(Equal("Ready"))
		Expect(readyCondition.Status).To(Equal(corev1.ConditionTrue))
		Expect(readyCondition.Reason).To(Equal("SuccessfulCreateOrUpdate"))
		Expect(readyCondition.LastTransitionTime).NotTo(Equal(metav1.Time{}))

		By("setting status.observedGeneration")
		Expect(updatedReplication.Status.ObservedGeneration).To(Equal(updatedReplication.GetGeneration()))

		By("not allowing updates on rabbitmqClusterReference")
		updateTest := topology.SchemaReplication{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: replication.Name, Namespace: replication.Namespace}, &updateTest)).To(Succeed())
		updateTest.Spec.RabbitmqClusterReference.Name = "new-cluster"
		Expect(k8sClient.Update(ctx, &updateTest).Error()).To(ContainSubstring("spec.rabbitmqClusterReference: Forbidden: update on rabbitmqClusterReference is forbidden"))

		By("unsetting schema replication upstream global parameters on deletion")
		Expect(k8sClient.Delete(ctx, replication)).To(Succeed())
		Eventually(func() []rabbithole.GlobalRuntimeParameter {
			var err error
			allGlobalParams, err = rabbitClient.ListGlobalParameters()
			Expect(err).NotTo(HaveOccurred())
			return allGlobalParams
		}, 5, 2).Should(HaveLen(2)) // cluster_name and internal_cluster_id are set by default by RabbitMQ
	})
})
