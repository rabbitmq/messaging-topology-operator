package system_tests

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	rabbitmqv1beta1 "github.com/rabbitmq/cluster-operator/api/v1beta1"
	topology "github.com/rabbitmq/messaging-topology-operator/api/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("RabbitmqCluster with TLS", func() {
	var (
		namespace        = MustHaveEnv("NAMESPACE")
		ctx              = context.Background()
		targetCluster    *rabbitmqv1beta1.RabbitmqCluster
		targetClusterRef topology.RabbitmqClusterReference
		policy           topology.Policy
	)

	BeforeEach(func() {
		targetCluster = basicTestRabbitmqCluster("tls-cluster", namespace)
		setupTestRabbitmqCluster(k8sClient, targetCluster)

		secretName := fmt.Sprintf("rmq-test-cert-%v", uuid.New())
		_, _, _ = createTLSSecret(secretName, namespace, "tls-cluster.rabbitmq-system.svc.cluster.local")

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

		Expect(k8sClient.Get(context.Background(), types.NamespacedName{Name: targetCluster.Name, Namespace: targetCluster.Namespace}, targetCluster)).To(Succeed())
		targetCluster.Spec.TLS.SecretName = secretName
		targetCluster.Spec.TLS.DisableNonTLSListeners = true
		updateTestRabbitmqCluster(k8sClient, targetCluster)

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
		}, 90, 10).Should(ContainSubstring("not found"))
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
		}, 90, 10).Should(ContainSubstring("not found"))
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
	})
})
