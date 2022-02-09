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
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("RabbitmqCluster with TLS", func() {
	var (
		namespace        = MustHaveEnv("NAMESPACE")
		ctx              = context.Background()
		targetCluster    *rabbitmqv1beta1.RabbitmqCluster
		targetClusterRef topology.RabbitmqClusterReference
		policy           topology.Policy
		exchange         topology.Exchange
		tlsSecretName    string
		connectionSecret *corev1.Secret
	)

	BeforeEach(func() {
		tlsSecretName = fmt.Sprintf("rmq-test-cert-%v", uuid.New())
		_, _, _ = createTLSSecret(tlsSecretName, namespace, "tls-cluster.rabbitmq-system.svc")

		patchBytes, _ := fixtures.ReadFile("fixtures/patch-test-ca.yaml")
		_, err := kubectl(
			"-n",
			namespace,
			"patch",
			"deployment",
			"messaging-topology-operator",
			"--patch",
			fmt.Sprintf(string(patchBytes), tlsSecretName+"-ca"),
		)
		Expect(err).NotTo(HaveOccurred())

		targetCluster = basicTestRabbitmqCluster("tls-cluster", namespace)
		targetCluster.Spec.TLS.SecretName = tlsSecretName
		targetCluster.Spec.TLS.DisableNonTLSListeners = true
		setupTestRabbitmqCluster(k8sClient, targetCluster)
		targetClusterRef = topology.RabbitmqClusterReference{Name: targetCluster.Name}

		user, pass, err := getUsernameAndPassword(ctx, clientSet, targetCluster.Namespace, targetCluster.Name)
		Expect(err).NotTo(HaveOccurred(), "failed to get user and pass")
		connectionSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "uri-secret",
				Namespace: namespace,
			},
			StringData: map[string]string{
				"username": user,
				"password": pass,
				"uri":      fmt.Sprintf("https://%s:15671", "tls-cluster.rabbitmq-system.svc"),
			},
		}
		Expect(k8sClient.Create(ctx, connectionSecret, &client.CreateOptions{})).To(Succeed())
		Eventually(func() string {
			output, err := kubectl(
				"-n",
				namespace,
				"get",
				"secrets",
				connectionSecret.Name,
			)
			if err != nil {
				Expect(string(output)).To(ContainSubstring("NotFound"))
			}
			return string(output)
		}, 10).Should(ContainSubstring("uri-secret"))
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
		Expect(k8sClient.Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: connectionSecret.Name, Namespace: targetCluster.Namespace}})).To(Succeed())
		Expect(k8sClient.Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: tlsSecretName, Namespace: targetCluster.Namespace}})).To(Succeed())
		Expect(k8sClient.Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: tlsSecretName + "-ca", Namespace: targetCluster.Namespace}})).To(Succeed())
	})

	It("succeeds creating objects on the TLS-enabled instance", func() {
		By("setting rabbitmqClusterReference.name")
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

		By("setting rabbitmqClusterReference.connectionSecret instead of RabbitmqClusterName")
		exchange = topology.Exchange{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "tls-test",
				Namespace: namespace,
			},
			Spec: topology.ExchangeSpec{
				Name:       "tls-test",
				Type:       "direct",
				AutoDelete: false,
				Durable:    true,
				RabbitmqClusterReference: topology.RabbitmqClusterReference{
					ConnectionSecret: &corev1.LocalObjectReference{Name: connectionSecret.Name},
				},
			},
		}
		Expect(k8sClient.Create(ctx, &exchange)).To(Succeed())

		var fetched topology.Exchange
		Eventually(func() []topology.Condition {
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: exchange.Name, Namespace: exchange.Namespace}, &fetched)).To(Succeed())
			return fetched.Status.Conditions
		}, 10, 2).Should(HaveLen(1))

		readyCondition = fetched.Status.Conditions[0]
		Expect(string(readyCondition.Type)).To(Equal("Ready"))
		Expect(readyCondition.Status).To(Equal(corev1.ConditionTrue))
		Expect(readyCondition.Reason).To(Equal("SuccessfulCreateOrUpdate"))
	})
})
