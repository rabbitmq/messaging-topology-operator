package internal_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	topology "github.com/rabbitmq/messaging-topology-operator/api/v1beta2"
	"github.com/rabbitmq/messaging-topology-operator/internal"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("GenerateVhostSettings", func() {
	var v *topology.Vhost

	BeforeEach(func() {
		v = &topology.Vhost{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test",
			},
			Spec: topology.VhostSpec{
				Tracing: true,
			},
		}
	})

	It("sets 'tracing' according to vhost.spec", func() {
		settings := internal.GenerateVhostSettings(v)
		Expect(settings.Tracing).To(BeTrue())
	})
})
