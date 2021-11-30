package v1beta1

import (
	"context"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("SuperStreamConsumer spec", func() {
	var (
		namespace = "default"
		ctx       = context.Background()
	)

	It("creates a superstreamconsumer with default settings", func() {
		superStreamConsumer := SuperStreamConsumer{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-super-stream-consumer",
				Namespace: namespace,
			},
			Spec: SuperStreamConsumerSpec{
				ConsumerPodSpec: SuperStreamConsumerPodSpec{},
				SuperStreamReference: SuperStreamReference{
					Name: "some-super-stream",
				},
			},
		}
		Expect(k8sClient.Create(ctx, &superStreamConsumer)).To(Succeed())
		fetchedSuperStreamConsumer := &SuperStreamConsumer{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{
			Name:      superStreamConsumer.Name,
			Namespace: superStreamConsumer.Namespace,
		}, fetchedSuperStreamConsumer)).To(Succeed())
		Expect(fetchedSuperStreamConsumer.Spec).To(Equal(superStreamConsumer.Spec))
	})
})
