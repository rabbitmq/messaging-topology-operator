package v1beta1

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("compositeConsumer webhook", func() {
	var compositeConsumer = CompositeConsumer{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test",
		},
		Spec: CompositeConsumerSpec{
			SuperStreamReference: SuperStreamReference{
				Name: "a-super-stream",
			},
		},
	}

	It("does not allow updates on SuperStreamReference", func() {
		newCompositeConsumer := compositeConsumer.DeepCopy()
		newCompositeConsumer.Spec.SuperStreamReference = SuperStreamReference{
			Name: "new-cluster",
		}
		Expect(apierrors.IsForbidden(newCompositeConsumer.ValidateUpdate(&compositeConsumer))).To(BeTrue())
	})
})
