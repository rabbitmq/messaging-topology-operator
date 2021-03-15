package v1alpha1

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Conditions", func() {
	Context("Ready", func() {
		It("returns 'Ready' condition set to true", func() {
			c := Ready()
			Expect(string(c.Type)).To(Equal("Ready"))
			Expect(c.Status).To(Equal(corev1.ConditionTrue))
			Expect(c.Reason).To(Equal("SuccessfulCreateOrUpdate"))
			Expect(c.LastTransitionTime).NotTo(Equal(metav1.Time{})) // has been set; not empty
		})

	})

	Context("NotReady", func() {
		It("returns 'Ready' condition set to false", func() {
			c := NotReady("fail to declare queue")
			Expect(string(c.Type)).To(Equal("Ready"))
			Expect(c.Status).To(Equal(corev1.ConditionFalse))
			Expect(c.Reason).To(Equal("FailedCreateOrUpdate"))
			Expect(c.Message).To(Equal("fail to declare queue"))
			Expect(c.LastTransitionTime).NotTo(Equal(metav1.Time{})) // has been set; not empty
		})
	})
})
