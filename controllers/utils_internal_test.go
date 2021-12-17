package controllers

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Utils", func() {
	Context("Service DNS address", func() {
		var someService *v1.Service
		BeforeEach(func() {
			someService = &v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "myservice",
					Namespace: "mynamespace",
				},
			}
		})

		It("generates an address with cluster domain suffix", func() {
			someDomain := ".example.com"
			serviceDnsWithDomain := serviceDNSAddress(someService, someDomain)
			Expect(serviceDnsWithDomain).To(Equal("myservice.mynamespace.svc.example.com"))
		})

		When("the domain suffix is not present", func() {
			It("generates the shortname", func() {
				serviceDnsWithDomain := serviceDNSAddress(someService, "")
				Expect(serviceDnsWithDomain).To(Equal("myservice.mynamespace.svc"))
			})
		})
	})
})
