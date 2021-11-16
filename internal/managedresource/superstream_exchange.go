package managedresource

import (
	"fmt"
	topology "github.com/rabbitmq/messaging-topology-operator/api/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	superStreamExchangeSuffix = "-exchange"
)

type SuperStreamExchangeBuilder struct {
	*Builder
	rabbitmqCluster *topology.RabbitmqClusterReference
}

func (builder *Builder) SuperStreamExchange(rabbitmqCluster *topology.RabbitmqClusterReference) *SuperStreamExchangeBuilder {
	return &SuperStreamExchangeBuilder{builder, rabbitmqCluster}
}

func (builder *SuperStreamExchangeBuilder) Build() (client.Object, error) {
	return &topology.Exchange{
		ObjectMeta: metav1.ObjectMeta{
			Name:      builder.GenerateChildResourceName(superStreamExchangeSuffix),
			Namespace: builder.ObjectOwner.GetNamespace(),
		},
	}, nil
}

func (builder *SuperStreamExchangeBuilder) Update(object client.Object) error {
	exchange := object.(*topology.Exchange)
	exchange.Spec.Durable = true
	exchange.Spec.Name = builder.ObjectOwner.GetName()
	exchange.Spec.RabbitmqClusterReference = *builder.rabbitmqCluster

	if err := controllerutil.SetControllerReference(builder.ObjectOwner, object, builder.Scheme); err != nil {
		return fmt.Errorf("failed setting controller reference: %w", err)
	}

	return nil
}

func (builder *SuperStreamExchangeBuilder) ResourceType() string { return "Exchange" }
