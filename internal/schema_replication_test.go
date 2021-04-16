package internal_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/rabbitmq/messaging-topology-operator/internal"
	corev1 "k8s.io/api/core/v1"
)

var _ = Describe("GenerateSchemaReplicationParameters", func() {
	var secret corev1.Secret

	BeforeEach(func() {
		secret = corev1.Secret{
			Type: corev1.SecretTypeOpaque,
			Data: map[string][]byte{
				"username":  []byte("a-random-user"),
				"password":  []byte("a-random-password"),
				"endpoints": []byte("a.endpoints.local:5672,b.endpoints.local:5672,c.endpoints.local:5672"),
			},
		}
	})

	It("generates expected replication parameters", func() {
		parameters, err := internal.GenerateSchemaReplicationParameters(&secret)
		Expect(err).NotTo(HaveOccurred())
		Expect(parameters.Username).To(Equal("a-random-user"))
		Expect(parameters.Password).To(Equal("a-random-password"))
		Expect(parameters.Endpoints).To(Equal([]string{
			"a.endpoints.local:5672",
			"b.endpoints.local:5672",
			"c.endpoints.local:5672",
		}))
	})
})
