package v1beta1

import (
	"context"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("vhost webhook", func() {

	var (
		vhost = Vhost{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-vhost",
			},
			Spec: VhostSpec{
				Name:             "test",
				Tracing:          false,
				DefaultQueueType: "classic",
				RabbitmqClusterReference: RabbitmqClusterReference{
					Name: "a-cluster",
				},
			},
		}
		rootCtx        = context.Background()
		vhostValidator VhostValidator
	)

	Context("ValidateCreate", func() {
		It("does not allow both spec.rabbitmqClusterReference.name and spec.rabbitmqClusterReference.connectionSecret be configured", func() {
			notAllowed := vhost.DeepCopy()
			notAllowed.Spec.RabbitmqClusterReference.ConnectionSecret = &corev1.LocalObjectReference{Name: "some-secret"}
			_, err := vhostValidator.ValidateCreate(rootCtx, notAllowed)
			Expect(err).To(MatchError(ContainSubstring("invalid RabbitmqClusterReference: do not provide both name and connectionSecret")))
		})

		It("spec.rabbitmqClusterReference.name and spec.rabbitmqClusterReference.connectionSecret cannot both be empty", func() {
			notAllowed := vhost.DeepCopy()
			notAllowed.Spec.RabbitmqClusterReference.Name = ""
			notAllowed.Spec.RabbitmqClusterReference.ConnectionSecret = nil
			_, err := vhostValidator.ValidateCreate(rootCtx, notAllowed)
			Expect(err).To(MatchError(ContainSubstring("invalid RabbitmqClusterReference: must provide either name or connectionSecret")))
		})
	})

	Context("ValidateUpdate", func() {
		It("does not allow updates on vhost name", func() {
			newVhost := vhost.DeepCopy()
			newVhost.Spec.Name = "new-name"
			_, err := vhostValidator.ValidateUpdate(rootCtx, &vhost, newVhost)
			Expect(err).To(MatchError(ContainSubstring("updates on name and rabbitmqClusterReference are all forbidden")))
		})

		It("does not allow updates on RabbitmqClusterReference", func() {
			newVhost := vhost.DeepCopy()
			newVhost.Spec.RabbitmqClusterReference = RabbitmqClusterReference{
				Name: "new-cluster",
			}
			_, err := vhostValidator.ValidateUpdate(rootCtx, &vhost, newVhost)
			Expect(err).To(MatchError(ContainSubstring("updates on name and rabbitmqClusterReference are all forbidden")))
		})

		It("does not allow updates on rabbitmqClusterReference.connectionSecret", func() {
			connectionScr := Vhost{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-vhost",
				},
				Spec: VhostSpec{
					Name: "test",
					RabbitmqClusterReference: RabbitmqClusterReference{
						ConnectionSecret: &corev1.LocalObjectReference{
							Name: "a-secret",
						},
					},
				},
			}
			newVhost := connectionScr.DeepCopy()
			newVhost.Spec.RabbitmqClusterReference.Name = "a-name"
			newVhost.Spec.RabbitmqClusterReference.ConnectionSecret = nil
			_, err := vhostValidator.ValidateUpdate(rootCtx, &connectionScr, newVhost)
			Expect(err).To(MatchError(ContainSubstring("updates on name and rabbitmqClusterReference are all forbidden")))
		})

		It("allows updates on vhost.spec.tracing", func() {
			newVhost := vhost.DeepCopy()
			newVhost.Spec.Tracing = true
			_, err := vhostValidator.ValidateUpdate(rootCtx, &vhost, newVhost)
			Expect(err).ToNot(HaveOccurred())
		})

		It("allows updates on vhost.spec.tags", func() {
			newVhost := vhost.DeepCopy()
			newVhost.Spec.Tags = []string{"new-tag"}
			_, err := vhostValidator.ValidateUpdate(rootCtx, &vhost, newVhost)
			Expect(err).ToNot(HaveOccurred())
		})

		It("allows updates on vhost.spec.defaultQueueType", func() {
			newVhost := vhost.DeepCopy()
			newVhost.Spec.DefaultQueueType = "quorum"
			_, err := vhostValidator.ValidateUpdate(rootCtx, &vhost, newVhost)
			Expect(err).ToNot(HaveOccurred())
		})

		It("allows updates on vhost.spec.deletionPolicy", func() {
			newVhost := vhost.DeepCopy()
			newVhost.Spec.DeletionPolicy = "retain"
			_, err := vhostValidator.ValidateUpdate(rootCtx, &vhost, newVhost)
			Expect(err).ToNot(HaveOccurred())
		})
	})
})
