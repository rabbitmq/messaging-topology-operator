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

var _ = Describe("User Webhook", func() {
	var (
		obj       *rabbitmqcomv1beta1.User
		oldObj    *rabbitmqcomv1beta1.User
		validator UserCustomValidator
	)

	BeforeEach(func() {
		obj = &rabbitmqcomv1beta1.User{}
		oldObj = &rabbitmqcomv1beta1.User{}
		validator = UserCustomValidator{}
	})

	Context("structural validation (no k8s client needed)", func() {
		It("allows creation when only a cluster name is provided", func() {
			obj = &rabbitmqcomv1beta1.User{
				ObjectMeta: metav1.ObjectMeta{Name: "test-user", Namespace: "default"},
				Spec: rabbitmqcomv1beta1.UserSpec{
					RabbitmqClusterReference: rabbitmqcomv1beta1.RabbitmqClusterReference{
						Name: "my-cluster",
					},
				},
			}
			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).NotTo(HaveOccurred())
		})

		It("denies creation when both cluster name and connectionSecret are provided", func() {
			obj = &rabbitmqcomv1beta1.User{
				ObjectMeta: metav1.ObjectMeta{Name: "test-user", Namespace: "default"},
				Spec: rabbitmqcomv1beta1.UserSpec{
					RabbitmqClusterReference: rabbitmqcomv1beta1.RabbitmqClusterReference{
						Name:             "my-cluster",
						ConnectionSecret: &corev1.LocalObjectReference{Name: "my-conn-secret"},
					},
				},
			}
			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).To(HaveOccurred())
		})

		It("denies updates that change rabbitmqClusterReference", func() {
			oldObj = &rabbitmqcomv1beta1.User{
				ObjectMeta: metav1.ObjectMeta{Name: "test-user", Namespace: "default"},
				Spec: rabbitmqcomv1beta1.UserSpec{
					RabbitmqClusterReference: rabbitmqcomv1beta1.RabbitmqClusterReference{Name: "cluster-a"},
				},
			}
			obj = &rabbitmqcomv1beta1.User{
				ObjectMeta: metav1.ObjectMeta{Name: "test-user", Namespace: "default"},
				Spec: rabbitmqcomv1beta1.UserSpec{
					RabbitmqClusterReference: rabbitmqcomv1beta1.RabbitmqClusterReference{Name: "cluster-b"},
				},
			}
			_, err := validator.ValidateUpdate(ctx, oldObj, obj)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("rabbitmqClusterReference"))
		})
	})

	Context("importCredentialsSecret label enforcement", func() {
		const testNS = "default"

		buildUser := func(importSecretName string) *rabbitmqcomv1beta1.User {
			return &rabbitmqcomv1beta1.User{
				ObjectMeta: metav1.ObjectMeta{Name: "test-user", Namespace: testNS},
				Spec: rabbitmqcomv1beta1.UserSpec{
					RabbitmqClusterReference: rabbitmqcomv1beta1.RabbitmqClusterReference{
						Name: "my-cluster",
					},
					ImportCredentialsSecret: &corev1.LocalObjectReference{Name: importSecretName},
				},
			}
		}

		It("allows creation when importCredentialsSecret carries the topology operator label", func() {
			labeledSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "labeled-import-secret",
					Namespace: testNS,
					Labels: map[string]string{
						rabbitmqcomv1beta1.TopologyOperatorLabel: rabbitmqcomv1beta1.TopologyOperatorLabelValue,
					},
				},
			}
			// The cache client finds the labeled secret → validateSecretLabel returns nil.
			cacheClient := fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(labeledSecret).Build()
			v := UserCustomValidator{Client: cacheClient, APIReader: cacheClient}

			_, err := v.ValidateCreate(ctx, buildUser("labeled-import-secret"))
			Expect(err).NotTo(HaveOccurred())
		})

		It("denies creation when importCredentialsSecret exists without the topology operator label", func() {
			unlabeledSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: "unlabeled-import-secret", Namespace: testNS},
			}
			// The filtered cache has no entry (simulates the label selector filtering it out).
			// The apiReader finds the secret, confirming it exists but is unlabeled.
			emptyCache := fake.NewClientBuilder().WithScheme(scheme.Scheme).Build()
			apiReader := fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(unlabeledSecret).Build()
			v := UserCustomValidator{Client: emptyCache, APIReader: apiReader}

			_, err := v.ValidateCreate(ctx, buildUser("unlabeled-import-secret"))
			Expect(err).To(MatchError(ContainSubstring("must have label")))
		})

		It("denies creation when importCredentialsSecret does not exist", func() {
			emptyCache := fake.NewClientBuilder().WithScheme(scheme.Scheme).Build()
			v := UserCustomValidator{Client: emptyCache, APIReader: emptyCache}

			_, err := v.ValidateCreate(ctx, buildUser("nonexistent-secret"))
			Expect(err).To(MatchError(ContainSubstring("not found")))
		})

		It("denies update when the new importCredentialsSecret lacks the topology operator label", func() {
			unlabeledSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: "unlabeled-update-secret", Namespace: testNS},
			}
			emptyCache := fake.NewClientBuilder().WithScheme(scheme.Scheme).Build()
			apiReader := fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(unlabeledSecret).Build()
			v := UserCustomValidator{Client: emptyCache, APIReader: apiReader}

			oldObj = &rabbitmqcomv1beta1.User{
				ObjectMeta: metav1.ObjectMeta{Name: "test-user", Namespace: testNS},
				Spec: rabbitmqcomv1beta1.UserSpec{
					RabbitmqClusterReference: rabbitmqcomv1beta1.RabbitmqClusterReference{Name: "my-cluster"},
				},
			}
			obj = buildUser("unlabeled-update-secret")
			_, err := v.ValidateUpdate(ctx, oldObj, obj)
			Expect(err).To(MatchError(ContainSubstring("must have label")))
		})

		It("allows update when the User has a DeletionTimestamp even if the import secret is gone", func() {
			emptyCache := fake.NewClientBuilder().WithScheme(scheme.Scheme).Build()
			v := UserCustomValidator{Client: emptyCache, APIReader: emptyCache}

			now := metav1.Now()
			oldObj = &rabbitmqcomv1beta1.User{
				ObjectMeta: metav1.ObjectMeta{Name: "test-user", Namespace: testNS},
				Spec: rabbitmqcomv1beta1.UserSpec{
					RabbitmqClusterReference: rabbitmqcomv1beta1.RabbitmqClusterReference{Name: "my-cluster"},
				},
			}
			obj = &rabbitmqcomv1beta1.User{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "test-user",
					Namespace:         testNS,
					DeletionTimestamp: &now,
					Finalizers:        []string{"deletion.finalizers.users.rabbitmq.com"},
				},
				Spec: rabbitmqcomv1beta1.UserSpec{
					RabbitmqClusterReference: rabbitmqcomv1beta1.RabbitmqClusterReference{Name: "my-cluster"},
					ImportCredentialsSecret:  &corev1.LocalObjectReference{Name: "already-deleted-secret"},
				},
			}
			_, err := v.ValidateUpdate(ctx, oldObj, obj)
			Expect(err).NotTo(HaveOccurred())
		})

		It("allows update when the new importCredentialsSecret carries the topology operator label", func() {
			labeledSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "labeled-update-secret",
					Namespace: testNS,
					Labels: map[string]string{
						rabbitmqcomv1beta1.TopologyOperatorLabel: rabbitmqcomv1beta1.TopologyOperatorLabelValue,
					},
				},
			}
			cacheClient := fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(labeledSecret).Build()
			v := UserCustomValidator{Client: cacheClient, APIReader: cacheClient}

			oldObj = &rabbitmqcomv1beta1.User{
				ObjectMeta: metav1.ObjectMeta{Name: "test-user", Namespace: testNS},
				Spec: rabbitmqcomv1beta1.UserSpec{
					RabbitmqClusterReference: rabbitmqcomv1beta1.RabbitmqClusterReference{Name: "my-cluster"},
				},
			}
			obj = buildUser("labeled-update-secret")
			_, err := v.ValidateUpdate(ctx, oldObj, obj)
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
