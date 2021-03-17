package v1alpha1

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
			Source:          "test",
			Destination:     "test",
			DestinationType: "queue",
			RabbitmqClusterReference: RabbitmqClusterReference{
				Name:      "some-cluster",
				Namespace: "default",
			},
		},
	}

	It("does not allow updates on source", func() {
		newBinding := Binding{
			ObjectMeta: metav1.ObjectMeta{
				Name: "update-binding",
			},
			Spec: BindingSpec{
				Source:          "updated-source",
				Destination:     "test",
				DestinationType: "queue",
				RabbitmqClusterReference: RabbitmqClusterReference{
					Name:      "some-cluster",
					Namespace: "default",
				},
			},
		}
		Expect(apierrors.IsForbidden(newBinding.ValidateUpdate(&oldBinding))).To(BeTrue())
	})

	It("does not allow updates on destination", func() {
		newBinding := Binding{
			ObjectMeta: metav1.ObjectMeta{
				Name: "update-binding",
			},
			Spec: BindingSpec{
				Source:          "test",
				Destination:     "updated-des",
				DestinationType: "queue",
				RabbitmqClusterReference: RabbitmqClusterReference{
					Name:      "some-cluster",
					Namespace: "default",
				},
			},
		}
		Expect(apierrors.IsForbidden(newBinding.ValidateUpdate(&oldBinding))).To(BeTrue())
	})

	It("does not allow updates on destination type", func() {
		newBinding := Binding{
			ObjectMeta: metav1.ObjectMeta{
				Name: "update-binding",
			},
			Spec: BindingSpec{
				Source:          "test",
				Destination:     "test",
				DestinationType: "exchange",
				RabbitmqClusterReference: RabbitmqClusterReference{
					Name:      "some-cluster",
					Namespace: "default",
				},
			},
		}
		Expect(apierrors.IsForbidden(newBinding.ValidateUpdate(&oldBinding))).To(BeTrue())
	})

	It("does not allow updates on routing key", func() {
		newBinding := Binding{
			ObjectMeta: metav1.ObjectMeta{
				Name: "update-binding",
			},
			Spec: BindingSpec{
				Source:          "test",
				Destination:     "test",
				DestinationType: "queue",
				RoutingKey:      "not-allowed",
				RabbitmqClusterReference: RabbitmqClusterReference{
					Name:      "some-cluster",
					Namespace: "default",
				},
			},
		}
		Expect(apierrors.IsForbidden(newBinding.ValidateUpdate(&oldBinding))).To(BeTrue())
	})

	It("does not allow updates on binding arguments", func() {
		newBinding := Binding{
			ObjectMeta: metav1.ObjectMeta{
				Name: "update-binding",
			},
			Spec: BindingSpec{
				Source:          "test",
				Destination:     "test",
				DestinationType: "queue",
				Arguments: &runtime.RawExtension{
					Raw: []byte(`{"new":"new-value"}`),
				},
				RabbitmqClusterReference: RabbitmqClusterReference{
					Name:      "some-cluster",
					Namespace: "default",
				},
			},
		}
		Expect(apierrors.IsForbidden(newBinding.ValidateUpdate(&oldBinding))).To(BeTrue())
	})

	It("does not allow updates on RabbitmqClusterReference", func() {
		newBinding := Binding{
			ObjectMeta: metav1.ObjectMeta{
				Name: "update-binding",
			},
			Spec: BindingSpec{
				Source:          "test",
				Destination:     "test",
				DestinationType: "queue",
				RabbitmqClusterReference: RabbitmqClusterReference{
					Name:      "new-cluster",
					Namespace: "default",
				},
			},
		}
		Expect(apierrors.IsForbidden(newBinding.ValidateUpdate(&oldBinding))).To(BeTrue())
	})
})
