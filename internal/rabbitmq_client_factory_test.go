package internal_test

import (
	"net/http"

	rabbithole "github.com/michaelklishin/rabbit-hole/v2"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
	"github.com/rabbitmq/messaging-topology-operator/internal"
)

var _ = Describe("RabbitholeClientFactory", func() {

	var fakeRabbitMQServer *ghttp.Server
	BeforeEach(func() {
		fakeRabbitMQServer = mockRabbitMQServer()
		fakeRabbitMQServer.RouteToHandler("PUT", "/api/users/example-user", func(w http.ResponseWriter, req *http.Request) {
		})
	})
	AfterEach(func() {
		fakeRabbitMQServer.Close()
	})
	It("generates a rabbithole client which is compliant with the factory interface", func() {
		generatedClient, err := internal.RabbitholeClientFactory(fakeRabbitMQServer.URL(), "guest", "guest")
		Expect(err).NotTo(HaveOccurred())
		Expect(generatedClient).NotTo(BeNil())

		_, err = generatedClient.PutUser("example-user", rabbithole.UserSettings{})
		Expect(err).NotTo(HaveOccurred())
		Expect(len(fakeRabbitMQServer.ReceivedRequests())).To(Equal(1))
	})

})
