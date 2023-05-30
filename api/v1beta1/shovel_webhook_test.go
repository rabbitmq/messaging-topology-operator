package v1beta1

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

var _ = Describe("shovel webhook", func() {
	var shovel = Shovel{
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

	Context("ValidateCreate", func() {
		It("does not allow both spec.rabbitmqClusterReference.name and spec.rabbitmqClusterReference.connectionSecret be configured", func() {
			notAllowed := shovel.DeepCopy()
			notAllowed.Spec.RabbitmqClusterReference.ConnectionSecret = &corev1.LocalObjectReference{Name: "some-secret"}
			Expect(apierrors.IsForbidden(ignoreNilWarning(notAllowed.ValidateCreate()))).To(BeTrue())
		})

		It("spec.rabbitmqClusterReference.name and spec.rabbitmqClusterReference.connectionSecret cannot both be empty", func() {
			notAllowed := shovel.DeepCopy()
			notAllowed.Spec.RabbitmqClusterReference.Name = ""
			notAllowed.Spec.RabbitmqClusterReference.ConnectionSecret = nil
			Expect(apierrors.IsForbidden(ignoreNilWarning(notAllowed.ValidateCreate()))).To(BeTrue())
		})

		It("spec.srcAddress must be set if spec.srcProtocol is amqp10", func() {
			notValid := shovel.DeepCopy()
			notValid.Spec.SourceProtocol = "amqp10"
			notValid.Spec.SourceAddress = ""
			Expect(apierrors.IsInvalid(ignoreNilWarning(notValid.ValidateCreate()))).To(BeTrue())
		})

		It("spec.destAddress must be set if spec.destProtocol is amqp10", func() {
			notValid := shovel.DeepCopy()
			notValid.Spec.DestinationProtocol = "amqp10"
			notValid.Spec.DestinationAddress = ""
			Expect(apierrors.IsInvalid(ignoreNilWarning(notValid.ValidateCreate()))).To(BeTrue())
		})
	})

	Context("ValidateUpdate", func() {
		It("does not allow updates on name", func() {
			newShovel := shovel.DeepCopy()
			newShovel.Spec.Name = "another-shovel"
			Expect(apierrors.IsForbidden(ignoreNilWarning(newShovel.ValidateUpdate(&shovel)))).To(BeTrue())
		})

		It("does not allow updates on vhost", func() {
			newShovel := shovel.DeepCopy()
			newShovel.Spec.Vhost = "another-vhost"
			Expect(apierrors.IsForbidden(ignoreNilWarning(newShovel.ValidateUpdate(&shovel)))).To(BeTrue())
		})

		It("does not allow updates on RabbitmqClusterReference", func() {
			newShovel := shovel.DeepCopy()
			newShovel.Spec.RabbitmqClusterReference = RabbitmqClusterReference{
				Name: "another-cluster",
			}
			Expect(apierrors.IsForbidden(ignoreNilWarning(newShovel.ValidateUpdate(&shovel)))).To(BeTrue())
		})

		It("spec.srcAddress must be set if spec.srcProtocol is amqp10", func() {
			newShovel := shovel.DeepCopy()
			newShovel.Spec.SourceProtocol = "amqp10"
			newShovel.Spec.SourceAddress = ""
			Expect(apierrors.IsInvalid(ignoreNilWarning(newShovel.ValidateUpdate(&shovel)))).To(BeTrue())
		})

		It("spec.destAddress must be set if spec.destProtocol is amqp10", func() {
			newShovel := shovel.DeepCopy()
			newShovel.Spec.DestinationProtocol = "amqp10"
			newShovel.Spec.DestinationAddress = ""
			Expect(apierrors.IsInvalid(ignoreNilWarning(newShovel.ValidateUpdate(&shovel)))).To(BeTrue())
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
			new := connectionScr.DeepCopy()
			new.Spec.RabbitmqClusterReference.ConnectionSecret.Name = "new-secret"
			Expect(apierrors.IsForbidden(ignoreNilWarning(new.ValidateUpdate(&connectionScr)))).To(BeTrue())
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
			Expect(ignoreNilWarning(newShovel.ValidateUpdate(&shovel))).To(Succeed())
		})
	})
})
