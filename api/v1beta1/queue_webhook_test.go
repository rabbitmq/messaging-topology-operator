package v1beta1

import (
	"context"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("queue webhook", func() {

	var (
		queue = Queue{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-queue",
			},
			Spec: QueueSpec{
				Name:           "test",
				Vhost:          "/a-vhost",
				Type:           "quorum",
				Durable:        false,
				AutoDelete:     true,
				DeleteIfEmpty:  true,
				DeleteIfUnused: false,
				RabbitmqClusterReference: RabbitmqClusterReference{
					Name: "some-cluster",
				},
			},
		}
		rootCtx = context.Background()
	)

	Context("ValidateCreate", func() {
		It("does not allow both spec.rabbitmqClusterReference.name and spec.rabbitmqClusterReference.connectionSecret be configured", func() {
			notAllowedQ := queue.DeepCopy()
			notAllowedQ.Spec.Durable = true
			notAllowedQ.Spec.RabbitmqClusterReference.ConnectionSecret = &corev1.LocalObjectReference{Name: "some-secret"}
			_, err := notAllowedQ.ValidateCreate(rootCtx, notAllowedQ)
			Expect(err).To(MatchError(ContainSubstring("invalid RabbitmqClusterReference: do not provide both name and connectionSecret")))
		})

		It("spec.rabbitmqClusterReference.name and spec.rabbitmqClusterReference.connectionSecret cannot both be empty", func() {
			notAllowedQ := queue.DeepCopy()
			notAllowedQ.Spec.Durable = true
			notAllowedQ.Spec.RabbitmqClusterReference.Name = ""
			notAllowedQ.Spec.RabbitmqClusterReference.ConnectionSecret = nil
			_, err := notAllowedQ.ValidateCreate(rootCtx, notAllowedQ)
			Expect(err).To(MatchError(ContainSubstring("invalid RabbitmqClusterReference: must provide either name or connectionSecret")))
		})

		It("does not allow non-durable quorum queues", func() {
			notAllowedQ := queue.DeepCopy()
			notAllowedQ.Spec.AutoDelete = false
			_, err := notAllowedQ.ValidateCreate(rootCtx, notAllowedQ)
			Expect(err).To(MatchError(ContainSubstring("Quorum queues must have durable set to true")))
		})
	})

	Context("ValidateUpdate", func() {
		It("does not allow updates on queue name", func() {
			newQueue := queue.DeepCopy()
			newQueue.Spec.Name = "new-name"
			_, err := newQueue.ValidateUpdate(rootCtx, &queue, newQueue)
			Expect(err).To(MatchError(ContainSubstring("updates on name, vhost, and rabbitmqClusterReference are all forbidden")))
		})

		It("does not allow updates on vhost", func() {
			newQueue := queue.DeepCopy()
			newQueue.Spec.Vhost = "/new-vhost"
			_, err := newQueue.ValidateUpdate(rootCtx, &queue, newQueue)
			Expect(err).To(MatchError(ContainSubstring("updates on name, vhost, and rabbitmqClusterReference are all forbidden")))
		})

		It("does not allow updates on rabbitmqClusterReference.name", func() {
			newQueue := queue.DeepCopy()
			newQueue.Spec.RabbitmqClusterReference = RabbitmqClusterReference{
				Name: "new-cluster",
			}
			_, err := newQueue.ValidateUpdate(rootCtx, &queue, newQueue)
			Expect(err).To(MatchError(ContainSubstring("updates on name, vhost, and rabbitmqClusterReference are all forbidden")))
		})

		It("does not allow updates on rabbitmqClusterReference.namespace", func() {
			newQueue := queue.DeepCopy()
			newQueue.Spec.RabbitmqClusterReference = RabbitmqClusterReference{
				Namespace: "new-ns",
			}
			_, err := newQueue.ValidateUpdate(rootCtx, &queue, newQueue)
			Expect(err).To(MatchError(ContainSubstring("updates on name, vhost, and rabbitmqClusterReference are all forbidden")))
		})

		It("does not allow updates on rabbitmqClusterReference.connectionSecret", func() {
			connectionScrQ := Queue{
				ObjectMeta: metav1.ObjectMeta{
					Name: "connect-test-queue",
				},
				Spec: QueueSpec{
					Name: "test",
					RabbitmqClusterReference: RabbitmqClusterReference{
						ConnectionSecret: &corev1.LocalObjectReference{
							Name: "a-secret",
						},
					},
				},
			}
			newQueue := connectionScrQ.DeepCopy()
			newQueue.Spec.RabbitmqClusterReference.ConnectionSecret.Name = "new-secret"
			_, err := newQueue.ValidateUpdate(rootCtx, &connectionScrQ, newQueue)
			Expect(err).To(MatchError(ContainSubstring("updates on name, vhost, and rabbitmqClusterReference are all forbidden")))
		})

		It("does not allow updates on queue type", func() {
			newQueue := queue.DeepCopy()
			newQueue.Spec.Type = "classic"
			_, err := newQueue.ValidateUpdate(rootCtx, &queue, newQueue)
			Expect(err).To(MatchError(ContainSubstring("queue type cannot be updated")))
		})

		It("does not allow updates on durable", func() {
			newQueue := queue.DeepCopy()
			newQueue.Spec.Durable = true
			_, err := newQueue.ValidateUpdate(rootCtx, &queue, newQueue)
			Expect(err).To(MatchError(ContainSubstring("durable cannot be updated")))
		})

		It("does not allow updates on autoDelete", func() {
			newQueue := queue.DeepCopy()
			newQueue.Spec.AutoDelete = false
			_, err := newQueue.ValidateUpdate(rootCtx, &queue, newQueue)
			Expect(err).To(MatchError(ContainSubstring("autoDelete cannot be updated")))
		})
	})
})
