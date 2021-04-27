package v1beta1

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

var _ = Describe("policy webhook", func() {
	var policy = Policy{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test",
		},
		Spec: PolicySpec{
			Name:     "test",
			Vhost:    "/test",
			Pattern:  "a-pattern",
			ApplyTo:  "all",
			Priority: 0,
			RabbitmqClusterReference: RabbitmqClusterReference{
				Name: "a-cluster",
			},
		},
	}

	It("does not allow updates on policy name", func() {
		newPolicy := policy.DeepCopy()
		newPolicy.Spec.Name = "new-name"
		Expect(apierrors.IsForbidden(newPolicy.ValidateUpdate(&policy))).To(BeTrue())
	})

	It("does not allow updates on vhost", func() {
		newPolicy := policy.DeepCopy()
		newPolicy.Spec.Vhost = "new-vhost"
		Expect(apierrors.IsForbidden(newPolicy.ValidateUpdate(&policy))).To(BeTrue())
	})

	It("does not allow updates on RabbitmqClusterReference", func() {
		newPolicy := policy.DeepCopy()
		newPolicy.Spec.RabbitmqClusterReference = RabbitmqClusterReference{
			Name: "new-cluster",
		}
		Expect(apierrors.IsForbidden(newPolicy.ValidateUpdate(&policy))).To(BeTrue())
	})

	It("allows updates on policy.spec.pattern", func() {
		newPolicy := policy.DeepCopy()
		newPolicy.Spec.Pattern = "new-pattern"
		Expect(newPolicy.ValidateUpdate(&policy)).To(Succeed())
	})

	It("allows updates on policy.spec.applyTo", func() {
		newPolicy := policy.DeepCopy()
		newPolicy.Spec.ApplyTo = "queues"
		Expect(newPolicy.ValidateUpdate(&policy)).To(Succeed())
	})

	It("allows updates on policy.spec.priority", func() {
		newPolicy := policy.DeepCopy()
		newPolicy.Spec.Priority = 1000
		Expect(newPolicy.ValidateUpdate(&policy)).To(Succeed())
	})

	It("allows updates on policy.spec.definition", func() {
		newPolicy := policy.DeepCopy()
		newPolicy.Spec.Definition = &runtime.RawExtension{Raw: []byte(`{"key":"new-definition-value"}`)}
		Expect(newPolicy.ValidateUpdate(&policy)).To(Succeed())
	})
})
