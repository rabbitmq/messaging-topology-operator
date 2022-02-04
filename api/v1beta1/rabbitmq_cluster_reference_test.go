package v1beta1

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
)

var _ = Describe("RabbitmqClusterReference HasChange", func() {
	var reference *RabbitmqClusterReference

	BeforeEach(func() {
		reference = &RabbitmqClusterReference{
			Name:      "a-name",
			Namespace: "a-ns",
			ConnectionSecret: &v1.LocalObjectReference{
				Name: "a-secret-name",
			},
		}
	})

	When("name was changed", func() {
		It("returns true", func() {
			new := reference.DeepCopy()
			new.Name = "new-name"
			Expect(reference.HasChange(new)).To(BeTrue())
		})
	})

	When("namespace was changed", func() {
		It("returns true", func() {
			new := reference.DeepCopy()
			new.Namespace = "new-ns"
			Expect(reference.HasChange(new)).To(BeTrue())
		})
	})

	When("connectionSecret.name was changed", func() {
		It("returns true", func() {
			new := reference.DeepCopy()
			new.ConnectionSecret.Name = "new-secret-name"
			Expect(reference.HasChange(new)).To(BeTrue())
		})
	})

	When("connectionSecret was removed", func() {
		It("returns true", func() {
			new := reference.DeepCopy()
			new.ConnectionSecret = nil
			Expect(reference.HasChange(new)).To(BeTrue())
		})
	})

	When("connectionSecret was added", func() {
		It("returns true", func() {
			reference.ConnectionSecret = nil
			new := reference.DeepCopy()
			new.ConnectionSecret = &v1.LocalObjectReference{
				Name: "a-secret-name",
			}
			Expect(reference.HasChange(new)).To(BeTrue())
		})
	})

	When("RabbitmqClusterReference stayed the same", func() {
		It("returns false", func() {
			new := reference.DeepCopy()
			Expect(reference.HasChange(new)).To(BeFalse())
		})
	})
})
