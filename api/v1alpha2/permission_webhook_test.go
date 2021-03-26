package v1alpha2

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("permission webhook", func() {
	var permission = Permission{
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

	It("does not allow updates on user", func() {
		newPermission := permission.DeepCopy()
		newPermission.Spec.User = "new-user"
		Expect(apierrors.IsForbidden(newPermission.ValidateUpdate(&permission))).To(BeTrue())
	})

	It("does not allow updates on vhost", func() {
		newPermission := permission.DeepCopy()
		newPermission.Spec.Vhost = "new-vhost"
		Expect(apierrors.IsForbidden(newPermission.ValidateUpdate(&permission))).To(BeTrue())
	})

	It("does not allow updates on RabbitmqClusterReference", func() {
		newPermission := permission.DeepCopy()
		newPermission.Spec.RabbitmqClusterReference = RabbitmqClusterReference{
			Name: "new-cluster",
		}
		Expect(apierrors.IsForbidden(newPermission.ValidateUpdate(&permission))).To(BeTrue())
	})

	It("allows updates on permission.spec.permissions.configure", func() {
		newPermission := permission.DeepCopy()
		newPermission.Spec.Permissions.Configure = "?"
		Expect(newPermission.ValidateUpdate(&permission)).To(Succeed())
	})

	It("allows updates on permission.spec.permissions.read", func() {
		newPermission := permission.DeepCopy()
		newPermission.Spec.Permissions.Read = "?"
		Expect(newPermission.ValidateUpdate(&permission)).To(Succeed())
	})

	It("allows updates on permission.spec.permissions.write", func() {
		newPermission := permission.DeepCopy()
		newPermission.Spec.Permissions.Write = "?"
		Expect(newPermission.ValidateUpdate(&permission)).To(Succeed())
	})
})
