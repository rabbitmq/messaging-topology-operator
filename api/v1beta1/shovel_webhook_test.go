package v1beta1

import (
	"context"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

var _ = Describe("shovel webhook", func() {
	var (
		shovel = Shovel{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test",
			},
			Spec: ShovelSpec{
				Name:  "test-upstream",
				Vhost: "/a-vhost",
				UriSecret: &corev1.LocalObjectReference{
					Name: "a-secret",
				},
				AckMode:                          "no-ack",
				AddForwardHeaders:                true,
				DeleteAfter:                      "never",
				DestinationAddForwardHeaders:     true,
				DestinationAddTimestampHeader:    true,
				DestinationAddress:               "myQueue",
				DestinationApplicationProperties: &runtime.RawExtension{Raw: []byte(`{"key": "a-property"}`)},
				DestinationExchange:              "an-exchange",
				DestinationExchangeKey:           "a-key",
				DestinationProperties:            &runtime.RawExtension{Raw: []byte(`{"key": "a-property"}`)},
				DestinationProtocol:              "amqp091",
				DestinationPublishProperties:     &runtime.RawExtension{Raw: []byte(`{"delivery_mode": 1}`)},
				DestinationMessageAnnotations:    &runtime.RawExtension{Raw: []byte(`{"a-key": "an-annotation"}`)},
				DestinationQueue:                 "a-queue",
				PrefetchCount:                    10,
				ReconnectDelay:                   10,
				SourceAddress:                    "myQueue",
				SourceDeleteAfter:                "never",
				SourceExchange:                   "an-exchange",
				SourceExchangeKey:                "a-key",
				SourcePrefetchCount:              10,
				SourceProtocol:                   "amqp091",
				SourceQueue:                      "a-queue",
				SourceConsumerArgs:               &runtime.RawExtension{Raw: []byte(`{"x-priority": 1}`)},
				RabbitmqClusterReference: RabbitmqClusterReference{
					Name: "a-cluster",
				},
			},
		}
		rootCtx         = context.Background()
		shovelValidator ShovelValidator
	)

	Context("ValidateCreate", func() {
		It("does not allow both spec.rabbitmqClusterReference.name and spec.rabbitmqClusterReference.connectionSecret be configured", func() {
			notAllowed := shovel.DeepCopy()
			notAllowed.Spec.RabbitmqClusterReference.ConnectionSecret = &corev1.LocalObjectReference{Name: "some-secret"}
			_, err := shovelValidator.ValidateCreate(rootCtx, notAllowed)
			Expect(err).To(MatchError(ContainSubstring("invalid RabbitmqClusterReference: do not provide both name and connectionSecret")))
		})

		It("spec.rabbitmqClusterReference.name and spec.rabbitmqClusterReference.connectionSecret cannot both be empty", func() {
			notAllowed := shovel.DeepCopy()
			notAllowed.Spec.RabbitmqClusterReference.Name = ""
			notAllowed.Spec.RabbitmqClusterReference.ConnectionSecret = nil
			_, err := shovelValidator.ValidateCreate(rootCtx, notAllowed)
			Expect(err).To(MatchError(ContainSubstring("invalid RabbitmqClusterReference: must provide either name or connectionSecret")))
		})

		It("spec.srcAddress must be set if spec.srcProtocol is amqp10", func() {
			notValid := shovel.DeepCopy()
			notValid.Spec.SourceProtocol = "amqp10"
			notValid.Spec.SourceAddress = ""
			_, err := shovelValidator.ValidateCreate(rootCtx, notValid)
			Expect(err).To(MatchError(ContainSubstring("must specify spec.srcAddress when spec.srcProtocol is amqp10")))
		})

		It("spec.destAddress must be set if spec.destProtocol is amqp10", func() {
			notValid := shovel.DeepCopy()
			notValid.Spec.DestinationProtocol = "amqp10"
			notValid.Spec.DestinationAddress = ""
			_, err := shovelValidator.ValidateCreate(rootCtx, notValid)
			Expect(err).To(MatchError(ContainSubstring("must specify spec.destAddress when spec.destProtocol is amqp10")))
		})
	})

	Context("ValidateUpdate", func() {
		It("does not allow updates on name", func() {
			newShovel := shovel.DeepCopy()
			newShovel.Spec.Name = "another-shovel"
			_, err := shovelValidator.ValidateUpdate(rootCtx, &shovel, newShovel)
			Expect(err).To(MatchError(ContainSubstring("updates on name, vhost and rabbitmqClusterReference are all forbidden")))
		})

		It("does not allow updates on vhost", func() {
			newShovel := shovel.DeepCopy()
			newShovel.Spec.Vhost = "another-vhost"
			_, err := shovelValidator.ValidateUpdate(rootCtx, &shovel, newShovel)
			Expect(err).To(MatchError(ContainSubstring("updates on name, vhost and rabbitmqClusterReference are all forbidden")))
		})

		It("does not allow updates on RabbitmqClusterReference", func() {
			newShovel := shovel.DeepCopy()
			newShovel.Spec.RabbitmqClusterReference = RabbitmqClusterReference{
				Name: "another-cluster",
			}
			_, err := shovelValidator.ValidateUpdate(rootCtx, &shovel, newShovel)
			Expect(err).To(MatchError(ContainSubstring("updates on name, vhost and rabbitmqClusterReference are all forbidden")))
		})

		It("spec.srcAddress must be set if spec.srcProtocol is amqp10", func() {
			newShovel := shovel.DeepCopy()
			newShovel.Spec.SourceProtocol = "amqp10"
			newShovel.Spec.SourceAddress = ""
			_, err := shovelValidator.ValidateCreate(rootCtx, newShovel)
			Expect(err).To(MatchError(ContainSubstring("must specify spec.srcAddress when spec.srcProtocol is amqp10")))
		})

		It("spec.destAddress must be set if spec.destProtocol is amqp10", func() {
			newShovel := shovel.DeepCopy()
			newShovel.Spec.DestinationProtocol = "amqp10"
			newShovel.Spec.DestinationAddress = ""
			_, err := shovelValidator.ValidateCreate(rootCtx, newShovel)
			Expect(err).To(MatchError(ContainSubstring("must specify spec.destAddress when spec.destProtocol is amqp10")))
		})

		It("does not allow updates on rabbitmqClusterReference.connectionSecret", func() {
			connectionScr := Shovel{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
				},
				Spec: ShovelSpec{
					Name:  "test-upstream",
					Vhost: "/a-vhost",
					UriSecret: &corev1.LocalObjectReference{
						Name: "a-secret",
					},
					RabbitmqClusterReference: RabbitmqClusterReference{
						ConnectionSecret: &corev1.LocalObjectReference{
							Name: "a-secret",
						},
					},
				},
			}
			newShovel := connectionScr.DeepCopy()
			newShovel.Spec.RabbitmqClusterReference.ConnectionSecret.Name = "new-secret"
			_, err := shovelValidator.ValidateUpdate(rootCtx, &connectionScr, newShovel)
			Expect(err).To(MatchError(ContainSubstring("updates on name, vhost and rabbitmqClusterReference are all forbidden")))
		})

		It("allows updates on shovel configurations", func() {
			newShovel := shovel.DeepCopy()
			newShovel.Spec.UriSecret = &corev1.LocalObjectReference{Name: "another-secret"}
			newShovel.Spec.AckMode = "on-confirm"
			newShovel.Spec.AddForwardHeaders = false
			newShovel.Spec.DeleteAfter = "100"
			newShovel.Spec.DestinationAddForwardHeaders = false
			newShovel.Spec.DestinationAddTimestampHeader = false
			newShovel.Spec.DestinationApplicationProperties = &runtime.RawExtension{Raw: []byte(`{"key": "new"}`)}
			newShovel.Spec.DestinationExchange = "new-exchange"
			newShovel.Spec.DestinationExchangeKey = "new-key"
			newShovel.Spec.DestinationProperties = &runtime.RawExtension{Raw: []byte(`{"key": "new"}`)}
			newShovel.Spec.DestinationProtocol = "new"
			newShovel.Spec.DestinationPublishProperties = &runtime.RawExtension{Raw: []byte(`{"key": "new"}`)}
			newShovel.Spec.DestinationMessageAnnotations = &runtime.RawExtension{Raw: []byte(`{"key": "new-annotation"}`)}
			newShovel.Spec.DestinationQueue = "another-queue"
			newShovel.Spec.PrefetchCount = 20
			newShovel.Spec.PrefetchCount = 10000
			newShovel.Spec.SourceAddress = "another-queue"
			newShovel.Spec.SourceDeleteAfter = "100"
			newShovel.Spec.SourceExchange = "another-exchange"
			newShovel.Spec.SourceExchangeKey = "another-key"
			newShovel.Spec.SourcePrefetchCount = 50
			newShovel.Spec.SourceProtocol = "another-protocol"
			newShovel.Spec.SourceQueue = "another-queue"
			newShovel.Spec.SourceConsumerArgs = &runtime.RawExtension{Raw: []byte(`{"x-priority": 10}`)}
			_, err := shovelValidator.ValidateUpdate(rootCtx, &shovel, newShovel)
			Expect(err).ToNot(HaveOccurred())
		})

		It("allows updates on shovel.spec.deletionPolicy", func() {
			newShovel := shovel.DeepCopy()
			newShovel.Spec.DeletionPolicy = "retain"
			_, err := shovelValidator.ValidateUpdate(rootCtx, &shovel, newShovel)
			Expect(err).ToNot(HaveOccurred())
		})
	})
})
