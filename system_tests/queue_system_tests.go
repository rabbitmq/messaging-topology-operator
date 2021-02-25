package system_tests

import (
	"context"
	rabbithole "github.com/michaelklishin/rabbit-hole/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"

	topologyv1beta1 "github.com/rabbitmq/messaging-topology-operator/api/v1beta1"
)

var _ = Describe("Queue Controller", func() {
	var (
		namespace = MustHaveEnv("NAMESPACE")
		ctx       = context.Background()
		q         *topologyv1beta1.Queue
	)

	BeforeEach(func() {
		q = &topologyv1beta1.Queue{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "queue-test",
				Namespace: namespace,
			},
			Spec: topologyv1beta1.QueueSpec{
				RabbitmqClusterReference: topologyv1beta1.RabbitmqClusterReference{
					Name:      rmq.Name,
					Namespace: rmq.Namespace,
				},
				Type:       "quorum",
				AutoDelete: false,
				Durable:    true,
				Arguments: &runtime.RawExtension{
					Raw: []byte(`{"x-quorum-initial-group-size": 3}`),
				},
			},
		}
	})

	It("declares and deletes a queue successfully", func() {
		By("declaring queue")
		Expect(k8sClient.Create(ctx, q, &client.CreateOptions{})).To(Succeed())
		var qInfo *rabbithole.DetailedQueueInfo
		Eventually(func() error {
			var err error
			qInfo, err = rabbitClient.GetQueue(q.Spec.Vhost, q.Name)
			return err
		}, 10, 2).Should(BeNil())

		Expect(*qInfo).To(MatchFields(IgnoreExtras, Fields{
			"Name":       Equal(q.Name),
			"Vhost":      Equal(q.Spec.Vhost),
			"AutoDelete": BeFalse(),
			"Durable":    BeTrue(),
		}))
		Expect(qInfo.Arguments).To(HaveKeyWithValue("x-quorum-initial-group-size", float64(3)))
		Expect(qInfo.Arguments).To(HaveKeyWithValue("x-queue-type", "quorum"))

		By("deleting queue")
		Expect(k8sClient.Delete(ctx, q)).To(Succeed())
		_, err := rabbitClient.GetQueue(q.Spec.Vhost, q.Name)
		Expect(err.Error()).To(ContainSubstring("Object Not Found"))
	})
})
