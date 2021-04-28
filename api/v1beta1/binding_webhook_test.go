package v1beta1

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

var _ = Describe("Binding webhook", func() {

	var oldBinding = Binding{
		ObjectMeta: metav1.ObjectMeta{
			Name: "update-binding",
		},
		Spec: BindingSpec{
			Vhost:           "/test",
			Source:          "test",
			Destination:     "test",
			DestinationType: "queue",
			RabbitmqClusterReference: RabbitmqClusterReference{
				Name: "some-cluster",
			},
		},
	}

	It("does not allow updates on vhost", func() {
		newBinding := oldBinding.DeepCopy()
		newBinding.Spec.Vhost = "/new-vhost"
		Expect(apierrors.IsForbidden(newBinding.ValidateUpdate(&oldBinding))).To(BeTrue())
	})

	It("does not allow updates on RabbitmqClusterReference", func() {
		newBinding := oldBinding.DeepCopy()
		newBinding.Spec.RabbitmqClusterReference = RabbitmqClusterReference{
			Name: "new-cluster",
		}
		Expect(apierrors.IsForbidden(newBinding.ValidateUpdate(&oldBinding))).To(BeTrue())
	})

	It("does not allow updates on source", func() {
		newBinding := oldBinding.DeepCopy()
		newBinding.Spec.Source = "updated-source"
		Expect(apierrors.IsInvalid(newBinding.ValidateUpdate(&oldBinding))).To(BeTrue())
	})

	It("does not allow updates on destination", func() {
		newBinding := oldBinding.DeepCopy()
		newBinding.Spec.Destination = "updated-des"
		Expect(apierrors.IsInvalid(newBinding.ValidateUpdate(&oldBinding))).To(BeTrue())
	})

	It("does not allow updates on destination type", func() {
		newBinding := oldBinding.DeepCopy()
		newBinding.Spec.DestinationType = "exchange"
		Expect(apierrors.IsInvalid(newBinding.ValidateUpdate(&oldBinding))).To(BeTrue())
	})

	It("does not allow updates on routing key", func() {
		newBinding := oldBinding.DeepCopy()
		newBinding.Spec.RoutingKey = "not-allowed"
		Expect(apierrors.IsInvalid(newBinding.ValidateUpdate(&oldBinding))).To(BeTrue())
	})

	It("does not allow updates on binding arguments", func() {
		newBinding := oldBinding.DeepCopy()
		newBinding.Spec.Arguments = &runtime.RawExtension{Raw: []byte(`{"new":"new-value"}`)}
		Expect(apierrors.IsInvalid(newBinding.ValidateUpdate(&oldBinding))).To(BeTrue())
	})
})
