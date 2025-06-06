package system_tests

import (
	"context"
	rabbithole "github.com/michaelklishin/rabbit-hole/v3"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"

	topology "github.com/rabbitmq/messaging-topology-operator/api/v1beta1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

var _ = Describe("Queue Controller", func() {
	var (
		namespace = MustHaveEnv("NAMESPACE")
		ctx       = context.Background()
		q         *topology.Queue
	)

	BeforeEach(func() {
		q = &topology.Queue{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "queue-test",
				Namespace: namespace,
			},
			Spec: topology.QueueSpec{
				RabbitmqClusterReference: topology.RabbitmqClusterReference{
					Name: rmq.Name,
				},
				Name:       "queue-test",
				Type:       "quorum",
				AutoDelete: false,
				Durable:    true,
				Arguments: &runtime.RawExtension{
					Raw: []byte(`{"x-quorum-initial-group-size": 3}`),
				},
			},
		}
	})

	AfterEach(func() {
		_ = k8sClient.Delete(ctx, q)
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
			"Name":       Equal(q.Spec.Name),
			"Vhost":      Equal(q.Spec.Vhost),
			"AutoDelete": Equal(rabbithole.AutoDelete(false)),
			"Durable":    BeTrue(),
		}))
		Expect(qInfo.Arguments).To(HaveKeyWithValue("x-quorum-initial-group-size", float64(3)))
		Expect(qInfo.Arguments).To(HaveKeyWithValue("x-queue-type", "quorum"))

		By("updating status condition 'Ready'")
		updatedQueue := topology.Queue{}

		Eventually(func() []topology.Condition {
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: q.Name, Namespace: q.Namespace}, &updatedQueue)).To(Succeed())
			return updatedQueue.Status.Conditions
		}, waitUpdatedStatusCondition, 2).Should(HaveLen(1), "Queue status condition should be present")

		readyCondition := updatedQueue.Status.Conditions[0]
		Expect(string(readyCondition.Type)).To(Equal("Ready"))
		Expect(readyCondition.Status).To(Equal(corev1.ConditionTrue))
		Expect(readyCondition.Reason).To(Equal("SuccessfulCreateOrUpdate"))
		Expect(readyCondition.LastTransitionTime).NotTo(Equal(metav1.Time{}))

		By("setting correct finalizer")
		Expect(updatedQueue.ObjectMeta.Finalizers).To(ConsistOf("deletion.finalizers.queues.rabbitmq.com"))

		By("setting status.observedGeneration")
		Expect(updatedQueue.Status.ObservedGeneration).To(Equal(updatedQueue.GetGeneration()))

		By("not allowing certain updates")
		updateQ := topology.Queue{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: q.Name, Namespace: q.Namespace}, &updateQ)).To(Succeed())
		updateQ.Spec.Name = "a-new-name"
		Expect(k8sClient.Update(ctx, &updateQ).Error()).To(ContainSubstring("spec.name: Forbidden: updates on name, vhost, and rabbitmqClusterReference are all forbidden"))
		updateQ.Spec.Name = q.Spec.Name
		updateQ.Spec.Type = "classic"
		Expect(k8sClient.Update(ctx, &updateQ).Error()).To(ContainSubstring("spec.type: Invalid value: \"classic\": queue type cannot be updated"))

		By("deleting queue")
		Expect(k8sClient.Delete(ctx, q)).To(Succeed())
		var err error
		Eventually(func() error {
			_, err = rabbitClient.GetQueue(q.Spec.Vhost, q.Name)
			return err
		}, 30).Should(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("Object Not Found"))
	})

	When("deletion policy is retain", func() {
		It("deletes k8s resource but keeps the queue in RabbitMQ", func() {
			queueWithRetain := &topology.Queue{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "retain-policy-test",
					Namespace: namespace,
				},
				Spec: topology.QueueSpec{
					Name:           "retain-policy-test",
					Type:           "classic",
					Durable:        true,
					DeletionPolicy: "retain",
					RabbitmqClusterReference: topology.RabbitmqClusterReference{
						Name: rmq.Name,
					},
				},
			}

			By("creating a queue with retain policy")
			Expect(k8sClient.Create(ctx, queueWithRetain, &client.CreateOptions{})).To(Succeed())

			By("waiting for the queue to be created in RabbitMQ")
			Eventually(func() error {
				_, err := rabbitClient.GetQueue("/", queueWithRetain.Spec.Name)
				return err
			}, 30, 2).ShouldNot(HaveOccurred())

			By("deleting the k8s resource")
			Expect(k8sClient.Delete(ctx, queueWithRetain)).To(Succeed())

			By("verifying k8s resource is gone")
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: queueWithRetain.Name, Namespace: queueWithRetain.Namespace}, &topology.Queue{})
				return apierrors.IsNotFound(err)
			}, 30, 2).Should(BeTrue())

			By("verifying queue still exists in RabbitMQ")
			_, err := rabbitClient.GetQueue("/", queueWithRetain.Spec.Name)
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
