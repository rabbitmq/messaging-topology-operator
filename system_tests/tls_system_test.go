package system_tests

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	rabbitmqv1beta1 "github.com/rabbitmq/cluster-operator/api/v1beta1"
	topology "github.com/rabbitmq/messaging-topology-operator/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("RabbitmqCluster with TLS", func() {
	var (
		namespace        = MustHaveEnv("NAMESPACE")
		ctx              = context.Background()
		targetCluster    *rabbitmqv1beta1.RabbitmqCluster
		targetClusterRef topology.RabbitmqClusterReference
		policy           topology.Policy
		secretName       string
	)

	BeforeEach(func() {
		secretName = fmt.Sprintf("rmq-test-cert-%v", uuid.New())
		_, _, _ = createTLSSecret(secretName, namespace, "tls-cluster.rabbitmq-system.svc")

		patchBytes, _ := fixtures.ReadFile("fixtures/patch-test-ca.yaml")
		_, err := kubectl(
			"-n",
			namespace,
			"patch",
			"deployment",
			"messaging-topology-operator",
			"--patch",
			fmt.Sprintf(string(patchBytes), secretName+"-ca"),
		)
		Expect(err).NotTo(HaveOccurred())

		targetCluster = basicTestRabbitmqCluster("tls-cluster", namespace)
		targetCluster.Spec.TLS.SecretName = secretName
		targetCluster.Spec.TLS.DisableNonTLSListeners = true
		setupTestRabbitmqCluster(k8sClient, targetCluster)
		targetClusterRef = topology.RabbitmqClusterReference{Name: targetCluster.Name}
	})

	AfterEach(func() {
		Expect(k8sClient.Delete(ctx, &policy)).To(Succeed())
		Eventually(func() string {
			output, _ := kubectl(
				"-n",
				policy.Namespace,
				"get",
				"policy",
				policy.Name,
			)
			return string(output)
		}, 90, 10).Should(ContainSubstring("NotFound"))
		Expect(k8sClient.Delete(ctx, &rabbitmqv1beta1.RabbitmqCluster{ObjectMeta: metav1.ObjectMeta{Name: targetCluster.Name, Namespace: targetCluster.Namespace}})).To(Succeed())
		Eventually(func() string {
			output, _ := kubectl(
				"-n",
				targetCluster.Namespace,
				"get",
				"rabbitmqclusters",
				targetCluster.Name,
			)
			return string(output)
		}, 90, 10).Should(ContainSubstring("NotFound"))
		Expect(k8sClient.Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: secretName, Namespace: targetCluster.Namespace}})).To(Succeed())
		Expect(k8sClient.Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: secretName + "-ca", Namespace: targetCluster.Namespace}})).To(Succeed())
	})

	It("succeeds creating objects on the TLS-enabled instance", func() {
		policy = topology.Policy{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "policy-tls-test",
				Namespace: namespace,
			},
			Spec: topology.PolicySpec{
				Name:    "policy-tls-test",
				Pattern: ".*",
				ApplyTo: "queues",
				Definition: &runtime.RawExtension{
					Raw: []byte(`{"ha-mode":"all"}`),
				},
				RabbitmqClusterReference: targetClusterRef,
			},
		}
		Expect(k8sClient.Create(ctx, &policy)).To(Succeed())

		var fetchedPolicy topology.Policy
		Eventually(func() []topology.Condition {
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: policy.Name, Namespace: policy.Namespace}, &fetchedPolicy)).To(Succeed())
			return fetchedPolicy.Status.Conditions
		}, 10, 2).Should(HaveLen(1))

		readyCondition := fetchedPolicy.Status.Conditions[0]
		Expect(string(readyCondition.Type)).To(Equal("Ready"))
		Expect(readyCondition.Status).To(Equal(corev1.ConditionTrue))
		Expect(readyCondition.Reason).To(Equal("SuccessfulCreateOrUpdate"))
	})
})
