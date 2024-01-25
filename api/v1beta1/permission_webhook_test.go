package v1beta1

import (
	"context"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("permission webhook", func() {
	var (
		permission = Permission{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test",
			},
			Spec: PermissionSpec{
				User:  "test-user",
				Vhost: "/a-vhost",
				Permissions: VhostPermissions{
					Configure: ".*",
					Read:      ".*",
					Write:     ".*",
				},
				RabbitmqClusterReference: RabbitmqClusterReference{
					Name: "a-cluster",
				},
			},
		}
		rootCtx = context.Background()
	)

	Context("ValidateCreate", func() {
		It("does not allow user and userReference to be specified at the same time", func() {
			invalidPermission := permission.DeepCopy()
			invalidPermission.Spec.UserReference = &corev1.LocalObjectReference{Name: "invalid"}
			invalidPermission.Spec.User = "test-user"
			_, err := invalidPermission.ValidateCreate(rootCtx, invalidPermission)
			Expect(err).To(MatchError(ContainSubstring("cannot specify spec.user and spec.userReference at the same time")))
		})
		It("does not allow both user and userReference to be unset", func() {
			invalidPermission := permission.DeepCopy()
			invalidPermission.Spec.UserReference = nil
			invalidPermission.Spec.User = ""
			_, err := invalidPermission.ValidateCreate(rootCtx, invalidPermission)
			Expect(err).To(MatchError(ContainSubstring("must specify either spec.user or spec.userReference")))
		})

		It("does not allow both spec.rabbitmqClusterReference.name and spec.rabbitmqClusterReference.connectionSecret be configured", func() {
			notAllowed := permission.DeepCopy()
			notAllowed.Spec.RabbitmqClusterReference.ConnectionSecret = &corev1.LocalObjectReference{Name: "some-secret"}
			_, err := notAllowed.ValidateCreate(rootCtx, notAllowed)
			Expect(err).To(MatchError(ContainSubstring("invalid RabbitmqClusterReference: do not provide both name and connectionSecret")))
		})

		It("spec.rabbitmqClusterReference.name and spec.rabbitmqClusterReference.connectionSecret cannot both be empty", func() {
			notAllowed := permission.DeepCopy()
			notAllowed.Spec.RabbitmqClusterReference.Name = ""
			notAllowed.Spec.RabbitmqClusterReference.ConnectionSecret = nil
			_, err := notAllowed.ValidateCreate(rootCtx, notAllowed)
			Expect(err).To(MatchError(ContainSubstring("invalid RabbitmqClusterReference: must provide either name or connectionSecret")))
		})
	})

	Context("ValidateUpdate", func() {
		It("does not allow updates on user", func() {
			newPermission := permission.DeepCopy()
			newPermission.Spec.User = "new-user"
			_, err := newPermission.ValidateUpdate(rootCtx, &permission, newPermission)
			Expect(err).To(MatchError(ContainSubstring("updates on user, userReference, vhost and rabbitmqClusterReference are all forbidden")))
		})

		It("does not allow updates on userReference", func() {
			permissionWithUserRef := permission.DeepCopy()
			permissionWithUserRef.Spec.User = ""
			permissionWithUserRef.Spec.UserReference = &corev1.LocalObjectReference{Name: "a-user"}
			newPermission := permissionWithUserRef.DeepCopy()
			newPermission.Spec.UserReference = &corev1.LocalObjectReference{Name: "a-new-user"}
			_, err := newPermission.ValidateUpdate(rootCtx, &permission, newPermission)
			Expect(err).To(MatchError(ContainSubstring("updates on user, userReference, vhost and rabbitmqClusterReference are all forbidden")))
		})

		It("does not allow updates on vhost", func() {
			newPermission := permission.DeepCopy()
			newPermission.Spec.Vhost = "new-vhost"
			_, err := newPermission.ValidateUpdate(rootCtx, &permission, newPermission)
			Expect(err).To(MatchError(ContainSubstring("updates on user, userReference, vhost and rabbitmqClusterReference are all forbidden")))
		})

		It("does not allow updates on RabbitmqClusterReference", func() {
			newPermission := permission.DeepCopy()
			newPermission.Spec.RabbitmqClusterReference = RabbitmqClusterReference{
				Name: "new-cluster",
			}
			_, err := newPermission.ValidateUpdate(rootCtx, &permission, newPermission)
			Expect(err).To(MatchError(ContainSubstring("updates on user, userReference, vhost and rabbitmqClusterReference are all forbidden")))
		})

		It("does not allow updates on rabbitmqClusterReference.connectionSecret", func() {
			connectionScr := Permission{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
				},
				Spec: PermissionSpec{
					User:  "test-user",
					Vhost: "/a-vhost",
					Permissions: VhostPermissions{
						Configure: ".*",
						Read:      ".*",
						Write:     ".*",
					},
					RabbitmqClusterReference: RabbitmqClusterReference{
						ConnectionSecret: &corev1.LocalObjectReference{
							Name: "a-secret",
						},
					},
				},
			}
			newPermission := connectionScr.DeepCopy()
			newPermission.Spec.RabbitmqClusterReference.ConnectionSecret.Name = "new-secret"
			_, err := newPermission.ValidateUpdate(rootCtx, &connectionScr, newPermission)
			Expect(err).To(MatchError(ContainSubstring("updates on user, userReference, vhost and rabbitmqClusterReference are all forbidden")))
		})

		It("allows updates on permission.spec.permissions.configure", func() {
			newPermission := permission.DeepCopy()
			newPermission.Spec.Permissions.Configure = "?"
			_, err := newPermission.ValidateUpdate(rootCtx, &permission, newPermission)
			Expect(err).ToNot(HaveOccurred())
		})

		It("allows updates on permission.spec.permissions.read", func() {
			newPermission := permission.DeepCopy()
			newPermission.Spec.Permissions.Read = "?"
			_, err := newPermission.ValidateUpdate(rootCtx, &permission, newPermission)
			Expect(err).ToNot(HaveOccurred())
		})

		It("allows updates on permission.spec.permissions.write", func() {
			newPermission := permission.DeepCopy()
			newPermission.Spec.Permissions.Write = "?"
			_, err := newPermission.ValidateUpdate(rootCtx, &permission, newPermission)
			Expect(err).ToNot(HaveOccurred())
		})

		It("does not allow user and userReference to be specified at the same time", func() {
			newPermission := permission.DeepCopy()
			newPermission.Spec.UserReference = &corev1.LocalObjectReference{Name: "invalid"}
			_, err := newPermission.ValidateUpdate(rootCtx, &permission, newPermission)
			Expect(err).To(MatchError(ContainSubstring("cannot specify spec.user and spec.userReference at the same time")))
		})

		It("does not allow both user and userReference to be unset", func() {
			newPermission := permission.DeepCopy()
			newPermission.Spec.User = ""
			newPermission.Spec.UserReference = nil
			_, err := newPermission.ValidateUpdate(rootCtx, &permission, newPermission)
			Expect(err).To(MatchError(ContainSubstring("must specify either spec.user or spec.userReference")))
		})
	})
})
