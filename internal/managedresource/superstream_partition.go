package managedresource

import (
	"fmt"
	topology "github.com/rabbitmq/messaging-topology-operator/api/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"strings"
)

type SuperStreamPartitionBuilder struct {
	*Builder
	routingKey      string
	rabbitmqCluster *topology.RabbitmqClusterReference
}

func (builder *Builder) SuperStreamPartition(routingKey string, rabbitmqCluster *topology.RabbitmqClusterReference) *SuperStreamPartitionBuilder {
	return &SuperStreamPartitionBuilder{builder, routingKey, rabbitmqCluster}
}

func partitionSuffix(routingKey string) string {
	return fmt.Sprintf("-partition-%s", routingKey)
}

func (builder *SuperStreamPartitionBuilder) Build() (client.Object, error) {
	return &topology.Queue{
		ObjectMeta: metav1.ObjectMeta{
			Name:      builder.GenerateChildResourceName(partitionSuffix(builder.routingKey)),
			Namespace: builder.ObjectOwner.GetNamespace(),
		},
	}, nil
}

func (builder *SuperStreamPartitionBuilder) Update(object client.Object) error {
	partition := object.(*topology.Queue)
	partition.Spec.Name = RoutingKeyToPartitionName(builder.ObjectOwner.GetName(), builder.routingKey)
	partition.Spec.Durable = true
	partition.Spec.Type = "stream"
	partition.Spec.RabbitmqClusterReference = *builder.rabbitmqCluster

	if err := controllerutil.SetControllerReference(builder.ObjectOwner, object, builder.Scheme); err != nil {
		return fmt.Errorf("failed setting controller reference: %w", err)
	}

	return nil
}

func (builder *SuperStreamPartitionBuilder) ResourceType() string { return "Partition" }

func RoutingKeyToPartitionName(parentObjectName, routingKey string) string {
	return fmt.Sprintf("%s.%s", parentObjectName, routingKey)
}

func PartitionNameToRoutingKey(parentObjectName, partitionName string) string {
	return strings.Replace(partitionName, fmt.Sprintf("%s.", parentObjectName), "", 1)
}
