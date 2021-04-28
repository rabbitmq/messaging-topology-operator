package v1beta1

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("user webhook", func() {
	var user = User{
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

	It("does not allow updates on RabbitmqClusterReference", func() {
		newUser := user.DeepCopy()
		newUser.Spec.RabbitmqClusterReference = RabbitmqClusterReference{
			Name: "newUser-cluster",
		}
		Expect(apierrors.IsForbidden(newUser.ValidateUpdate(&user))).To(BeTrue())
	})

	It("allows update on tags", func() {
		newUser := user.DeepCopy()
		newUser.Spec.Tags = []UserTag{"monitoring"}
		Expect(newUser.ValidateUpdate(&user)).To(Succeed())
	})
})
