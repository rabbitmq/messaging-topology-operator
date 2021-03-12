package system_tests

import (
	"context"
	rabbithole "github.com/michaelklishin/rabbit-hole/v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"

	topologyv1alpha1 "github.com/rabbitmq/messaging-topology-operator/api/v1alpha1"
)

var _ = Describe("Exchange", func() {
	var (
		namespace = MustHaveEnv("NAMESPACE")
		ctx       = context.Background()
		exchange  *topologyv1alpha1.Exchange
	)

	BeforeEach(func() {
		exchange = &topologyv1alpha1.Exchange{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "exchange-test",
				Namespace: namespace,
			},
			Spec: topologyv1alpha1.ExchangeSpec{
				RabbitmqClusterReference: topologyv1alpha1.RabbitmqClusterReference{
					Name:      rmq.Name,
					Namespace: rmq.Namespace,
				},
				Name:       "exchange-test",
				Type:       "fanout",
				AutoDelete: false,
				Durable:    true,
				Arguments: &runtime.RawExtension{
					Raw: []byte(`{"alternate-exchange": "system-test"}`),
				},
			},
		}
	})

	It("declares and deletes a exchange successfully", func() {
		By("declaring exchange")
		Expect(k8sClient.Create(ctx, exchange, &client.CreateOptions{})).To(Succeed())
		var exchangeInfo *rabbithole.DetailedExchangeInfo
		Eventually(func() error {
			var err error
			exchangeInfo, err = rabbitClient.GetExchange(exchange.Spec.Vhost, exchange.Name)
			return err
		}, 10, 2).Should(BeNil())

		Expect(*exchangeInfo).To(MatchFields(IgnoreExtras, Fields{
			"Name":       Equal(exchange.Spec.Name),
			"Vhost":      Equal(exchange.Spec.Vhost),
			"Type":       Equal(exchange.Spec.Type),
			"AutoDelete": BeFalse(),
			"Durable":    BeTrue(),
		}))
		Expect(exchangeInfo.Arguments).To(HaveKeyWithValue("alternate-exchange", "system-test"))

		By("updating status condition 'Ready'")
		updatedExchange := topologyv1alpha1.Exchange{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: exchange.Name, Namespace: exchange.Namespace}, &updatedExchange)).To(Succeed())

		Expect(updatedExchange.Status.Conditions).To(HaveLen(1))
		readyCondition := updatedExchange.Status.Conditions[0]
		Expect(string(readyCondition.Type)).To(Equal("Ready"))
		Expect(readyCondition.Status).To(Equal(corev1.ConditionTrue))
		Expect(readyCondition.Reason).To(Equal("SuccessfulCreateOrUpdate"))
		Expect(readyCondition.LastTransitionTime).NotTo(Equal(metav1.Time{}))

		By("deleting exchange")
		Expect(k8sClient.Delete(ctx, exchange)).To(Succeed())
		var err error
		Eventually(func() error {
			_, err = rabbitClient.GetExchange(exchange.Spec.Vhost, exchange.Name)
			return err
		}, 5).Should(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("Object Not Found"))
	})
})
