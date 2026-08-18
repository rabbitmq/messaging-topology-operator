/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1beta1

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	rabbitmqcomv1beta1 "github.com/rabbitmq/messaging-topology-operator/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("Vhost Webhook", func() {
	var (
		obj       *rabbitmqcomv1beta1.Vhost
		oldObj    *rabbitmqcomv1beta1.Vhost
		validator VhostCustomValidator
	)

	BeforeEach(func() {
		obj = &rabbitmqcomv1beta1.Vhost{}
		oldObj = &rabbitmqcomv1beta1.Vhost{}
		validator = VhostCustomValidator{}
	})

	Context("structural validation (no k8s client needed)", func() {
		It("allows creation when only a cluster name is provided", func() {
			obj = &rabbitmqcomv1beta1.Vhost{
				ObjectMeta: metav1.ObjectMeta{Name: "test-vhost", Namespace: "default"},
				Spec: rabbitmqcomv1beta1.VhostSpec{
					Name: "test-vhost",
					RabbitmqClusterReference: rabbitmqcomv1beta1.RabbitmqClusterReference{
						Name: "my-cluster",
					},
				},
			}
			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).NotTo(HaveOccurred())
		})

		It("denies creation when both cluster name and connectionSecret are provided", func() {
			obj = &rabbitmqcomv1beta1.Vhost{
				ObjectMeta: metav1.ObjectMeta{Name: "test-vhost", Namespace: "default"},
				Spec: rabbitmqcomv1beta1.VhostSpec{
					Name: "test-vhost",
					RabbitmqClusterReference: rabbitmqcomv1beta1.RabbitmqClusterReference{
						Name:             "my-cluster",
						ConnectionSecret: &corev1.LocalObjectReference{Name: "conn-secret"},
					},
				},
			}
			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).To(HaveOccurred())
		})

		It("denies updates to rabbitmqClusterReference", func() {
			oldObj = &rabbitmqcomv1beta1.Vhost{
				ObjectMeta: metav1.ObjectMeta{Name: "test-vhost", Namespace: "default"},
				Spec: rabbitmqcomv1beta1.VhostSpec{
					Name: "test-vhost",
					RabbitmqClusterReference: rabbitmqcomv1beta1.RabbitmqClusterReference{
						Name: "my-cluster",
					},
				},
			}
			obj = oldObj.DeepCopy()
			obj.Spec.RabbitmqClusterReference.Name = "other-cluster"

			_, err := validator.ValidateUpdate(ctx, oldObj, obj)
			Expect(err).To(HaveOccurred())
		})
	})

	Context("connectionSecret label enforcement respects rabbitmqClusterReference.namespace", func() {
		const (
			resourceNS   = "default"
			referencedNS = "other-ns"
		)

		buildVhostWithConnectionSecret := func(refNamespace, secretName string) *rabbitmqcomv1beta1.Vhost {
			return &rabbitmqcomv1beta1.Vhost{
				ObjectMeta: metav1.ObjectMeta{Name: "test-vhost", Namespace: resourceNS},
				Spec: rabbitmqcomv1beta1.VhostSpec{
					Name: "test-vhost",
					RabbitmqClusterReference: rabbitmqcomv1beta1.RabbitmqClusterReference{
						Namespace:        refNamespace,
						ConnectionSecret: &corev1.LocalObjectReference{Name: secretName},
					},
				},
			}
		}

		When("rabbitmqClusterReference.namespace is set", func() {
			When("the connectionSecret exists in that namespace", func() {
				It("allows creation", func() {
					labeledSecret := &corev1.Secret{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "conn-secret",
							Namespace: referencedNS,
							Labels: map[string]string{
								rabbitmqcomv1beta1.TopologyOperatorLabel: rabbitmqcomv1beta1.TopologyOperatorLabelValue,
							},
						},
					}
					cacheClient := fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(labeledSecret).Build()
					v := VhostCustomValidator{Client: cacheClient, APIReader: cacheClient}

					_, err := v.ValidateCreate(ctx, buildVhostWithConnectionSecret(referencedNS, "conn-secret"))
					Expect(err).NotTo(HaveOccurred())
				})
			})

			When("the connectionSecret only exists in the resource's own namespace", func() {
				It("denies creation", func() {
					secretInWrongNamespace := &corev1.Secret{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "conn-secret",
							Namespace: resourceNS,
							Labels: map[string]string{
								rabbitmqcomv1beta1.TopologyOperatorLabel: rabbitmqcomv1beta1.TopologyOperatorLabelValue,
							},
						},
					}
					cacheClient := fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(secretInWrongNamespace).Build()
					v := VhostCustomValidator{Client: cacheClient, APIReader: cacheClient}

					_, err := v.ValidateCreate(ctx, buildVhostWithConnectionSecret(referencedNS, "conn-secret"))
					Expect(err).To(MatchError(ContainSubstring("not found")))
				})
			})
		})

		When("rabbitmqClusterReference.namespace is unset", func() {
			It("falls back to looking up the connectionSecret in the resource's own namespace", func() {
				labeledSecret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "conn-secret",
						Namespace: resourceNS,
						Labels: map[string]string{
							rabbitmqcomv1beta1.TopologyOperatorLabel: rabbitmqcomv1beta1.TopologyOperatorLabelValue,
						},
					},
				}
				cacheClient := fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(labeledSecret).Build()
				v := VhostCustomValidator{Client: cacheClient, APIReader: cacheClient}

				_, err := v.ValidateCreate(ctx, buildVhostWithConnectionSecret("", "conn-secret"))
				Expect(err).NotTo(HaveOccurred())
			})
		})
	})
})
