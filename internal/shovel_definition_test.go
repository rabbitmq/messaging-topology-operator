package internal

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	topology "github.com/rabbitmq/messaging-topology-operator/api/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

var _ = Describe("GenerateShovelDefinition", func() {
	var shovel *topology.Shovel

	BeforeEach(func() {
		shovel = &topology.Shovel{
			ObjectMeta: metav1.ObjectMeta{
				Name: "new-shovel",
			},
			Spec: topology.ShovelSpec{
				Vhost: "/new-vhost",
				Name:  "new-shovel",
			},
		}
	})

	It("sets source and destination uris correctly for a single uri", func() {
		definition, err := GenerateShovelDefinition(shovel, "a-rabbitmq-src@test.com", "a-rabbitmq-dest@test.com")
		Expect(err).NotTo(HaveOccurred())
		Expect(definition.SourceURI).To(ConsistOf("a-rabbitmq-src@test.com"))
		Expect(definition.DestinationURI).To(ConsistOf("a-rabbitmq-dest@test.com"))
	})

	It("sets source and destination uris correctly for multiple uris", func() {
		definition, err := GenerateShovelDefinition(shovel, "a-rabbitmq-src@test.com0,a-rabbitmq-src@test1.com", "a-rabbitmq-dest@test0.com,a-rabbitmq-dest@test1.com")
		Expect(err).NotTo(HaveOccurred())
		Expect(definition.SourceURI).To(ConsistOf("a-rabbitmq-src@test.com0", "a-rabbitmq-src@test1.com"))
		Expect(definition.DestinationURI).To(ConsistOf("a-rabbitmq-dest@test0.com", "a-rabbitmq-dest@test1.com"))
	})

	It("sets 'AckMode' correctly", func() {
		shovel.Spec.AckMode = "on-publish"
		definition, err := GenerateShovelDefinition(shovel, "", "")
		Expect(err).NotTo(HaveOccurred())
		Expect(definition.AckMode).To(Equal("on-publish"))
	})

	It("sets 'AddForwardHeaders' correctly", func() {
		shovel.Spec.AddForwardHeaders = true
		definition, err := GenerateShovelDefinition(shovel, "", "")
		Expect(err).NotTo(HaveOccurred())
		Expect(definition.AddForwardHeaders).To(BeTrue())
	})

	It("sets 'DeleteAfter' correctly", func() {
		shovel.Spec.DeleteAfter = "never"
		definition, err := GenerateShovelDefinition(shovel, "", "")
		Expect(err).NotTo(HaveOccurred())
		Expect(string(definition.DeleteAfter)).To(Equal("never"))
	})

	It("sets 'DestinationAddForwardHeaders' correctly", func() {
		shovel.Spec.DestinationAddForwardHeaders = true
		definition, err := GenerateShovelDefinition(shovel, "", "")
		Expect(err).NotTo(HaveOccurred())
		Expect(definition.DestinationAddForwardHeaders).To(BeTrue())
	})

	It("sets 'DestinationAddTimestampHeader' correctly", func() {
		shovel.Spec.DestinationAddTimestampHeader = true
		definition, err := GenerateShovelDefinition(shovel, "", "")
		Expect(err).NotTo(HaveOccurred())
		Expect(definition.DestinationAddTimestampHeader).To(BeTrue())
	})

	It("sets 'DestinationAddress' correctly", func() {
		shovel.Spec.DestinationAddress = "an-address"
		definition, err := GenerateShovelDefinition(shovel, "", "")
		Expect(err).NotTo(HaveOccurred())
		Expect(definition.DestinationAddress).To(Equal("an-address"))
	})

	It("sets 'DestinationApplicationProperties' correctly", func() {
		shovel.Spec.DestinationApplicationProperties = &runtime.RawExtension{Raw: []byte(`{"key": "a-property"}`)}
		definition, err := GenerateShovelDefinition(shovel, "", "")
		Expect(err).NotTo(HaveOccurred())
		Expect(definition.DestinationApplicationProperties).To(HaveKeyWithValue("key", "a-property"))
	})

	It("sets 'DestinationExchange' correctly", func() {
		shovel.Spec.DestinationExchange = "an-exchange"
		definition, err := GenerateShovelDefinition(shovel, "", "")
		Expect(err).NotTo(HaveOccurred())
		Expect(definition.DestinationExchange).To(Equal("an-exchange"))
	})

	It("sets 'DestinationExchangeKey' correctly", func() {
		shovel.Spec.DestinationExchangeKey = "a-key"
		definition, err := GenerateShovelDefinition(shovel, "", "")
		Expect(err).NotTo(HaveOccurred())
		Expect(definition.DestinationExchangeKey).To(Equal("a-key"))
	})

	It("sets 'DestinationProperties' correctly", func() {
		shovel.Spec.DestinationProperties = &runtime.RawExtension{Raw: []byte(`{"dest-property": "a-value"}`)}
		definition, err := GenerateShovelDefinition(shovel, "", "")
		Expect(err).NotTo(HaveOccurred())
		Expect(definition.DestinationProperties).To(HaveKeyWithValue("dest-property", "a-value"))
	})

	It("sets 'DestinationProtocol' correctly", func() {
		shovel.Spec.DestinationProtocol = "amqp10"
		definition, err := GenerateShovelDefinition(shovel, "", "")
		Expect(err).NotTo(HaveOccurred())
		Expect(definition.DestinationProtocol).To(Equal("amqp10"))
	})

	It("sets 'DestinationPublishProperties' correctly", func() {
		shovel.Spec.DestinationPublishProperties = &runtime.RawExtension{Raw: []byte(`{"delivery_mode": 1}`)}
		definition, err := GenerateShovelDefinition(shovel, "", "")
		Expect(err).NotTo(HaveOccurred())
		Expect(definition.DestinationPublishProperties).To(HaveKeyWithValue("delivery_mode", float64(1)))
	})

	It("sets 'DestinationQueue' correctly", func() {
		shovel.Spec.DestinationQueue = "a-destination-queue"
		definition, err := GenerateShovelDefinition(shovel, "", "")
		Expect(err).NotTo(HaveOccurred())
		Expect(definition.DestinationQueue).To(Equal("a-destination-queue"))
	})

	It("sets 'PrefetchCount' correctly", func() {
		shovel.Spec.PrefetchCount = 200
		definition, err := GenerateShovelDefinition(shovel, "", "")
		Expect(err).NotTo(HaveOccurred())
		Expect(definition.PrefetchCount).To(Equal(200))
	})

	It("sets 'ReconnectDelay' correctly", func() {
		shovel.Spec.ReconnectDelay = 2000
		definition, err := GenerateShovelDefinition(shovel, "", "")
		Expect(err).NotTo(HaveOccurred())
		Expect(definition.ReconnectDelay).To(Equal(2000))
	})

	It("sets 'SourceAddress' correctly", func() {
		shovel.Spec.SourceAddress = "an-address"
		definition, err := GenerateShovelDefinition(shovel, "", "")
		Expect(err).NotTo(HaveOccurred())
		Expect(definition.SourceAddress).To(Equal("an-address"))
	})

	It("sets 'SourceDeleteAfter' correctly", func() {
		shovel.Spec.SourceDeleteAfter = "10000000"
		definition, err := GenerateShovelDefinition(shovel, "", "")
		Expect(err).NotTo(HaveOccurred())
		Expect(string(definition.SourceDeleteAfter)).To(Equal("10000000"))
	})

	It("sets 'SourceExchange' correctly", func() {
		shovel.Spec.SourceExchange = "an-source-exchange"
		definition, err := GenerateShovelDefinition(shovel, "", "")
		Expect(err).NotTo(HaveOccurred())
		Expect(definition.SourceExchange).To(Equal("an-source-exchange"))
	})

	It("sets 'SourceExchangeKey' correctly", func() {
		shovel.Spec.SourceExchangeKey = "an-source-exchange-key"
		definition, err := GenerateShovelDefinition(shovel, "", "")
		Expect(err).NotTo(HaveOccurred())
		Expect(definition.SourceExchangeKey).To(Equal("an-source-exchange-key"))
	})

	It("sets 'SourcePrefetchCount' correctly", func() {
		shovel.Spec.SourcePrefetchCount = 200
		definition, err := GenerateShovelDefinition(shovel, "", "")
		Expect(err).NotTo(HaveOccurred())
		Expect(definition.SourcePrefetchCount).To(Equal(200))
	})

	It("sets 'SourceProtocol' correctly", func() {
		shovel.Spec.SourceProtocol = "amqp09"
		definition, err := GenerateShovelDefinition(shovel, "", "")
		Expect(err).NotTo(HaveOccurred())
		Expect(definition.SourceProtocol).To(Equal("amqp09"))
	})

	It("sets 'SourceQueue' correctly", func() {
		shovel.Spec.SourceQueue = "a-great-queue"
		definition, err := GenerateShovelDefinition(shovel, "", "")
		Expect(err).NotTo(HaveOccurred())
		Expect(definition.SourceQueue).To(Equal("a-great-queue"))
	})
})
