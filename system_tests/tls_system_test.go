package system_tests

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	rabbitmqv1beta1 "github.com/rabbitmq/cluster-operator/api/v1beta1"
	topology "github.com/rabbitmq/messaging-topology-operator/api/v1alpha2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Management over TLS", func() {
	var (
		namespace        = MustHaveEnv("NAMESPACE")
		ctx              = context.Background()
		targetCluster    *rabbitmqv1beta1.RabbitmqCluster
		targetClusterRef topology.RabbitmqClusterReference
		exchange         topology.Exchange
		policy           topology.Policy
		queue            topology.Queue
		user             topology.User
		vhost            topology.Vhost
	)

	BeforeEach(func() {
		targetCluster = basicTestRabbitmqCluster("tls-cluster", namespace)
		targetCluster.Spec.Service.Type = "ClusterIP"
		setupTestRabbitmqCluster(k8sClient, targetCluster)

		secretName := fmt.Sprintf("rmq-test-cert-%v", uuid.New())
		hostname := clusterIP(ctx, clientSet, namespace, "tls-cluster")
		_, _, _ = createTLSSecret(secretName, namespace, hostname)

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

		targetCluster.Spec.TLS.SecretName = secretName
		targetCluster.Spec.TLS.DisableNonTLSListeners = true
		updateTestRabbitmqCluster(k8sClient, targetCluster)

		targetClusterRef = topology.RabbitmqClusterReference{Name: targetCluster.Name}
	})

	AfterEach(func() {
		Expect(k8sClient.Delete(ctx, &exchange)).To(Succeed())
		Eventually(func() string {
			output, _ := kubectl(
				"-n",
				exchange.Namespace,
				"get",
				"exchange",
				exchange.Name,
			)
			return string(output)
		}, 90, 10).Should(ContainSubstring("not found"))
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
		Expect(k8sClient.Delete(ctx, &queue)).To(Succeed())
		Eventually(func() string {
			output, _ := kubectl(
				"-n",
				queue.Namespace,
				"get",
				"queue",
				queue.Name,
			)
			return string(output)
		}, 90, 10).Should(ContainSubstring("not found"))
		Expect(k8sClient.Delete(ctx, &user)).To(Succeed())
		Eventually(func() string {
			output, _ := kubectl(
				"-n",
				user.Namespace,
				"get",
				"user",
				user.Name,
			)
			return string(output)
		}, 90, 10).Should(ContainSubstring("not found"))
		Expect(k8sClient.Delete(ctx, &vhost)).To(Succeed())
		Eventually(func() string {
			output, _ := kubectl(
				"-n",
				vhost.Namespace,
				"get",
				"vhost",
				vhost.Name,
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
		exchange = topology.Exchange{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "exchange-tls-test",
				Namespace: namespace,
			},
			Spec: topology.ExchangeSpec{
				Name:                     "exchange-tls-test",
				RabbitmqClusterReference: targetClusterRef,
			},
		}
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
		queue = topology.Queue{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "queue-tls-test",
				Namespace: namespace,
			},
			Spec: topology.QueueSpec{
				Name:                     "queue-tls-test",
				RabbitmqClusterReference: targetClusterRef,
			},
		}
		user = topology.User{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "user-tls-test",
				Namespace: namespace,
			},
			Spec: topology.UserSpec{
				RabbitmqClusterReference: targetClusterRef,
			},
		}
		vhost = topology.Vhost{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "vhost-tls-test",
				Namespace: namespace,
			},
			Spec: topology.VhostSpec{
				Name:                     "vhost-tls-test",
				RabbitmqClusterReference: targetClusterRef,
			},
		}
		Expect(k8sClient.Create(ctx, &exchange)).To(Succeed())
		Expect(k8sClient.Create(ctx, &policy)).To(Succeed())
		Expect(k8sClient.Create(ctx, &queue)).To(Succeed())
		Expect(k8sClient.Create(ctx, &user)).To(Succeed())
		Expect(k8sClient.Create(ctx, &vhost)).To(Succeed())
	})
})
