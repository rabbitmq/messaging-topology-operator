package v1alpha1

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	topologyv1beta1 "github.com/rabbitmq/messaging-topology-operator/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("superstream webhook", func() {
	var superstream = SuperStream{}
	BeforeEach(func() {
		superstream = SuperStream{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test",
			},
			Spec: SuperStreamSpec{
				Name:        "test",
				Partitions:  4,
				RoutingKeys: []string{"a1", "b2", "f17"},
				RabbitmqClusterReference: topologyv1beta1.RabbitmqClusterReference{
					Name: "a-cluster",
				},
			},
		}
	})

	Context("ValidateCreate", func() {
		It("does not allow both spec.rabbitmqClusterReference.name and spec.rabbitmqClusterReference.connectionSecret be configured", func() {
			notAllowed := superstream.DeepCopy()
			notAllowed.Spec.RabbitmqClusterReference.ConnectionSecret = &corev1.LocalObjectReference{Name: "some-secret"}
			_, err := notAllowed.ValidateCreate()
			Expect(apierrors.IsForbidden(err)).To(BeTrue())
		})

		It("spec.rabbitmqClusterReference.name and spec.rabbitmqClusterReference.connectionSecret cannot both be empty", func() {
			notAllowed := superstream.DeepCopy()
			notAllowed.Spec.RabbitmqClusterReference.Name = ""
			notAllowed.Spec.RabbitmqClusterReference.ConnectionSecret = nil
			_, err := notAllowed.ValidateCreate()
			Expect(apierrors.IsForbidden(err)).To(BeTrue())
		})
	})

	Context("ValidateUpdate", func() {
		It("does not allow updates on superstream name", func() {
			newSuperStream := superstream.DeepCopy()
			newSuperStream.Spec.Name = "new-name"
			_, err := newSuperStream.ValidateUpdate(&superstream)
			Expect(apierrors.IsForbidden(err)).To(BeTrue())
		})

		It("does not allow updates on superstream vhost", func() {
			newSuperStream := superstream.DeepCopy()
			newSuperStream.Spec.Vhost = "new-vhost"
			_, err := newSuperStream.ValidateUpdate(&superstream)
			Expect(apierrors.IsForbidden(err)).To(BeTrue())
		})

		It("does not allow updates on RabbitmqClusterReference", func() {
			newSuperStream := superstream.DeepCopy()
			newSuperStream.Spec.RabbitmqClusterReference = topologyv1beta1.RabbitmqClusterReference{
				Name: "new-cluster",
			}
			_, err := newSuperStream.ValidateUpdate(&superstream)
			Expect(apierrors.IsForbidden(err)).To(BeTrue())
		})

		It("does not allow updates on rabbitmqClusterReference.connectionSecret", func() {
			newSuperStream := superstream.DeepCopy()
			newSuperStream.Spec.RabbitmqClusterReference = topologyv1beta1.RabbitmqClusterReference{ConnectionSecret: &corev1.LocalObjectReference{Name: "a-secret"}}
			_, err := newSuperStream.ValidateUpdate(&superstream)
			Expect(apierrors.IsForbidden(err)).To(BeTrue())
		})

		It("does not allow updates on superstream.spec.routingKeys", func() {
			newSuperStream := superstream.DeepCopy()
			newSuperStream.Spec.RoutingKeys = []string{"a1", "d6"}
			_, err := newSuperStream.ValidateUpdate(&superstream)
			Expect(apierrors.IsForbidden(err)).To(BeTrue())
		})

		It("if the superstream previously had routing keys and the update only appends, the update succeeds", func() {
			newSuperStream := superstream.DeepCopy()
			newSuperStream.Spec.RoutingKeys = []string{"a1", "b2", "f17", "z66"}
			_, err := newSuperStream.ValidateUpdate(&superstream)
			Expect(err).NotTo(HaveOccurred())
		})

		It("if the superstream previously had no routing keys but now does, the update fails", func() {
			superstream.Spec.RoutingKeys = nil
			newSuperStream := superstream.DeepCopy()
			newSuperStream.Spec.RoutingKeys = []string{"a1", "b2", "f17"}
			_, err := newSuperStream.ValidateUpdate(&superstream)
			Expect(apierrors.IsForbidden(err)).To(BeTrue())
		})

		It("allows superstream.spec.partitions to be increased", func() {
			newSuperStream := superstream.DeepCopy()
			newSuperStream.Spec.Partitions = 1000
			_, err := newSuperStream.ValidateUpdate(&superstream)
			Expect(err).NotTo(HaveOccurred())
		})

		It("does not allow superstream.spec.partitions to be decreased", func() {
			newSuperStream := superstream.DeepCopy()
			newSuperStream.Spec.Partitions = 1
			_, err := newSuperStream.ValidateUpdate(&superstream)
			Expect(apierrors.IsForbidden(err)).To(BeTrue())
		})
	})
})
