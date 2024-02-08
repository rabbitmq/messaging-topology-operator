package v1beta1

import (
	"context"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("schema-replication webhook", func() {
	var (
		replication = SchemaReplication{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-replication",
			},
			Spec: SchemaReplicationSpec{
				UpstreamSecret: &corev1.LocalObjectReference{
					Name: "a-secret",
				},
				Endpoints: "abc.rmq.com:1234",
				RabbitmqClusterReference: RabbitmqClusterReference{
					Name: "a-cluster",
				},
			},
		}
		rootCtx = context.Background()
	)

	Context("ValidateCreate", func() {
		It("does not allow both spec.rabbitmqClusterReference.name and spec.rabbitmqClusterReference.connectionSecret be configured", func() {
			notAllowed := replication.DeepCopy()
			notAllowed.Spec.RabbitmqClusterReference.ConnectionSecret = &corev1.LocalObjectReference{Name: "some-secret"}
			_, err := notAllowed.ValidateCreate(rootCtx, notAllowed)
			Expect(err).To(MatchError(ContainSubstring("invalid RabbitmqClusterReference: do not provide both name and connectionSecret")))
		})

		It("spec.rabbitmqClusterReference.name and spec.rabbitmqClusterReference.connectionSecret cannot both be empty", func() {
			notAllowed := replication.DeepCopy()
			notAllowed.Spec.RabbitmqClusterReference.Name = ""
			notAllowed.Spec.RabbitmqClusterReference.ConnectionSecret = nil
			_, err := notAllowed.ValidateCreate(rootCtx, notAllowed)
			Expect(err).To(MatchError(ContainSubstring("invalid RabbitmqClusterReference: must provide either name or connectionSecret")))
		})

		It("does not allow both spec.upstreamSecret and spec.secretBackend.vault.userPath be configured", func() {
			notAllowed := replication.DeepCopy()
			notAllowed.Spec.SecretBackend = SecretBackend{
				Vault: &VaultSpec{
					SecretPath: "not-good",
				},
			}
			_, err := notAllowed.ValidateCreate(rootCtx, notAllowed)
			Expect(err).To(MatchError(ContainSubstring("do not provide both secretBackend.vault.secretPath and upstreamSecret")))
		})

		It("validates that either upstream secret or vault backend are configured", func() {
			notAllowed := replication.DeepCopy()
			notAllowed.Spec.SecretBackend = SecretBackend{
				Vault: &VaultSpec{},
			}
			notAllowed.Spec.UpstreamSecret.Name = ""
			_, err := notAllowed.ValidateCreate(rootCtx, notAllowed)
			Expect(err).To(MatchError(ContainSubstring("must provide either secretBackend.vault.secretPath or upstreamSecret")))
		})
	})

	Context("ValidateUpdate", func() {
		It("does not allow updates on RabbitmqClusterReference", func() {
			updated := replication.DeepCopy()
			updated.Spec.RabbitmqClusterReference = RabbitmqClusterReference{
				Name: "different-cluster",
			}
			_, err := updated.ValidateUpdate(rootCtx, &replication, updated)
			Expect(err).To(MatchError(ContainSubstring("update on rabbitmqClusterReference is forbidden")))
		})

		It("does not allow both spec.upstreamSecret and spec.secretBackend.vault.userPath be configured", func() {
			updated := replication.DeepCopy()
			updated.Spec.SecretBackend = SecretBackend{
				Vault: &VaultSpec{
					SecretPath: "not-good",
				},
			}
			_, err := updated.ValidateUpdate(rootCtx, &replication, updated)
			Expect(err).To(MatchError(ContainSubstring("do not provide both secretBackend.vault.secretPath and upstreamSecret")))
		})

		It("spec.upstreamSecret and spec.secretBackend.vault.userPath cannot both be not configured", func() {
			updated := replication.DeepCopy()
			updated.Spec.SecretBackend = SecretBackend{
				Vault: &VaultSpec{},
			}
			updated.Spec.UpstreamSecret.Name = ""
			_, err := updated.ValidateUpdate(rootCtx, &replication, updated)
			Expect(err).To(MatchError(ContainSubstring("must provide either secretBackend.vault.secretPath or upstreamSecret")))
		})

		It("allows update on spec.secretBackend.vault.userPath", func() {
			updated := replication.DeepCopy()
			updated.Spec.SecretBackend = SecretBackend{
				Vault: &VaultSpec{
					SecretPath: "a-new-path",
				},
			}
			updated.Spec.UpstreamSecret.Name = ""
			_, err := updated.ValidateUpdate(rootCtx, &replication, updated)
			Expect(err).ToNot(HaveOccurred())
		})

		It("does not allow updates on rabbitmqClusterReference.connectionSecret", func() {
			connectionScr := SchemaReplication{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-replication",
				},
				Spec: SchemaReplicationSpec{
					UpstreamSecret: &corev1.LocalObjectReference{
						Name: "a-secret",
					},
					Endpoints: "abc.rmq.com:1234",
					RabbitmqClusterReference: RabbitmqClusterReference{
						ConnectionSecret: &corev1.LocalObjectReference{
							Name: "a-secret",
						},
					},
				},
			}
			newObj := connectionScr.DeepCopy()
			newObj.Spec.RabbitmqClusterReference.ConnectionSecret.Name = "newObj-secret"
			_, err := newObj.ValidateUpdate(rootCtx, &connectionScr, newObj)
			Expect(err).To(MatchError(ContainSubstring("update on rabbitmqClusterReference is forbidden")))
		})

		It("allows updates on spec.upstreamSecret", func() {
			updated := replication.DeepCopy()
			updated.Spec.UpstreamSecret = &corev1.LocalObjectReference{
				Name: "a-different-secret",
			}
			_, err := updated.ValidateUpdate(rootCtx, &replication, updated)
			Expect(err).ToNot(HaveOccurred())
		})

		It("allows updates on spec.endpoints", func() {
			updated := replication.DeepCopy()
			updated.Spec.Endpoints = "abc.new-rmq:1111"
			_, err := updated.ValidateUpdate(rootCtx, &replication, updated)
			Expect(err).ToNot(HaveOccurred())
		})
	})
})
