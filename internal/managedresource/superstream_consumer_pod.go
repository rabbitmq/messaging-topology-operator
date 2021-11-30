package managedresource

import (
	"fmt"
	"github.com/mitchellh/hashstructure/v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"strconv"
)

const (
	AnnotationSuperStream          = "rabbitmq.com/super-stream"
	AnnotationSuperStreamPartition = "rabbitmq.com/super-stream-partition"
	AnnotationConsumerPodSpecHash = "rabbitmq.com/consumer-pod-spec-hash"
)

type SuperStreamConsumerPodBuilder struct {
	*Builder
	podSpec         corev1.PodSpec
	superStreamName string
	partition       string
}

func (builder *Builder) SuperStreamConsumerPod(podSpec corev1.PodSpec, superStreamName, partition string) *SuperStreamConsumerPodBuilder {
	return &SuperStreamConsumerPodBuilder{builder, podSpec, superStreamName, partition}
}

func (builder *SuperStreamConsumerPodBuilder) Build() (client.Object, error) {
	podSpecHash, err := hashstructure.Hash(builder.podSpec, hashstructure.FormatV2, nil)
	if err != nil {
		return nil, err
	}
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: fmt.Sprintf("%s-%s-", builder.ObjectOwner.GetName(), builder.partition),
			Namespace:    builder.ObjectOwner.GetNamespace(),
			Labels: map[string]string{
				AnnotationSuperStream:          builder.superStreamName,
				AnnotationSuperStreamPartition: builder.partition,
				AnnotationConsumerPodSpecHash: strconv.FormatUint(podSpecHash, 16),
			},
		},
		Spec: builder.podSpec,
	}, nil
}

func (builder *SuperStreamConsumerPodBuilder) Update(object client.Object) error {
	if err := controllerutil.SetControllerReference(builder.ObjectOwner, object, builder.Scheme); err != nil {
		return fmt.Errorf("failed setting controller reference: %w", err)
	}

	return nil
}

func (builder *SuperStreamConsumerPodBuilder) ResourceType() string { return "SuperStreamConsumerPod" }
