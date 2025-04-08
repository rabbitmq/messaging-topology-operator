package internal_test

import (
	. "github.com/onsi/ginkgo/v2"
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

	It("sets default queue type according to vhost.spec.defaultQueueType", func() {
		v.Spec.DefaultQueueType = "stream"
		settings := internal.GenerateVhostSettings(v)
		Expect(settings.DefaultQueueType).To(Equal("stream"))
	})
})

var _ = Describe("GenerateVhostLimits", func() {
	var limits topology.VhostLimits
	var connections, queues int32

	When("limits are configured", func() {
		BeforeEach(func() {
			connections = 1312
			queues = 137
			limits = topology.VhostLimits{
				Connections: &connections,
				Queues:      &queues,
			}
		})

		It("generates VhostLimitValues", func() {
			VhostLimitsValues := internal.GenerateVhostLimits(&limits)
			Expect(len(VhostLimitsValues)).To(Equal(2))
			Expect(VhostLimitsValues["max-connections"]).To(Equal(int(connections)))
			Expect(VhostLimitsValues["max-queues"]).To(Equal(int(queues)))
		})
	})

	When("some imits are configured", func() {
		BeforeEach(func() {
			queues = 137
			limits = topology.VhostLimits{
				Connections: &connections,
				Queues:      nil,
			}
		})

		It("generates VhostLimitValues only for those limits", func() {
			VhostLimitsValues := internal.GenerateVhostLimits(&limits)
			Expect(len(VhostLimitsValues)).To(Equal(1))
			Expect(VhostLimitsValues["max-connections"]).To(Equal(int(connections)))
		})
	})

	When("no limits are configured", func() {
		It("generates VhostLimitValues", func() {
			VhostLimitsValues := internal.GenerateVhostLimits(nil)
			Expect(VhostLimitsValues).To(BeEmpty())
		})
	})

	When("special values are configured", func() {
		BeforeEach(func() {
			connections = 0
			queues = -1
			limits = topology.VhostLimits{
				Connections: &connections,
				Queues:      &queues,
			}
		})

		It("generates VhostLimitValues", func() {
			VhostLimitsValues := internal.GenerateVhostLimits(&limits)
			Expect(len(VhostLimitsValues)).To(Equal(2))
			Expect(VhostLimitsValues["max-connections"]).To(Equal(int(connections)))
			Expect(VhostLimitsValues["max-queues"]).To(Equal(int(queues)))
		})
	})
})
