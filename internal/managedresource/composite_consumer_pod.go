package managedresource

import (
	"fmt"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	compositeConsumerPodSuffix     = "-exchange"
	AnnotationSuperStream          = "rabbitmq.com/super-stream"
	AnnotationSuperStreamPartition = "rabbitmq.com/super-stream-partition"
)

type CompositeConsumerPodBuilder struct {
	*Builder
	podSpec         corev1.PodSpec
	superStreamName string
	partition       string
}

func (builder *Builder) CompositeConsumerPod(podSpec corev1.PodSpec, superStreamName, partition string) *CompositeConsumerPodBuilder {
	return &CompositeConsumerPodBuilder{builder, podSpec, superStreamName, partition}
}

func (builder *CompositeConsumerPodBuilder) Build() (client.Object, error) {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s", builder.ObjectOwner.GetName(), builder.partition),
			Namespace: builder.ObjectOwner.GetNamespace(),
			Labels: map[string]string{
				AnnotationSuperStream:          builder.superStreamName,
				AnnotationSuperStreamPartition: builder.partition,
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
