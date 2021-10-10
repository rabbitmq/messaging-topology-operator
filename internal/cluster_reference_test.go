package internal_test

import (
	"context"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	rabbitmqv1beta1 "github.com/rabbitmq/cluster-operator/api/v1beta1"
	topology "github.com/rabbitmq/messaging-topology-operator/api/v1beta1"
	"github.com/rabbitmq/messaging-topology-operator/internal"
	"github.com/rabbitmq/messaging-topology-operator/internal/internalfakes"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("ParseRabbitmqClusterReference", func() {
	var (
		objs                     []runtime.Object
		fakeClient               client.Client
		existingRabbitMQUsername = "abc123"
		existingRabbitMQPassword = "foo1234"
		existingRabbitMQCluster  *rabbitmqv1beta1.RabbitmqCluster
		existingService          *corev1.Service
		ctx                      = context.Background()
		fakeCredentialsProvider  *internalfakes.FakeCredentialsProvider
		fakeCredentialsLocator   *internalfakes.FakeCredentialsLocator
	)
	JustBeforeEach(func() {
		s := scheme.Scheme
		s.AddKnownTypes(rabbitmqv1beta1.SchemeBuilder.GroupVersion, &rabbitmqv1beta1.RabbitmqCluster{})
		fakeClient = fake.NewClientBuilder().WithScheme(s).WithRuntimeObjects(objs...).Build()
		fakeCredentialsProvider = &internalfakes.FakeCredentialsProvider{}
		fakeCredentialsLocator = &internalfakes.FakeCredentialsLocator{}
		fakeCredentialsProvider.GetUserReturns(existingRabbitMQUsername)
		fakeCredentialsProvider.GetPasswordReturns(existingRabbitMQPassword)
		fakeCredentialsLocator.ReadCredentialsReturns(fakeCredentialsProvider, nil)
	})

	When("the RabbitmqCluster is configured without TLS", func() {
		BeforeEach(func() {
			existingRabbitMQCluster = &rabbitmqv1beta1.RabbitmqCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "rmq",
					Namespace: "rabbitmq-system",
				},
				Status: rabbitmqv1beta1.RabbitmqClusterStatus{
					Binding: &corev1.LocalObjectReference{
						Name: "rmq-default-user-credentials",
					},
					DefaultUser: &rabbitmqv1beta1.RabbitmqClusterDefaultUser{
						ServiceReference: &rabbitmqv1beta1.RabbitmqClusterServiceReference{
							Name:      "rmq-service",
							Namespace: "rabbitmq-system",
						},
					},
				},
				Spec: rabbitmqv1beta1.RabbitmqClusterSpec{
					SecretBackend: rabbitmqv1beta1.SecretBackend{
						Vault: &rabbitmqv1beta1.VaultSpec{
							DefaultUserPath: "some/secret/path",
						},
					},
				},
			}
			existingService = &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "rmq-service",
					Namespace: "rabbitmq-system",
				},
				Spec: corev1.ServiceSpec{
					ClusterIP: "1.2.3.4",
					Ports: []corev1.ServicePort{
						{
							Name: "management",
							Port: int32(15672),
						},
					},
				},
			}
			objs = []runtime.Object{existingRabbitMQCluster, existingService}
		})

		It("generates a rabbithole client which makes successful requests to the RabbitMQ Server", func() {
			rmq, svc, credsProvider, err := internal.ParseRabbitmqClusterReference(ctx, fakeClient, topology.RabbitmqClusterReference{Name: existingRabbitMQCluster.Name}, existingRabbitMQCluster.Namespace, fakeCredentialsLocator)
			Expect(err).NotTo(HaveOccurred())
			Expect(rmq.ObjectMeta).To(Equal(existingRabbitMQCluster.ObjectMeta))
			Expect(rmq.Status).To(Equal(existingRabbitMQCluster.Status))
			Expect(svc.ObjectMeta).To(Equal(existingService.ObjectMeta))
			Expect(svc.Spec).To(Equal(existingService.Spec))
			Expect(credsProvider.GetUser()).To(Equal(existingRabbitMQUsername))
			Expect(credsProvider.GetPassword()).To(Equal(existingRabbitMQPassword))
		})

		When("RabbitmqCluster does not have status.binding set", func() {
			BeforeEach(func() {
				*existingRabbitMQCluster = rabbitmqv1beta1.RabbitmqCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "rmq-incomplete",
						Namespace: "rabbitmq-system",
					},
					Status: rabbitmqv1beta1.RabbitmqClusterStatus{
						DefaultUser: &rabbitmqv1beta1.RabbitmqClusterDefaultUser{
							ServiceReference: &rabbitmqv1beta1.RabbitmqClusterServiceReference{
								Name:      "rmq-service",
								Namespace: "rabbitmq-system",
							},
						},
					},
					Spec: rabbitmqv1beta1.RabbitmqClusterSpec{
						SecretBackend: rabbitmqv1beta1.SecretBackend{
							Vault: &rabbitmqv1beta1.VaultSpec{
								DefaultUserPath: "some/secret/path",
							},
						},
					},
				}
			})

			It("errors", func() {
				_, _, _, err := internal.ParseRabbitmqClusterReference(ctx, fakeClient, topology.RabbitmqClusterReference{Name: existingRabbitMQCluster.Name}, existingRabbitMQCluster.Namespace, fakeCredentialsLocator)
				Expect(err).To(MatchError("no status.binding set"))
			})
		})

		When("RabbitmqCluster does not have spec.secretBackend.vault.defaultUserPath set", func() {
			BeforeEach(func() {
				*existingRabbitMQCluster = rabbitmqv1beta1.RabbitmqCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "rmq-incomplete",
						Namespace: "rabbitmq-system",
					},
					Status: rabbitmqv1beta1.RabbitmqClusterStatus{
						Binding: &corev1.LocalObjectReference{
							Name: "rmq-default-user-credentials",
						},
						DefaultUser: &rabbitmqv1beta1.RabbitmqClusterDefaultUser{
							ServiceReference: &rabbitmqv1beta1.RabbitmqClusterServiceReference{
								Name:      "rmq-service",
								Namespace: "rabbitmq-system",
							},
						},
					},
					Spec: rabbitmqv1beta1.RabbitmqClusterSpec{
						SecretBackend: rabbitmqv1beta1.SecretBackend{
							Vault: &rabbitmqv1beta1.VaultSpec{
								DefaultUserPath: "",
							},
						},
					},
				}
			})

			It("errors", func() {
				_, _, _, err := internal.ParseRabbitmqClusterReference(ctx, fakeClient, topology.RabbitmqClusterReference{Name: existingRabbitMQCluster.Name}, existingRabbitMQCluster.Namespace, fakeCredentialsLocator)
				Expect(err).To(MatchError("no spec.secretBackend.vault.defaultUserPath"))
			})
		})
	})
})
