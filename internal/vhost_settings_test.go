package internal_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	topology "github.com/rabbitmq/messaging-topology-operator/api/v1beta1"
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
				Tags:    []string{"tag1", "tag2", "multi_dc_replication"},
			},
		}
	})

	It("sets 'tracing' according to vhost.spec", func() {
		settings := internal.GenerateVhostSettings(v)
		Expect(settings.Tracing).To(BeTrue())
	})

	It("sets 'tags' according to vhost.spec.tags", func() {
		settings := internal.GenerateVhostSettings(v)
		Expect(settings.Tags).To(ConsistOf("tag1", "tag2", "multi_dc_replication"))
	})
})
