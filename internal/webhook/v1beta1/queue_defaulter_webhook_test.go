package v1beta1

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	rabbitmqcomv1beta1 "github.com/rabbitmq/messaging-topology-operator/api/v1beta1"
	"k8s.io/utils/ptr"
)

var _ = Describe("Queue Defaulter Webhook", func() {
	var (
		obj       *rabbitmqcomv1beta1.Queue
		defaulter QueueCustomDefaulter
	)

	BeforeEach(func() {
		obj = &rabbitmqcomv1beta1.Queue{}
		defaulter = QueueCustomDefaulter{}
	})

	Context("when defaulting a new Queue", func() {
		It("sets durable to true when not specified", func() {
			Expect(obj.Spec.Durable).To(BeNil())
			Expect(defaulter.Default(ctx, obj)).To(Succeed())
			Expect(ptr.Deref(obj.Spec.Durable, false)).To(BeTrue())
		})

		It("does not change durable when already true", func() {
			obj.Spec.Durable = new(true)
			Expect(defaulter.Default(ctx, obj)).To(Succeed())
			Expect(ptr.Deref(obj.Spec.Durable, false)).To(BeTrue())
		})

		It("does not change durable when explicitly set to false", func() {
			obj.Spec.Durable = new(false)
			Expect(defaulter.Default(ctx, obj)).To(Succeed())
			Expect(ptr.Deref(obj.Spec.Durable, true)).To(BeFalse())
		})

		It("leaves autoDelete as false when not specified", func() {
			Expect(obj.Spec.AutoDelete).To(BeFalse())
			Expect(defaulter.Default(ctx, obj)).To(Succeed())
			Expect(obj.Spec.AutoDelete).To(BeFalse())
		})

		It("does not override autoDelete when explicitly set to true", func() {
			obj.Spec.AutoDelete = true
			Expect(defaulter.Default(ctx, obj)).To(Succeed())
			Expect(obj.Spec.AutoDelete).To(BeTrue())
		})
	})
})
