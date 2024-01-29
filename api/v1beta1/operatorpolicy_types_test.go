package v1beta1

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("OperatorPolicy", func() {
	var (
		namespace = "default"
		ctx       = context.Background()
	)

	It("creates an operator policy with minimal configurations", func() {
		policy := OperatorPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-operator-policy",
				Namespace: namespace,
			},
			Spec: OperatorPolicySpec{
				Name:    "test-operator-policy",
				Pattern: "^some-prefix",
				Definition: &runtime.RawExtension{
					Raw: []byte(`{"max-length": 10}`),
				},
				RabbitmqClusterReference: RabbitmqClusterReference{
					Name: "some-cluster",
				},
			},
		}
		Expect(k8sClient.Create(ctx, &policy)).To(Succeed())
		fetched := &OperatorPolicy{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{
			Name:      policy.Name,
			Namespace: policy.Namespace,
		}, fetched)).To(Succeed())
		Expect(fetched.Spec.RabbitmqClusterReference).To(Equal(RabbitmqClusterReference{
			Name: "some-cluster",
		}))
		Expect(fetched.Spec.Name).To(Equal("test-operator-policy"))
		Expect(fetched.Spec.Vhost).To(Equal("/"))
		Expect(fetched.Spec.Pattern).To(Equal("^some-prefix"))
		Expect(fetched.Spec.ApplyTo).To(Equal("queues"))
		Expect(fetched.Spec.Priority).To(Equal(0))
		Expect(fetched.Spec.Definition.Raw).To(Equal([]byte(`{"max-length":10}`)))
	})

	It("creates operator policy with configurations", func() {
		policy := OperatorPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "random-policy",
				Namespace: namespace,
			},
			Spec: OperatorPolicySpec{
				Name:     "test-policy",
				Vhost:    "/hello",
				Pattern:  "*.",
				ApplyTo:  "quorum_queues",
				Priority: 100,
				Definition: &runtime.RawExtension{
					Raw: []byte(`{"max-length":10}`),
				},
				RabbitmqClusterReference: RabbitmqClusterReference{
					Name: "random-cluster",
				},
			},
		}
		Expect(k8sClient.Create(ctx, &policy)).To(Succeed())
		fetched := &OperatorPolicy{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{
			Name:      policy.Name,
			Namespace: policy.Namespace,
		}, fetched)).To(Succeed())

		Expect(fetched.Spec.Name).To(Equal("test-policy"))
		Expect(fetched.Spec.Vhost).To(Equal("/hello"))
		Expect(fetched.Spec.Pattern).To(Equal("*."))
		Expect(fetched.Spec.ApplyTo).To(Equal("quorum_queues"))
		Expect(fetched.Spec.Priority).To(Equal(100))
		Expect(fetched.Spec.RabbitmqClusterReference).To(Equal(
			RabbitmqClusterReference{
				Name: "random-cluster",
			}))
		Expect(fetched.Spec.Definition.Raw).To(Equal([]byte(`{"max-length":10}`)))
	})

	When("creating a policy with an invalid 'ApplyTo' value", func() {
		It("fails with validation errors", func() {
			policy := OperatorPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "invalid",
					Namespace: namespace,
				},
				Spec: OperatorPolicySpec{
					Name:    "test-policy",
					Pattern: "a-queue-name",
					Definition: &runtime.RawExtension{
						Raw: []byte(`{"max-length":10}`),
					},
					ApplyTo: "yo-yo",
					RabbitmqClusterReference: RabbitmqClusterReference{
						Name: "some-cluster",
					},
				},
			}
			Expect(k8sClient.Create(ctx, &policy)).To(HaveOccurred())
			Expect(k8sClient.Create(ctx, &policy)).To(MatchError(`OperatorPolicy.rabbitmq.com "invalid" is invalid: spec.applyTo: Unsupported value: "yo-yo": supported values: "queues", "classic_queues", "quorum_queues", "streams"`))
		})
	})

})
