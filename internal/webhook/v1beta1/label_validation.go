package v1beta1

import (
	"context"
	"fmt"

	topology "github.com/rabbitmq/messaging-topology-operator/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// validateSecretLabel checks that the Secret referenced by ref carries the topology-operator label.
// The cached client c only holds labeled Secrets; a NotFound response means the Secret either
// does not exist or exists without the label. The apiReader disambiguates for a clear error message.
func validateSecretLabel(ctx context.Context, c client.Client, apiReader client.Reader, ref *corev1.LocalObjectReference, ns string) error {
	if ref == nil || c == nil {
		return nil
	}
	secret := &corev1.Secret{}
	if err := c.Get(ctx, types.NamespacedName{Name: ref.Name, Namespace: ns}, secret); err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
		// Cache miss: check whether the Secret exists without the label.
		if apiErr := apiReader.Get(ctx, types.NamespacedName{Name: ref.Name, Namespace: ns}, secret); apiErr == nil {
			return fmt.Errorf("secret %q must have label %s=%s to be used by the Topology Operator",
				ref.Name, topology.TopologyOperatorLabel, topology.TopologyOperatorLabelValue)
		}
		return fmt.Errorf("secret %q not found in namespace %q", ref.Name, ns)
	}
	// Cache hit guarantees the label is present.
	return nil
}
