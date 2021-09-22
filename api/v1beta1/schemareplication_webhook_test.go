package v1beta1

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("schema-replication webhook", func() {
	var replication = SchemaReplication{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-replication",
		},
		Spec: SchemaReplicationSpec{
			UpstreamSecret: &corev1.LocalObjectReference{
				Name: "a-secret",
			},
			Endpoints: "abc.rmq.com:1234",
			RabbitmqClusterReference: RabbitmqClusterReference{
				Name: "a-cluster",
			},
		},
	}

	It("does not allow updates on RabbitmqClusterReference", func() {
		updated := replication.DeepCopy()
		updated.Spec.RabbitmqClusterReference = RabbitmqClusterReference{
			Name: "different-cluster",
		}
		Expect(apierrors.IsForbidden(updated.ValidateUpdate(&replication))).To(BeTrue())
	})

	It("allows updates on spec.upstreamSecret", func() {
		updated := replication.DeepCopy()
		updated.Spec.UpstreamSecret = &corev1.LocalObjectReference{
			Name: "a-different-secret",
		}
		Expect(updated.ValidateUpdate(&replication)).To(Succeed())
	})

	It("allows updates on spec.endpoints", func() {
		updated := replication.DeepCopy()
		updated.Spec.Endpoints = "abc.new-rmq:1111"
		Expect(updated.ValidateUpdate(&replication)).To(Succeed())
	})
})
