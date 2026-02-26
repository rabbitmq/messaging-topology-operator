package managedresource_test

import (
	"testing"

	topology "github.com/rabbitmq/messaging-topology-operator/api/v1beta1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestResource(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "ManagedResource Suite")
}

var testRabbitmqClusterReference = &topology.RabbitmqClusterReference{
	Name:      "test-rabbit",
	Namespace: "example-namespace",
}
