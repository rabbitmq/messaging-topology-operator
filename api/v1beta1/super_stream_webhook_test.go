package v1beta1

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("superstream webhook", func() {
	var superstream = SuperStream{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test",
		},
		Spec: SuperStreamSpec{
			Name:       "test",
			Partitions: 4,
			RabbitmqClusterReference: RabbitmqClusterReference{
				Name: "a-cluster",
			},
		},
	}

	It("does not allow updates on superstream name", func() {
		newSuperStream := superstream.DeepCopy()
		newSuperStream.Spec.Name = "new-name"
		Expect(apierrors.IsForbidden(newSuperStream.ValidateUpdate(&superstream))).To(BeTrue())
	})

	It("does not allow updates on RabbitmqClusterReference", func() {
		newSuperStream := superstream.DeepCopy()
		newSuperStream.Spec.RabbitmqClusterReference = RabbitmqClusterReference{
			Name: "new-cluster",
		}
		Expect(apierrors.IsForbidden(newSuperStream.ValidateUpdate(&superstream))).To(BeTrue())
	})

	It("allows updates on superstream.spec.partitions", func() {
		newSuperStream := superstream.DeepCopy()
		newSuperStream.Spec.Partitions = 1000
		Expect(newSuperStream.ValidateUpdate(&superstream)).To(Succeed())
	})

})
