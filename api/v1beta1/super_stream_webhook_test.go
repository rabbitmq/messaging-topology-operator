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
			Name:        "test",
			Partitions:  4,
			RoutingKeys: []string{"a1", "b2", "f17"},
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

	It("does not allow updates on superstream.spec.routingKeys", func() {
		newSuperStream := superstream.DeepCopy()
		newSuperStream.Spec.RoutingKeys = []string{"a1", "d6"}
		Expect(apierrors.IsForbidden(newSuperStream.ValidateUpdate(&superstream))).To(BeTrue())
	})

	It("if the superstream previously had no routing keys but now does, the update succeeds", func() {
		superstream.Spec.RoutingKeys = nil
		newSuperStream := superstream.DeepCopy()
		newSuperStream.Spec.RoutingKeys = []string{"a1", "b2", "f17"}
		Expect(newSuperStream.ValidateUpdate(&superstream)).To(Succeed())
	})

	It("allows superstream.spec.partitions to be increased", func() {
		newSuperStream := superstream.DeepCopy()
		newSuperStream.Spec.Partitions = 1000
		Expect(newSuperStream.ValidateUpdate(&superstream)).To(Succeed())
	})
	It("does not allow superstream.spec.partitions to be decreased", func() {
		newSuperStream := superstream.DeepCopy()
		newSuperStream.Spec.Partitions = 1
		Expect(apierrors.IsForbidden(newSuperStream.ValidateUpdate(&superstream))).To(BeTrue())
	})

})
