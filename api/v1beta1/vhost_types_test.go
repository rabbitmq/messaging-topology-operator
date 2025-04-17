package v1beta1

import (
	"context"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("Vhost", func() {
	var (
		namespace = "default"
		ctx       = context.Background()
	)

	It("creates a vhost", func() {
		expectedSpec := VhostSpec{
			Name:           "test-vhost",
			Tracing:        false,
			DeletionPolicy: "delete",
			RabbitmqClusterReference: RabbitmqClusterReference{
				Name: "some-cluster",
			},
		}

		vhost := Vhost{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-vhost",
				Namespace: namespace,
			},
			Spec: VhostSpec{
				Name: "test-vhost",
				RabbitmqClusterReference: RabbitmqClusterReference{
					Name: "some-cluster",
				},
			},
		}
		Expect(k8sClient.Create(ctx, &vhost)).To(Succeed())
		fetched := &Vhost{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{
			Name:      vhost.Name,
			Namespace: vhost.Namespace,
		}, fetched)).To(Succeed())
		Expect(fetched.Spec).To(Equal(expectedSpec))
	})

	It("creates a vhost with 'tracing' configured", func() {
		vhost := Vhost{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "random-vhost",
				Namespace: namespace,
			},
			Spec: VhostSpec{
				Name:    "vhost-with-tracing",
				Tracing: true,
				RabbitmqClusterReference: RabbitmqClusterReference{
					Name: "random-cluster",
				},
			},
		}
		Expect(k8sClient.Create(ctx, &vhost)).To(Succeed())
		fetched := &Vhost{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{
			Name:      vhost.Name,
			Namespace: vhost.Namespace,
		}, fetched)).To(Succeed())

		Expect(fetched.Spec.Tracing).To(BeTrue())
		Expect(fetched.Spec.Name).To(Equal("vhost-with-tracing"))
		Expect(fetched.Spec.RabbitmqClusterReference).To(Equal(RabbitmqClusterReference{
			Name: "random-cluster",
		}))
	})

	It("creates a vhost with list of vhost tags configured", func() {
		vhost := Vhost{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "vhost-with-tags",
				Namespace: namespace,
			},
			Spec: VhostSpec{
				Name: "vhost-with-tags",
				Tags: []string{"tag1", "tag2", "multi_dc_replication"},
				RabbitmqClusterReference: RabbitmqClusterReference{
					Name: "random-cluster",
				},
			},
		}
		Expect(k8sClient.Create(ctx, &vhost)).To(Succeed())
		fetched := &Vhost{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{
			Name:      vhost.Name,
			Namespace: vhost.Namespace,
		}, fetched)).To(Succeed())

		Expect(fetched.Spec.Tags).To(ConsistOf("tag1", "tag2", "multi_dc_replication"))
		Expect(fetched.Spec.Name).To(Equal("vhost-with-tags"))
		Expect(fetched.Spec.RabbitmqClusterReference).To(Equal(RabbitmqClusterReference{
			Name: "random-cluster",
		}))
	})

	Context("Vhost Limits", func() {
		var connections, queues int32

		When("vhost limits are configured", func() {
			It("creates a vhost with limits", func() {
				connections = 1000
				queues = 500
				vhost := Vhost{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "vhost-with-limits",
						Namespace: namespace,
					},
					Spec: VhostSpec{
						Name: "vhost-with-limits",
						VhostLimits: &VhostLimits{
							Connections: &connections,
							Queues:      &queues,
						},
						RabbitmqClusterReference: RabbitmqClusterReference{
							Name: "random-cluster",
						},
					},
				}
				Expect(k8sClient.Create(ctx, &vhost)).To(Succeed())
				fetched := &Vhost{}
				Expect(k8sClient.Get(ctx, types.NamespacedName{
					Name:      vhost.Name,
					Namespace: vhost.Namespace,
				}, fetched)).To(Succeed())

				Expect(*fetched.Spec.VhostLimits.Connections).To(Equal(connections))
				Expect(*fetched.Spec.VhostLimits.Queues).To(Equal(queues))
				Expect(fetched.Spec.Name).To(Equal("vhost-with-limits"))
				Expect(fetched.Spec.RabbitmqClusterReference).To(Equal(RabbitmqClusterReference{
					Name: "random-cluster",
				}))
			})
		})

		When("No vhost limits are provided", func() {
			It("Does not set VhostLimits", func() {
				vhost := Vhost{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "vhost-with-no-provided-limits",
						Namespace: namespace,
					},
					Spec: VhostSpec{
						Name: "vhost-with-no-provided-limits",
						RabbitmqClusterReference: RabbitmqClusterReference{
							Name: "random-cluster",
						},
					},
				}
				Expect(k8sClient.Create(ctx, &vhost)).To(Succeed())
				fetched := &Vhost{}
				Expect(k8sClient.Get(ctx, types.NamespacedName{
					Name:      vhost.Name,
					Namespace: vhost.Namespace,
				}, fetched)).To(Succeed())

				Expect(fetched.Spec.VhostLimits).To(BeNil())
				Expect(fetched.Spec.Name).To(Equal("vhost-with-no-provided-limits"))
				Expect(fetched.Spec.RabbitmqClusterReference).To(Equal(RabbitmqClusterReference{
					Name: "random-cluster",
				}))
			})
		})

		When("Only some vhost limits are provided", func() {
			It("Configures those limits and lifts other limits", func() {
				queues = 800
				vhost := Vhost{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "vhost-some-limits",
						Namespace: namespace,
					},
					Spec: VhostSpec{
						Name: "vhost-some-limits",
						VhostLimits: &VhostLimits{
							Queues: &queues,
						},
						RabbitmqClusterReference: RabbitmqClusterReference{
							Name: "random-cluster",
						},
					},
				}
				Expect(k8sClient.Create(ctx, &vhost)).To(Succeed())
				fetched := &Vhost{}
				Expect(k8sClient.Get(ctx, types.NamespacedName{
					Name:      vhost.Name,
					Namespace: vhost.Namespace,
				}, fetched)).To(Succeed())

				Expect(fetched.Spec.VhostLimits.Connections).To(BeNil())
				Expect(*fetched.Spec.VhostLimits.Queues).To(Equal(queues))
				Expect(fetched.Spec.Name).To(Equal("vhost-some-limits"))
				Expect(fetched.Spec.RabbitmqClusterReference).To(Equal(RabbitmqClusterReference{
					Name: "random-cluster",
				}))
			})
		})
	})

	Context("Default queue types", func() {
		var qTypeVhost = &Vhost{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "some-vhost",
				Namespace: namespace,
			},
			Spec: VhostSpec{
				Name: "some-vhost",
				RabbitmqClusterReference: RabbitmqClusterReference{
					Name: "random-cluster",
				},
			},
		}
		It("creates a vhost with default queue type configured", func() {
			qTypeVhost.Spec.DefaultQueueType = "stream"
			Expect(k8sClient.Create(ctx, qTypeVhost)).To(Succeed())

			fetched := &Vhost{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      qTypeVhost.Name,
				Namespace: qTypeVhost.Namespace,
			}, fetched)).To(Succeed())
			Expect(fetched.Spec.DefaultQueueType).To(Equal("stream"))
			Expect(fetched.Spec.RabbitmqClusterReference).To(Equal(RabbitmqClusterReference{
				Name: "random-cluster",
			}))
		})

		It("fails when default queue type is invalid", func() {
			qTypeVhost.Spec.DefaultQueueType = "aqueuetype"
			Expect(k8sClient.Create(ctx, qTypeVhost)).To(HaveOccurred())
			Expect(k8sClient.Create(ctx, qTypeVhost)).To(MatchError(`Vhost.rabbitmq.com "some-vhost" is invalid: spec.defaultQueueType: Unsupported value: "aqueuetype": supported values: "quorum", "classic", "stream"`))
		})
	})

	It("creates a vhost with non-default DeletionPolicy", func() {
		vhost := Vhost{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "vhost-with-retain-policy",
				Namespace: namespace,
			},
			Spec: VhostSpec{
				Name:           "vhost-with-retain-policy",
				DeletionPolicy: "retain",
				RabbitmqClusterReference: RabbitmqClusterReference{
					Name: "random-cluster",
				},
			},
		}
		Expect(k8sClient.Create(ctx, &vhost)).To(Succeed())
		fetched := &Vhost{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{
			Name:      vhost.Name,
			Namespace: vhost.Namespace,
		}, fetched)).To(Succeed())

		Expect(fetched.Spec.DeletionPolicy).To(Equal("retain"))
		Expect(fetched.Spec.Name).To(Equal("vhost-with-retain-policy"))
	})
})
