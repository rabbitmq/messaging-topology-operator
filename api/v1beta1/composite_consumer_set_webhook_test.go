package v1beta1

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("compositeConsumerSet webhook", func() {
	var compositeConsumerSet = CompositeConsumerSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test",
		},
		Spec: CompositeConsumerSetSpec{
			SuperStreamReference: SuperStreamReference{
				Name: "a-super-stream",
			},
		},
	}

	It("does not allow updates on SuperStreamReference", func() {
		newCompositeConsumerSet := compositeConsumerSet.DeepCopy()
		newCompositeConsumerSet.Spec.SuperStreamReference = SuperStreamReference{
			Name: "new-cluster",
		}
		Expect(apierrors.IsForbidden(newCompositeConsumerSet.ValidateUpdate(&compositeConsumerSet))).To(BeTrue())
	})
})
