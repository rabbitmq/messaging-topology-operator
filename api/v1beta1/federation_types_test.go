package v1beta1

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("Federation spec", func() {
	var (
		namespace = "default"
		ctx       = context.Background()
	)

	It("creates a federation with minimal configurations", func() {
		expectedSpec := FederationSpec{
			Name:  "test-federation",
			Vhost: "/",
			RabbitmqClusterReference: RabbitmqClusterReference{
				Name: "some-cluster",
			},
			UriSecret: &corev1.LocalObjectReference{
				Name: "a-secret",
			},
			ReconnectDelay: new(1),
			DeletionPolicy: "delete",
		}

		federation := Federation{
			Name:      "test-federation",
			Namespace: namespace,
			Spec: FederationSpec{
				Name: "test-federation",
				UriSecret: &corev1.LocalObjectReference{
					Name: "a-secret",
				},
				RabbitmqClusterReference: RabbitmqClusterReference{
					Name: "some-cluster",
				},
			},
		}
		Expect(k8sClient.Create(ctx, &federation)).To(Succeed())
		fetched := &Federation{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{
			Name:      federation.Name,
			Namespace: federation.Namespace,
		}, fetched)).To(Succeed())
		Expect(fetched.Spec).To(Equal(expectedSpec))
	})

	It("creates a federation with configurations", func() {
		federation := Federation{
			Name:      "configured-federation",
			Namespace: namespace,
			Spec: FederationSpec{
				Name:  "configured-federation",
				Vhost: "/hello",
				UriSecret: &corev1.LocalObjectReference{
					Name: "a-secret",
				},
				Expires:             1000,
				MessageTTL:          1000,
				MaxHops:             100,
				PrefetchCount:       50,
				ReconnectDelay:      new(10),
				TrustUserId:         true,
				Exchange:            "an-exchange",
				AckMode:             "no-ack",
				QueueType:           "quorum",
				ResourceCleanupMode: "never",
				RabbitmqClusterReference: RabbitmqClusterReference{
					Name: "some-cluster",
				},
			},
		}
		Expect(k8sClient.Create(ctx, &federation)).To(Succeed())
		fetched := &Federation{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{
			Name:      federation.Name,
			Namespace: federation.Namespace,
		}, fetched)).To(Succeed())

		Expect(fetched.Spec.Name).To(Equal("configured-federation"))
		Expect(fetched.Spec.Vhost).To(Equal("/hello"))
		Expect(fetched.Spec.RabbitmqClusterReference).To(Equal(
			RabbitmqClusterReference{
				Name: "some-cluster",
			}))

		Expect(fetched.Spec.UriSecret.Name).To(Equal("a-secret"))
		Expect(fetched.Spec.AckMode).To(Equal("no-ack"))
		Expect(fetched.Spec.Exchange).To(Equal("an-exchange"))
		Expect(fetched.Spec.Expires).To(Equal(1000))
		Expect(fetched.Spec.MessageTTL).To(Equal(1000))
		Expect(fetched.Spec.MaxHops).To(Equal(100))
		Expect(fetched.Spec.PrefetchCount).To(Equal(50))
		Expect(fetched.Spec.ReconnectDelay).To(HaveValue(Equal(10)))
		Expect(fetched.Spec.QueueType).To(Equal("quorum"))
		Expect(fetched.Spec.ResourceCleanupMode).To(Equal("never"))
	})

	When("creating a federation with an invalid 'AckMode' value", func() {
		It("fails with validation errors", func() {
			federation := Federation{
				Name:      "invalid-federation",
				Namespace: namespace,
				Spec: FederationSpec{
					Name: "test-federation",
					UriSecret: &corev1.LocalObjectReference{
						Name: "a-secret",
					},
					AckMode: "non-existing-ackmode",
					RabbitmqClusterReference: RabbitmqClusterReference{
						Name: "some-cluster",
					},
				},
			}
			Expect(k8sClient.Create(ctx, &federation)).To(HaveOccurred())
			Expect(k8sClient.Create(ctx, &federation)).To(MatchError(`Federation.rabbitmq.com "invalid-federation" is invalid: spec.ackMode: Unsupported value: "non-existing-ackmode": supported values: "on-confirm", "on-publish", "no-ack"`))
		})
	})

	Describe("reconnectDelay", func() {
		When("it is not set", func() {
			It("defaults to 1", func() {
				federation := Federation{
					Name:      "federation-without-reconnect-delay",
					Namespace: namespace,
					Spec: FederationSpec{
						Name: "federation-without-reconnect-delay",
						UriSecret: &corev1.LocalObjectReference{
							Name: "a-secret",
						},
						RabbitmqClusterReference: RabbitmqClusterReference{
							Name: "some-cluster",
						},
					},
				}
				Expect(k8sClient.Create(ctx, &federation)).To(Succeed())
				fetched := &Federation{}
				Expect(k8sClient.Get(ctx, types.NamespacedName{
					Name:      federation.Name,
					Namespace: federation.Namespace,
				}, fetched)).To(Succeed())

				Expect(fetched.Spec.ReconnectDelay).To(HaveValue(Equal(1)))
			})
		})

		When("it is explicitly set to 0 by a typed client", func() {
			It("is preserved rather than defaulted", func() {
				federation := Federation{
					Name:      "federation-with-zero-reconnect-delay",
					Namespace: namespace,
					Spec: FederationSpec{
						Name: "federation-with-zero-reconnect-delay",
						UriSecret: &corev1.LocalObjectReference{
							Name: "a-secret",
						},
						ReconnectDelay: new(0),
						RabbitmqClusterReference: RabbitmqClusterReference{
							Name: "some-cluster",
						},
					},
				}

				Expect(k8sClient.Create(ctx, &federation)).To(Succeed())
				fetched := &Federation{}
				Expect(k8sClient.Get(ctx, types.NamespacedName{
					Name:      federation.Name,
					Namespace: federation.Namespace,
				}, fetched)).To(Succeed())

				Expect(fetched.Spec.ReconnectDelay).To(HaveValue(BeZero()))
			})
		})

		When("it is explicitly set to 0 and the operator then adds its finalizer", func() {
			It("is still preserved", func() {
				federation := &unstructured.Unstructured{
					Object: map[string]any{
						"spec": map[string]any{
							"name":           "federation-zero-then-finalized",
							"reconnectDelay": int64(0),
							"uriSecret":      map[string]any{"name": "a-secret"},
							"rabbitmqClusterReference": map[string]any{
								"name": "some-cluster",
							},
						},
					},
				}
				federation.SetGroupVersionKind(GroupVersion.WithKind("Federation"))
				federation.SetName("federation-zero-then-finalized")
				federation.SetNamespace(namespace)
				Expect(k8sClient.Create(ctx, federation)).To(Succeed())

				fetched := &Federation{}
				Expect(k8sClient.Get(ctx, types.NamespacedName{
					Name:      federation.GetName(),
					Namespace: federation.GetNamespace(),
				}, fetched)).To(Succeed())
				fetched.SetFinalizers([]string{"deletion.finalizers.federations.rabbitmq.com"})
				Expect(k8sClient.Update(ctx, fetched)).To(Succeed())

				refetched := &Federation{}
				Expect(k8sClient.Get(ctx, types.NamespacedName{
					Name:      federation.GetName(),
					Namespace: federation.GetNamespace(),
				}, refetched)).To(Succeed())

				Expect(refetched.Spec.ReconnectDelay).To(HaveValue(BeZero()))
			})
		})
	})

	It("creates a federation with non-default DeletionPolicy", func() {
		federation := Federation{
			Name:      "federation-with-retain-policy",
			Namespace: namespace,
			Spec: FederationSpec{
				Name:           "federation-with-retain-policy",
				DeletionPolicy: "retain",
				UriSecret: &corev1.LocalObjectReference{
					Name: "a-secret",
				},
				RabbitmqClusterReference: RabbitmqClusterReference{
					Name: "some-cluster",
				},
			},
		}
		Expect(k8sClient.Create(ctx, &federation)).To(Succeed())
		fetched := &Federation{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{
			Name:      federation.Name,
			Namespace: federation.Namespace,
		}, fetched)).To(Succeed())

		Expect(fetched.Spec.DeletionPolicy).To(Equal("retain"))
		Expect(fetched.Spec.Name).To(Equal("federation-with-retain-policy"))
	})
})
