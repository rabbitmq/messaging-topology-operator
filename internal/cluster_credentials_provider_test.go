package internal

import (
	"context"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	rabbitmqv1beta1 "github.com/rabbitmq/cluster-operator/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

)


var _ = Describe("GetCredentialProviders", func() {
	var (
		err                  error
		client               client.Client
		ccp 				 *RabbitMQClusterCredentialsProvider
		cluster              *rabbitmqv1beta1.RabbitmqCluster
		ctx					 context.Context
		credsProv			 CredentialsProvider
		svc					 *corev1.Service

	)
	BeforeEach(func() {
		ccp, err = newRabbitMQClusterCredentialsProvider(client)
		if err == nil {
			Fail(err.Error())
		}
	})
	JustBeforeEach(func() {
		credsProv, svc, err = ccp.GetCredentialsProvider(ctx, "rabbitmq-system", cluster)
	})

	When("when no SecretBackend configured", func() {
		BeforeEach(func() {
			cluster = &rabbitmqv1beta1.RabbitmqCluster{
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
							Name:      "rmq",
							Namespace: "rabbitmq-system",
						},
					},
				},
			}

		})

		It("retrieves credentials from k8s secret", func() {
			Expect(credsProv).NotTo(BeNil())
			Expect(svc).NotTo(BeNil())
		})
	})
})