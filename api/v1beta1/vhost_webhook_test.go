package v1beta1

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("vhost webhook", func() {

	var vhost = Vhost{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-vhost",
		},
		Spec: VhostSpec{
			Name:    "test",
			Tracing: false,
			RabbitmqClusterReference: RabbitmqClusterReference{
				Name: "a-cluster",
			},
		},
	}

	It("does not allow updates on vhost name", func() {
		newVhost := vhost.DeepCopy()
		newVhost.Spec.Name = "new-name"
		Expect(apierrors.IsForbidden(newVhost.ValidateUpdate(&vhost))).To(BeTrue())
	})

	It("does not allow updates on RabbitmqClusterReference", func() {
		newVhost := vhost.DeepCopy()
		newVhost.Spec.RabbitmqClusterReference = RabbitmqClusterReference{
			Name: "new-cluster",
		}
		Expect(apierrors.IsForbidden(newVhost.ValidateUpdate(&vhost))).To(BeTrue())
	})

	It("allows updates on vhost.spec.tracing", func() {
		newVhost := vhost.DeepCopy()
		newVhost.Spec.Tracing = true
		Expect(newVhost.ValidateUpdate(&vhost)).To(Succeed())
	})

	It("allows updates on vhost.spec.tags", func() {
		newVhost := vhost.DeepCopy()
		newVhost.Spec.Tags = []string{"new-tag"}
		Expect(newVhost.ValidateUpdate(&vhost)).To(Succeed())
	})
})
