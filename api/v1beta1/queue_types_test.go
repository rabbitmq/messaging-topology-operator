package v1beta1

import (
	"context"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("Queue spec", func() {
	var (
		namespace = "default"
		ctx       = context.Background()
	)

	It("creates a queue with default settings", func() {
		expectedSpec := QueueSpec{
			Vhost:      "/",
			Durable:    false,
			AutoDelete: false,
			RabbitmqClusterReference: RabbitmqClusterReference{
				Name:      "some-cluster",
				Namespace: namespace,
			},
		}

		q := Queue{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "some-q",
				Namespace: namespace,
			},
			Spec: QueueSpec{
				RabbitmqClusterReference: RabbitmqClusterReference{
					Name:      "some-cluster",
					Namespace: namespace,
				},
			},
		}
		Expect(k8sClient.Create(ctx, &q)).To(Succeed())
		fetchedQ := &Queue{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{
			Name:      q.Name,
			Namespace: q.Namespace,
		}, fetchedQ)).To(Succeed())
		Expect(fetchedQ.Spec).To(Equal(expectedSpec))
	})

	It("creates a queue with configurations", func() {
		q := Queue{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "random-q",
				Namespace: namespace,
			},
			Spec: QueueSpec{
				Vhost:      "/hello",
				Type:       "a type",
				Durable:    true,
				AutoDelete: true,
				Arguments: &runtime.RawExtension{
					Raw: []byte(`{"yoyo":10}`),
				},
				RabbitmqClusterReference: RabbitmqClusterReference{
					Name:      "random-cluster",
					Namespace: namespace,
				},
			},
		}
		Expect(k8sClient.Create(ctx, &q)).To(Succeed())
		fetchedQ := &Queue{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{
			Name:      q.Name,
			Namespace: q.Namespace,
		}, fetchedQ)).To(Succeed())

		Expect(fetchedQ.Spec.Vhost).To(Equal("/hello"))
		Expect(fetchedQ.Spec.Type).To(Equal("a type"))
		Expect(fetchedQ.Spec.Durable).To(BeTrue())
		Expect(fetchedQ.Spec.AutoDelete).To(BeTrue())
		Expect(fetchedQ.Spec.RabbitmqClusterReference).To(Equal(
			RabbitmqClusterReference{
				Name:      "random-cluster",
				Namespace: namespace,
			}))
		Expect(fetchedQ.Spec.Arguments.Raw).To(Equal([]byte(`{"yoyo":10}`)))
	})
})
