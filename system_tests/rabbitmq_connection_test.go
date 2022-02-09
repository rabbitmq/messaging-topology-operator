package system_tests

import (
	"context"
	rabbithole "github.com/michaelklishin/rabbit-hole/v2"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	topology "github.com/rabbitmq/messaging-topology-operator/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("RabbitMQ connection using provided connection secret", func() {
	var (
		namespace = MustHaveEnv("NAMESPACE")
		ctx       = context.Background()
		q         *topology.Queue
		secret    *corev1.Secret
	)

	BeforeEach(func() {
		endpoint, err := managementURI(ctx, clientSet, rmq.Namespace, rmq.Name)
		Expect(err).NotTo(HaveOccurred(), "failed to get management uri")
		user, pass, err := getUsernameAndPassword(ctx, clientSet, rmq.Namespace, rmq.Name)
		Expect(err).NotTo(HaveOccurred(), "failed to get user and pass")

		secret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "uri-secret",
				Namespace: namespace,
			},
			StringData: map[string]string{
				"username": user,
				"password": pass,
				"uri":      endpoint,
			},
		}
		Expect(k8sClient.Create(ctx, secret, &client.CreateOptions{})).To(Succeed())
	})

	AfterEach(func() {
		Expect(k8sClient.Delete(ctx, secret)).To(Succeed())
	})

	It("succeeds creating an object in a RabbitMQ cluster configured with connection URI", func() {
		By("declaring queue")
		q = &topology.Queue{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "connection-test",
				Namespace: namespace,
			},
			Spec: topology.QueueSpec{
				Name: "connection-test",
				RabbitmqClusterReference: topology.RabbitmqClusterReference{
					ConnectionSecret: &corev1.LocalObjectReference{Name: secret.Name},
				},
			},
		}
		Expect(k8sClient.Create(ctx, q, &client.CreateOptions{})).To(Succeed())
		var qInfo *rabbithole.DetailedQueueInfo
		Eventually(func() error {
			var err error
			qInfo, err = rabbitClient.GetQueue("/", q.Name)
			return err
		}, 10, 2).Should(BeNil())

		Expect(qInfo.Name).To(Equal(q.Name))
		Expect(qInfo.Vhost).To(Equal("/"))

		By("deleting queue")
		Expect(k8sClient.Delete(ctx, q)).To(Succeed())
		var err error
		Eventually(func() error {
			_, err = rabbitClient.GetQueue(q.Spec.Vhost, q.Name)
			return err
		}, 30).Should(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("Object Not Found"))
	})
})
