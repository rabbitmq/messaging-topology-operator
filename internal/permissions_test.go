package internal_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	topology "github.com/rabbitmq/messaging-topology-operator/api/v1beta1"
	. "github.com/rabbitmq/messaging-topology-operator/internal"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("GeneratePermissions", func() {
	var p *topology.Permission

	BeforeEach(func() {
		p = &topology.Permission{
			ObjectMeta: metav1.ObjectMeta{
				Name: "user-permissions",
			},
			Spec: topology.PermissionSpec{
				User:  "a-user",
				Vhost: "/new-vhost",
			},
		}
	})

	It("sets 'Configure' correctly", func() {
		p.Spec.Permissions.Configure = ".*"
		rmqPermissions := GeneratePermissions(p)
		Expect(rmqPermissions.Configure).To(Equal(".*"))
	})

	It("sets 'Write' correctly", func() {
		p.Spec.Permissions.Write = ".~"
		rmqPermissions := GeneratePermissions(p)
		Expect(rmqPermissions.Write).To(Equal(".~"))
	})

	It("sets 'Read' correctly", func() {
		p.Spec.Permissions.Read = "^$"
		rmqPermissions := GeneratePermissions(p)
		Expect(rmqPermissions.Read).To(Equal("^$"))
	})
})
