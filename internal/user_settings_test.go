package internal_test

import (
	"strings"

	rabbithole "github.com/michaelklishin/rabbit-hole/v2"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	topologyv1alpha1 "github.com/rabbitmq/messaging-topology-operator/api/v1alpha1"
	"github.com/rabbitmq/messaging-topology-operator/internal"
)

var _ = Describe("GenerateUserSettings", func() {
	var userSpec topologyv1alpha1.UserSpec

	BeforeEach(func() {
		userSpec = topologyv1alpha1.UserSpec{
			Name: "my-user",
			Tags: []topologyv1alpha1.UserTag{"administrator", "monitoring"},
			RabbitmqClusterReference: topologyv1alpha1.RabbitmqClusterReference{
				Name:      "my-rabbit-cluster",
				Namespace: "my-rabbit-namespace",
			},
		}
	})

	It("generates the expected rabbithole.UserSettings", func() {
		settings, err := internal.GenerateUserSettings(userSpec)
		Expect(err).NotTo(HaveOccurred())
		Expect(settings.Name).To(Equal("my-user"))
		Expect(strings.Split(settings.Tags, ",")).To(ConsistOf("administrator", "monitoring"))
		Expect(settings.PasswordHash).NotTo(BeEmpty())
		Expect(settings.HashingAlgorithm.String()).To(Equal(string(rabbithole.HashingAlgorithmSHA256.String())))
	})

})
