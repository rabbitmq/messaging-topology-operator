package v1beta1

import (
	"context"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("CompositeConsumer spec", func() {
	var (
		namespace = "default"
		ctx       = context.Background()
	)

	It("creates a composite consumer with default settings", func() {
		compositeConsumer := CompositeConsumer{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-composite-consumer",
				Namespace: namespace,
			},
			Spec: CompositeConsumerSpec{
				ConsumerPodSpec: CompositeConsumerPodSpec{},
				SuperStreamReference: SuperStreamReference{
					Name: "some-super-stream",
				},
			},
		}
		Expect(k8sClient.Create(ctx, &compositeConsumer)).To(Succeed())
		fetchedCompositeConsumer := &CompositeConsumer{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{
			Name:      compositeConsumer.Name,
			Namespace: compositeConsumer.Namespace,
		}, fetchedCompositeConsumer)).To(Succeed())
		Expect(fetchedCompositeConsumer.Spec).To(Equal(compositeConsumer.Spec))
	})
})
