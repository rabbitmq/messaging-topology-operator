package v1beta1

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("superStreamConsumer webhook", func() {
	var superStreamConsumer = SuperStreamConsumer{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test",
		},
		Spec: SuperStreamConsumerSpec{
			SuperStreamReference: SuperStreamReference{
				Name: "a-super-stream",
			},
		},
	}

	It("does not allow updates on SuperStreamReference", func() {
		newSuperStreamConsumer := superStreamConsumer.DeepCopy()
		newSuperStreamConsumer.Spec.SuperStreamReference = SuperStreamReference{
			Name: "new-stream",
		}
		Expect(apierrors.IsForbidden(newSuperStreamConsumer.ValidateUpdate(&superStreamConsumer))).To(BeTrue())
	})

	It("allows updates on ConsumerPodSpec", func() {
		newSuperStreamConsumer := superStreamConsumer.DeepCopy()
		newSuperStreamConsumer.Spec.ConsumerPodSpec = SuperStreamConsumerPodSpec{
			Default: &corev1.PodSpec{
				RestartPolicy: corev1.RestartPolicyOnFailure,
			},
		}
		Expect(newSuperStreamConsumer.ValidateUpdate(&superStreamConsumer)).To(Succeed())
	})
})
