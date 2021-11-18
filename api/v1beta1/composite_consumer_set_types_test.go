package v1beta1

import (
	"context"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("CompositeConsumerSet spec", func() {
	var (
		namespace = "default"
		ctx       = context.Background()
	)

	It("creates a composite consumer set with default settings", func() {
		expectedSpec := CompositeConsumerSetSpec{
			SuperStreamReference: SuperStreamReference{
				Name: "some-super-stream",
			},
		}

		compositeConsumerSet := CompositeConsumerSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-composite-consumer-set",
				Namespace: namespace,
			},
			Spec: CompositeConsumerSetSpec{
				SuperStreamReference: SuperStreamReference{
					Name: "some-super-stream",
				},
			},
		}
		Expect(k8sClient.Create(ctx, &compositeConsumerSet)).To(Succeed())
		fetchedCompositeConsumerSet := &CompositeConsumerSet{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{
			Name:      compositeConsumerSet.Name,
			Namespace: compositeConsumerSet.Namespace,
		}, fetchedCompositeConsumerSet)).To(Succeed())
		Expect(fetchedCompositeConsumerSet.Spec).To(Equal(expectedSpec))
	})
})
