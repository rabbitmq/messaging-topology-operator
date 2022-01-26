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
	vhost           string
	routingKey      string
	partitionIndex  int
	rabbitmqCluster *topology.RabbitmqClusterReference
}

func (builder *Builder) SuperStreamPartition(partitionIndex int, routingKey, vhost string, rabbitmqCluster *topology.RabbitmqClusterReference) *SuperStreamPartitionBuilder {
	return &SuperStreamPartitionBuilder{builder, vhost, routingKey, partitionIndex, rabbitmqCluster}
}

func partitionSuffix(partitionIndex int) string {
	return fmt.Sprintf("-partition-%d", partitionIndex)
}

func (builder *SuperStreamPartitionBuilder) Build() (client.Object, error) {
	return &topology.Queue{
		ObjectMeta: metav1.ObjectMeta{
			Name:      builder.GenerateChildResourceName(partitionSuffix(builder.partitionIndex)),
			Namespace: builder.ObjectOwner.GetNamespace(),
			Labels: map[string]string{
				AnnotationSuperStream:           builder.ObjectOwner.GetName(),
				AnnotationSuperStreamRoutingKey: builder.routingKey,
			},
		},
	}, nil
}

func (builder *SuperStreamPartitionBuilder) Update(object client.Object) error {
	partition := object.(*topology.Queue)
	partition.Spec.Name = RoutingKeyToPartitionName(builder.ObjectOwner.GetName(), builder.routingKey)
	partition.Spec.Durable = true
	partition.Spec.Type = "stream"
	partition.Spec.Vhost = builder.vhost
	partition.Spec.RabbitmqClusterReference = *builder.rabbitmqCluster

	if err := controllerutil.SetControllerReference(builder.ObjectOwner, object, builder.Scheme); err != nil {
		return fmt.Errorf("failed setting controller reference: %w", err)
	}

	return nil
}

func (builder *SuperStreamPartitionBuilder) ResourceType() string { return "Partition" }

func RoutingKeyToPartitionName(parentObjectName, routingKey string) string {
	return fmt.Sprintf("%s-%s", parentObjectName, routingKey)
}

func PartitionNameToRoutingKey(parentObjectName, partitionName string) string {
	return strings.TrimPrefix(partitionName, fmt.Sprintf("%s-", parentObjectName))
}
