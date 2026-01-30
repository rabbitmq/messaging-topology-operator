package internal_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	topology "github.com/rabbitmq/messaging-topology-operator/api/v1beta1"
	. "github.com/rabbitmq/messaging-topology-operator/internal"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

var _ = Describe("GenerateOperatorPolicy", func() {
	var p *topology.OperatorPolicy

	BeforeEach(func() {
		p = &topology.OperatorPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name: "new-operatorPolicy",
			},
			Spec: topology.OperatorPolicySpec{
				Name:       "new-p",
				Vhost:      "/new-vhost",
				ApplyTo:    "queues",
				Pattern:    "queue-name",
				Priority:   5,
				Definition: &runtime.RawExtension{Raw: []byte(`{"key":"value"}`)},
			},
		}
	})

	It("sets operatorPolicy name according to operatorPolicySpec", func() {
		generated, err := GenerateOperatorPolicy(p)
		Expect(err).NotTo(HaveOccurred())
		Expect(generated.Name).To(Equal("new-p"))
	})

	It("sets operatorPolicy vhost according to operatorPolicySpec", func() {
		generated, err := GenerateOperatorPolicy(p)
		Expect(err).NotTo(HaveOccurred())
		Expect(generated.Vhost).To(Equal("/new-vhost"))
	})

	It("sets 'ApplyTo' according to operatorPolicySpec", func() {
		generated, err := GenerateOperatorPolicy(p)
		Expect(err).NotTo(HaveOccurred())
		Expect(generated.ApplyTo).To(Equal("queues"))
	})

	It("sets 'priority' according to operatorPolicySpec", func() {
		generated, err := GenerateOperatorPolicy(p)
		Expect(err).NotTo(HaveOccurred())
		Expect(generated.Priority).To(Equal(5))
	})

	It("sets 'pattern' according to operatorPolicySpec", func() {
		generated, err := GenerateOperatorPolicy(p)
		Expect(err).NotTo(HaveOccurred())
		Expect(generated.Pattern).To(Equal("queue-name"))
	})

	It("sets definition according to operatorPolicySpec", func() {
		generated, err := GenerateOperatorPolicy(p)
		Expect(err).NotTo(HaveOccurred())
		Expect(generated.Definition).Should(HaveLen(1))
		Expect(generated.Definition).Should(HaveKeyWithValue("key", "value"))
	})
})
