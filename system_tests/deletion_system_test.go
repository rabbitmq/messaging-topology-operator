package system_tests

import (
	"context"

	rabbitmqv1beta1 "github.com/rabbitmq/cluster-operator/api/v1beta1"
	topologyv1alpha1 "github.com/rabbitmq/messaging-topology-operator/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Deletion", func() {
	var (
		namespace     = MustHaveEnv("NAMESPACE")
		ctx           = context.Background()
		targetCluster *rabbitmqv1beta1.RabbitmqCluster
		exchange      topologyv1alpha1.Exchange
		policy        topologyv1alpha1.Policy
		queue         topologyv1alpha1.Queue
		user          topologyv1alpha1.User
		vhost         topologyv1alpha1.Vhost
	)

	BeforeEach(func() {
		targetCluster = setupTestRabbitmqCluster(k8sClient, "to-be-deleted", namespace)
		targetClusterRef := topologyv1alpha1.RabbitmqClusterReference{Name: targetCluster.Name, Namespace: targetCluster.Namespace}
		exchange = topologyv1alpha1.Exchange{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "exchange-deletion-test",
				Namespace: namespace,
			},
			Spec: topologyv1alpha1.ExchangeSpec{
				Name:                     "exchange-deletion-test",
				RabbitmqClusterReference: targetClusterRef,
			},
		}
		policy = topologyv1alpha1.Policy{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "policy-deletion-test",
				Namespace: namespace,
			},
			Spec: topologyv1alpha1.PolicySpec{
				Name:    "polocy-deletion-test",
				Pattern: ".*",
				ApplyTo: "queues",
				Definition: &runtime.RawExtension{
					Raw: []byte(`{"ha-mode":"all"}`),
				},
				RabbitmqClusterReference: targetClusterRef,
			},
		}
		queue = topologyv1alpha1.Queue{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "queue-deletion-test",
				Namespace: namespace,
			},
			Spec: topologyv1alpha1.QueueSpec{
				Name:                     "queue-deletion-test",
				RabbitmqClusterReference: targetClusterRef,
			},
		}
		user = topologyv1alpha1.User{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "user-deletion-test",
				Namespace: namespace,
			},
			Spec: topologyv1alpha1.UserSpec{
				RabbitmqClusterReference: targetClusterRef,
			},
		}
		vhost = topologyv1alpha1.Vhost{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "vhost-deletion-test",
				Namespace: namespace,
			},
			Spec: topologyv1alpha1.VhostSpec{
				Name:                     "vhost-deletion-test",
				RabbitmqClusterReference: targetClusterRef,
			},
		}
		Expect(k8sClient.Create(ctx, &exchange)).To(Succeed())
		Expect(k8sClient.Create(ctx, &policy)).To(Succeed())
		Expect(k8sClient.Create(ctx, &queue)).To(Succeed())
		Expect(k8sClient.Create(ctx, &user)).To(Succeed())
		Expect(k8sClient.Create(ctx, &vhost)).To(Succeed())
	})

	It("handles the referenced RabbitmqCluster being deleted", func() {
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
		By("allowing the topology objects to be deleted")
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
		}, 30, 10).Should(ContainSubstring("not found"))
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
		}, 30, 10).Should(ContainSubstring("not found"))
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
		}, 30, 10).Should(ContainSubstring("not found"))
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
		}, 30, 10).Should(ContainSubstring("not found"))
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
		}, 30, 10).Should(ContainSubstring("not found"))
	})
})
