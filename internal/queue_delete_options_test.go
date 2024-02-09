package internal_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	topology "github.com/rabbitmq/messaging-topology-operator/api/v1beta1"
	"github.com/rabbitmq/messaging-topology-operator/internal"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("GenerateQueueDeleteOptionsQuorum", func() {
	var q *topology.Queue

	BeforeEach(func() {
		q = &topology.Queue{
			ObjectMeta: metav1.ObjectMeta{
				Name: "a-queue",
			},
			Spec: topology.QueueSpec{
				Type:           "quorum",
				AutoDelete:     false,
				Durable:        true,
				DeleteIfEmpty:  true,
				DeleteIfUnused: false,
			},
		}
	})

	It("sets QueueDeleteOptions.IfEmpty to false because we handle a quorum queue", func() {
		options, err := internal.GenerateQueueDeleteOptions(q)
		Expect(err).NotTo(HaveOccurred())
		Expect(options.IfEmpty).To(BeFalse())
	})

	It("sets QueueDeleteOptions.IfUnused to false because we handle a quorum queue", func() {
		options, err := internal.GenerateQueueDeleteOptions(q)
		Expect(err).NotTo(HaveOccurred())
		Expect(options.IfUnused).To(BeFalse())
	})

})

var _ = Describe("GenerateQueueDeleteOptionsClassic", func() {
	var q *topology.Queue

	BeforeEach(func() {
		q = &topology.Queue{
			ObjectMeta: metav1.ObjectMeta{
				Name: "a-queue",
			},
			Spec: topology.QueueSpec{
				Type:           "classic",
				AutoDelete:     false,
				Durable:        true,
				DeleteIfEmpty:  true,
				DeleteIfUnused: false,
			},
		}
	})

	It("sets QueueDeleteOptions.IfEmpty according to queue.spec", func() {
		options, err := internal.GenerateQueueDeleteOptions(q)
		Expect(err).NotTo(HaveOccurred())
		Expect(options.IfEmpty).To(BeTrue())
	})

	It("sets QueueDeleteOptions.IfUnused according to queue.spec", func() {
		options, err := internal.GenerateQueueDeleteOptions(q)
		Expect(err).NotTo(HaveOccurred())
		Expect(options.IfUnused).To(BeFalse())
	})

})
