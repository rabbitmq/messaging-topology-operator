package v1beta1

import (
	"context"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("Exchange spec", func() {
	var (
		namespace = "default"
		ctx       = context.Background()
	)

	It("creates a exchange with default settings", func() {
		expectedSpec := ExchangeSpec{
			Name:       "test-exchange",
			Vhost:      "/",
			Durable:    false,
			AutoDelete: false,
			Type:       "direct",
			RabbitmqClusterReference: RabbitmqClusterReference{
				Name:      "some-cluster",
				Namespace: namespace,
			},
		}

		q := Exchange{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-exchange",
				Namespace: namespace,
			},
			Spec: ExchangeSpec{
				Name: "test-exchange",
				RabbitmqClusterReference: RabbitmqClusterReference{
					Name:      "some-cluster",
					Namespace: namespace,
				},
			},
		}
		Expect(k8sClient.Create(ctx, &q)).To(Succeed())
		fetchedExchange := &Exchange{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{
			Name:      q.Name,
			Namespace: q.Namespace,
		}, fetchedExchange)).To(Succeed())
		Expect(fetchedExchange.Spec).To(Equal(expectedSpec))
	})

	It("creates a exchange with configurations", func() {
		q := Exchange{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "random-q",
				Namespace: namespace,
			},
			Spec: ExchangeSpec{
				Name:       "test-exchange",
				Vhost:      "/hello",
				Type:       "fanout",
				Durable:    true,
				AutoDelete: true,
				Arguments: &runtime.RawExtension{
					Raw: []byte(`{"alternative-exchange":"alternative-name"}`),
				},
				RabbitmqClusterReference: RabbitmqClusterReference{
					Name:      "random-cluster",
					Namespace: namespace,
				},
			},
		}
		Expect(k8sClient.Create(ctx, &q)).To(Succeed())
		fetchedExchange := &Exchange{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{
			Name:      q.Name,
			Namespace: q.Namespace,
		}, fetchedExchange)).To(Succeed())

		Expect(fetchedExchange.Spec.Name).To(Equal("test-exchange"))
		Expect(fetchedExchange.Spec.Vhost).To(Equal("/hello"))
		Expect(fetchedExchange.Spec.Type).To(Equal("fanout"))
		Expect(fetchedExchange.Spec.Durable).To(BeTrue())
		Expect(fetchedExchange.Spec.AutoDelete).To(BeTrue())
		Expect(fetchedExchange.Spec.RabbitmqClusterReference).To(Equal(
			RabbitmqClusterReference{
				Name:      "random-cluster",
				Namespace: namespace,
			}))
		Expect(fetchedExchange.Spec.Arguments.Raw).To(Equal([]byte(`{"alternative-exchange":"alternative-name"}`)))
	})
})
