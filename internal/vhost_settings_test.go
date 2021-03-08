package internal_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	topologyv1alpha1 "github.com/rabbitmq/messaging-topology-operator/api/v1alpha1"
	"github.com/rabbitmq/messaging-topology-operator/internal"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("GenerateVhostSettings", func() {
	var v *topologyv1alpha1.Vhost

	BeforeEach(func() {
		v = &topologyv1alpha1.Vhost{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test",
			},
			Spec: topologyv1alpha1.VhostSpec{
				Tracing: true,
			},
		}
	})

	It("sets 'tracing' according to vhost.spec", func() {
		settings := internal.GenerateVhostSettings(v)
		Expect(settings.Tracing).To(BeTrue())
	})
})
