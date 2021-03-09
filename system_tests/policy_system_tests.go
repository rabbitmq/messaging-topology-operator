package system_tests

import (
	"context"
	rabbithole "github.com/michaelklishin/rabbit-hole/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"

	topologyv1alpha1 "github.com/rabbitmq/messaging-topology-operator/api/v1alpha1"
)

var _ = Describe("Policy", func() {
	var (
		namespace = MustHaveEnv("NAMESPACE")
		ctx       = context.Background()
		policy    *topologyv1alpha1.Policy
	)

	BeforeEach(func() {
		policy = &topologyv1alpha1.Policy{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "policy-test",
				Namespace: namespace,
			},
			Spec: topologyv1alpha1.PolicySpec{
				RabbitmqClusterReference: topologyv1alpha1.RabbitmqClusterReference{
					Name:      rmq.Name,
					Namespace: rmq.Namespace,
				},
				Name:    "policy-test",
				Pattern: "test-queue",
				ApplyTo: "queues",
				Definition: &runtime.RawExtension{
					Raw: []byte(`{"ha-mode":"all"}`),
				},
			},
		}
	})

	It("creates, updates and deletes a policy successfully", func() {
		By("creating policy")
		Expect(k8sClient.Create(ctx, policy, &client.CreateOptions{})).To(Succeed())
		var fetchedPolicy *rabbithole.Policy
		Eventually(func() error {
			var err error
			fetchedPolicy, err = rabbitClient.GetPolicy(policy.Spec.Vhost, policy.Name)
			return err
		}, 10, 2).Should(BeNil())

		Expect(*fetchedPolicy).To(MatchFields(IgnoreExtras, Fields{
			"Name":     Equal(policy.Spec.Name),
			"Vhost":    Equal(policy.Spec.Vhost),
			"Pattern":  Equal("test-queue"),
			"ApplyTo":  Equal("queues"),
			"Priority": Equal(0),
		}))

		Expect(fetchedPolicy.Definition).To(HaveKeyWithValue("ha-mode", "all"))

		By("updating policy")
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: policy.Name, Namespace: policy.Namespace}, policy)).To(Succeed())
		policy.Spec.Definition = &runtime.RawExtension{
			Raw: []byte(`{"ha-mode":"exactly",
"ha-params": 2
}`)}
		Expect(k8sClient.Update(ctx, policy, &client.UpdateOptions{})).To(Succeed())

		Eventually(func() rabbithole.PolicyDefinition {
			var err error
			fetchedPolicy, err = rabbitClient.GetPolicy(policy.Spec.Vhost, policy.Name)
			Expect(err).NotTo(HaveOccurred())
			return fetchedPolicy.Definition
		}, 10, 2).Should(HaveLen(2))

		Expect(fetchedPolicy.Definition).To(HaveKeyWithValue("ha-mode", "exactly"))
		Expect(fetchedPolicy.Definition).To(HaveKeyWithValue("ha-params", float64(2)))

		By("deleting policy")
		Expect(k8sClient.Delete(ctx, policy)).To(Succeed())
		var err error
		Eventually(func() error {
			_, err = rabbitClient.GetPolicy(policy.Spec.Vhost, policy.Name)
			return err
		}, 10).Should(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("Object Not Found"))
	})
})
