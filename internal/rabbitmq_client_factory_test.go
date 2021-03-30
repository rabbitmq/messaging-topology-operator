package internal_test

import (
	"context"
	"net/http"
	"net/url"
	"strconv"

	rabbithole "github.com/michaelklishin/rabbit-hole/v2"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
	rabbitmqv1beta1 "github.com/rabbitmq/cluster-operator/api/v1beta1"
	topology "github.com/rabbitmq/messaging-topology-operator/api/v1alpha2"
	"github.com/rabbitmq/messaging-topology-operator/internal"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("RabbitholeClientFactory", func() {

	var (
		ctx                      = context.Background()
		fakeRabbitMQServer       *ghttp.Server
		existingRabbitMQCluster  *rabbitmqv1beta1.RabbitmqCluster
		existingRabbitMQUsername = "abc123"
		existingRabbitMQPassword = "foo1234"
		fakeClient               client.Client
	)
	BeforeEach(func() {
		fakeRabbitMQServer = mockRabbitMQServer()
		fakeRabbitMQServer.RouteToHandler("PUT", "/api/users/example-user", func(w http.ResponseWriter, req *http.Request) {
			user, password, ok := req.BasicAuth()
			if !(ok && user == existingRabbitMQUsername && password == existingRabbitMQPassword) {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
		})
		fakeRabbitMQURL, err := url.Parse(fakeRabbitMQServer.URL())
		Expect(err).NotTo(HaveOccurred())
		fakeRabbitMQPort, err := strconv.Atoi(fakeRabbitMQURL.Port())
		Expect(err).NotTo(HaveOccurred())

		existingRabbitMQCluster = &rabbitmqv1beta1.RabbitmqCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "rmq",
				Namespace: "rabbitmq-system",
			},
			Status: rabbitmqv1beta1.RabbitmqClusterStatus{
				DefaultUser: &rabbitmqv1beta1.RabbitmqClusterDefaultUser{
					SecretReference: &rabbitmqv1beta1.RabbitmqClusterSecretReference{
						Name:      "rmq-default-user-credentials",
						Namespace: "rabbitmq-system",
					},
					ServiceReference: &rabbitmqv1beta1.RabbitmqClusterServiceReference{
						Name:      "rmq-service",
						Namespace: "rabbitmq-system",
					},
				},
			},
		}
		existingCredentialSecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "rmq-default-user-credentials",
				Namespace: "rabbitmq-system",
			},
			Data: map[string][]byte{
				"username": []byte(existingRabbitMQUsername),
				"password": []byte(existingRabbitMQPassword),
			},
		}
		existingService := &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "rmq-service",
				Namespace: "rabbitmq-system",
			},
			Spec: corev1.ServiceSpec{
				ClusterIP: fakeRabbitMQURL.Hostname(),
				Ports: []corev1.ServicePort{
					corev1.ServicePort{
						Name: "management",
						Port: int32(fakeRabbitMQPort),
					},
				},
			},
		}
		objs := []runtime.Object{existingRabbitMQCluster, existingCredentialSecret, existingService}
		s := scheme.Scheme
		s.AddKnownTypes(rabbitmqv1beta1.SchemeBuilder.GroupVersion, existingRabbitMQCluster)
		fakeClient = fake.NewClientBuilder().WithScheme(s).WithRuntimeObjects(objs...).Build()
	})
	AfterEach(func() {
		fakeRabbitMQServer.Close()
	})
	It("generates a rabbithole client which makes sucessful requests to the RabbitMQ Server", func() {
		generatedClient, err := internal.RabbitholeClientFactory(ctx, fakeClient, topology.RabbitmqClusterReference{Name: existingRabbitMQCluster.Name}, existingRabbitMQCluster.Namespace)
		Expect(err).NotTo(HaveOccurred())
		Expect(generatedClient).NotTo(BeNil())

		_, err = generatedClient.PutUser("example-user", rabbithole.UserSettings{})
		Expect(err).NotTo(HaveOccurred())
		Expect(len(fakeRabbitMQServer.ReceivedRequests())).To(Equal(1))
	})

})
