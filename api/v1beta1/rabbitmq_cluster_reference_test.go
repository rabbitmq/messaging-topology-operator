package v1beta1

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
)

var _ = Describe("RabbitmqClusterReference Matches", func() {
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

	When("name is different", func() {
		It("returns false", func() {
			new := reference.DeepCopy()
			new.Name = "new-name"
			Expect(reference.Matches(new)).To(BeFalse())
		})
	})

	When("namespace is different", func() {
		It("returns false", func() {
			new := reference.DeepCopy()
			new.Namespace = "new-ns"
			Expect(reference.Matches(new)).To(BeFalse())
		})
	})

	When("connectionSecret.name is different", func() {
		It("returns false", func() {
			new := reference.DeepCopy()
			new.ConnectionSecret.Name = "new-secret-name"
			Expect(reference.Matches(new)).To(BeFalse())
		})
	})

	When("connectionSecret is removed", func() {
		It("returns false", func() {
			new := reference.DeepCopy()
			new.ConnectionSecret = nil
			Expect(reference.Matches(new)).To(BeFalse())
		})
	})

	When("connectionSecret is added", func() {
		It("returns false", func() {
			reference.ConnectionSecret = nil
			new := reference.DeepCopy()
			new.ConnectionSecret = &v1.LocalObjectReference{
				Name: "a-secret-name",
			}
			Expect(reference.Matches(new)).To(BeFalse())
		})
	})

	When("RabbitmqClusterReference stayed the same", func() {
		It("returns true", func() {
			new := reference.DeepCopy()
			Expect(reference.Matches(new)).To(BeTrue())
		})
	})
})
