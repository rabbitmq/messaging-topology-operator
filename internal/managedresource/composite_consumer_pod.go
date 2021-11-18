package managedresource

import (
	"fmt"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"strconv"
)

const (
	compositeConsumerPodSuffix = "-exchange"
)

type CompositeConsumerPodBuilder struct {
	*Builder
	podSpec   corev1.PodSpec
	partition string
	replica   int
}

func (builder *Builder) CompositeConsumerPod(podSpec corev1.PodSpec, partition string, replica int) *CompositeConsumerPodBuilder {
	return &CompositeConsumerPodBuilder{builder, podSpec, partition, replica}
}

func (builder *CompositeConsumerPodBuilder) Build() (client.Object, error) {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s-%d", builder.ObjectOwner.GetName(), builder.partition, builder.replica),
			Namespace: builder.ObjectOwner.GetNamespace(),
			Labels: map[string]string{
				"rabbitmq.com/super-stream-partition": builder.partition,
				"rabbitmq.com/composite-consumer-replica": strconv.Itoa(builder.replica),
			},
		},
	}, nil
}

func (builder *CompositeConsumerPodBuilder) Update(object client.Object) error {
	pod := object.(*corev1.Pod)
	pod.Spec = builder.podSpec

	if err := controllerutil.SetControllerReference(builder.ObjectOwner, object, builder.Scheme); err != nil {
		return fmt.Errorf("failed setting controller reference: %w", err)
	}

	return nil
}

func (builder *CompositeConsumerPodBuilder) ResourceType() string { return "CompositeConsumerPod" }
