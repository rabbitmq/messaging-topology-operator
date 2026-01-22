package v1beta1

import (
	"context"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("user webhook", func() {
	var (
		user = User{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test",
			},
			Spec: UserSpec{
				Tags: []UserTag{"policymaker"},
				RabbitmqClusterReference: RabbitmqClusterReference{
					Name: "a-cluster",
				},
			},
		}
		rootCtx       = context.Background()
		userValidator UserValidator
	)

	Context("ValidateCreate", func() {
		It("does not allow both spec.rabbitmqClusterReference.name and spec.rabbitmqClusterReference.connectionSecret be configured", func() {
			notAllowed := user.DeepCopy()
			notAllowed.Spec.RabbitmqClusterReference.ConnectionSecret = &corev1.LocalObjectReference{Name: "some-secret"}
			_, err := userValidator.ValidateCreate(rootCtx, notAllowed)
			Expect(err).To(MatchError(ContainSubstring("invalid RabbitmqClusterReference: do not provide both name and connectionSecret")))
		})

		It("spec.rabbitmqClusterReference.name and spec.rabbitmqClusterReference.connectionSecret cannot both be empty", func() {
			notAllowed := user.DeepCopy()
			notAllowed.Spec.RabbitmqClusterReference.Name = ""
			notAllowed.Spec.RabbitmqClusterReference.ConnectionSecret = nil
			_, err := userValidator.ValidateCreate(rootCtx, notAllowed)
			Expect(err).To(MatchError(ContainSubstring("invalid RabbitmqClusterReference: must provide either name or connectionSecret")))
		})
	})

	Context("ValidateUpdate", func() {
		It("does not allow updates on RabbitmqClusterReference", func() {
			newUser := user.DeepCopy()
			newUser.Spec.RabbitmqClusterReference = RabbitmqClusterReference{
				Name: "newUser-cluster",
			}
			_, err := userValidator.ValidateUpdate(rootCtx, &user, newUser)
			Expect(err).To(MatchError(ContainSubstring("update on rabbitmqClusterReference is forbidden")))
		})

		It("does not allow updates on rabbitmqClusterReference.connectionSecret", func() {
			connectionScr := User{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
				},
				Spec: UserSpec{
					Tags: []UserTag{"policymaker"},
					RabbitmqClusterReference: RabbitmqClusterReference{
						ConnectionSecret: &corev1.LocalObjectReference{
							Name: "a-secret",
						},
					},
				},
			}
			newUser := connectionScr.DeepCopy()
			newUser.Spec.RabbitmqClusterReference.Name = "a-name"
			newUser.Spec.RabbitmqClusterReference.ConnectionSecret = nil
			_, err := userValidator.ValidateUpdate(rootCtx, &connectionScr, newUser)
			Expect(err).To(MatchError(ContainSubstring("update on rabbitmqClusterReference is forbidden")))
		})

		It("allows update on tags", func() {
			newUser := user.DeepCopy()
			newUser.Spec.Tags = []UserTag{"monitoring"}
			_, err := userValidator.ValidateUpdate(rootCtx, &user, newUser)
			Expect(err).ToNot(HaveOccurred())
		})
	})
})
