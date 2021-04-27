package v1beta1

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("queue webhook", func() {

	var queue = Queue{
		ObjectMeta: metav1.ObjectMeta{
			Name: "update-binding",
		},
		Spec: QueueSpec{
			Name:       "test",
			Vhost:      "/a-vhost",
			Type:       "quorum",
			Durable:    false,
			AutoDelete: true,
			RabbitmqClusterReference: RabbitmqClusterReference{
				Name: "some-cluster",
			},
		},
	}

	It("does not allow updates on queue name", func() {
		newQueue := queue.DeepCopy()
		newQueue.Spec.Name = "new-name"
		Expect(apierrors.IsForbidden(newQueue.ValidateUpdate(&queue))).To(BeTrue())
	})

	It("does not allow updates on vhost", func() {
		newQueue := queue.DeepCopy()
		newQueue.Spec.Vhost = "/new-vhost"
		Expect(apierrors.IsForbidden(newQueue.ValidateUpdate(&queue))).To(BeTrue())
	})

	It("does not allow updates on RabbitmqClusterReference", func() {
		newQueue := queue.DeepCopy()
		newQueue.Spec.RabbitmqClusterReference = RabbitmqClusterReference{
			Name: "new-cluster",
		}
		Expect(apierrors.IsForbidden(newQueue.ValidateUpdate(&queue))).To(BeTrue())
	})

	It("does not allow updates on queue type", func() {
		newQueue := queue.DeepCopy()
		newQueue.Spec.Type = "classic"
		Expect(apierrors.IsInvalid(newQueue.ValidateUpdate(&queue))).To(BeTrue())
	})

	It("does not allow updates on durable", func() {
		newQueue := queue.DeepCopy()
		newQueue.Spec.Durable = true
		Expect(apierrors.IsInvalid(newQueue.ValidateUpdate(&queue))).To(BeTrue())
	})

	It("does not allow updates on autoDelete", func() {
		newQueue := queue.DeepCopy()
		newQueue.Spec.AutoDelete = false
		Expect(apierrors.IsInvalid(newQueue.ValidateUpdate(&queue))).To(BeTrue())
	})
})
