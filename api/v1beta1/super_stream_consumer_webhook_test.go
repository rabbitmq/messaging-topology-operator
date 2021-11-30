package v1beta1

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("superStreamConsumer webhook", func() {
	var superStreamConsumer = SuperStreamConsumer{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test",
		},
		Spec: SuperStreamConsumerSpec{
			SuperStreamReference: SuperStreamReference{
				Name: "a-super-stream",
			},
		},
	}

	It("does not allow updates on SuperStreamReference", func() {
		newSuperStreamConsumer := superStreamConsumer.DeepCopy()
		newSuperStreamConsumer.Spec.SuperStreamReference = SuperStreamReference{
			Name: "new-cluster",
		}
		Expect(apierrors.IsForbidden(newSuperStreamConsumer.ValidateUpdate(&superStreamConsumer))).To(BeTrue())
	})
})
