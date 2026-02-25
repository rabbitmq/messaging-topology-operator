package controller

import (
	"context"
	"errors"

	rabbithole "github.com/michaelklishin/rabbit-hole/v3"
	"github.com/rabbitmq/messaging-topology-operator/internal"
	"github.com/rabbitmq/messaging-topology-operator/rabbitmqclient"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	topology "github.com/rabbitmq/messaging-topology-operator/api/v1beta1"
)

// +kubebuilder:rbac:groups=rabbitmq.com,resources=vhosts,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rabbitmq.com,resources=vhosts/finalizers,verbs=update
// +kubebuilder:rbac:groups=rabbitmq.com,resources=vhosts/status,verbs=get;update;patch

type VhostReconciler struct {
	client.Client
}

func (r *VhostReconciler) DeclareFunc(ctx context.Context, client rabbitmqclient.Client, obj topology.TopologyResource) error {
	logger := ctrl.LoggerFrom(ctx)
	vhost := obj.(*topology.Vhost)
	settings := internal.GenerateVhostSettings(vhost)
	logger.V(1).Info("generated vhost settings", "vhost", vhost.Spec.Name, "settings", settings)
	err := validateResponse(client.PutVhost(vhost.Spec.Name, *settings))
	if err != nil {
		return err
	}

	newVhostLimits := internal.GenerateVhostLimits(vhost.Spec.VhostLimits)
	logger.V(1).Info("getting existing vhost limits", "vhost", vhost.Spec.Name)
	existingVhostLimits, err := r.getVhostLimits(client, vhost.Spec.Name)
	if err != nil {
		return err
	}
	limitsToDelete := r.vhostLimitsToDelete(existingVhostLimits, newVhostLimits)
	if len(limitsToDelete) > 0 {
		logger.Info("Deleting outdated vhost limits", "vhost", vhost.Spec.Name, "limits", limitsToDelete)
		err = validateResponseForDeletion(client.DeleteVhostLimits(vhost.Spec.Name, limitsToDelete))
		if err != nil && !errors.Is(err, NotFound) {
			return err
		}
	}
	if len(newVhostLimits) > 0 {
		logger.Info("creating new vhost limits", "vhost", vhost.Spec.Name, "limits", newVhostLimits)
		return validateResponse(client.PutVhostLimits(vhost.Spec.Name, newVhostLimits))
	}
	return nil
}

// DeleteFunc deletes vhost from server
// if server responds with '404' Not Found, it logs and does not requeue on error
func (r *VhostReconciler) DeleteFunc(ctx context.Context, client rabbitmqclient.Client, obj topology.TopologyResource) error {
	logger := ctrl.LoggerFrom(ctx)
	vhost := obj.(*topology.Vhost)
	if shouldSkipDeletion(ctx, vhost.Spec.DeletionPolicy, vhost.Spec.Name) {
		return nil
	}
	err := validateResponseForDeletion(client.DeleteVhost(vhost.Spec.Name))
	if errors.Is(err, NotFound) {
		logger.Info("cannot find vhost in rabbitmq server; already deleted", "vhost", vhost.Spec.Name)
	} else if err != nil {
		return err
	}
	return nil
}

func (r *VhostReconciler) vhostLimitsToDelete(existingVhostLimits, newVhostLimits rabbithole.VhostLimitsValues) (limitsToDelete rabbithole.VhostLimits) {
	vhostLimitKeys := []string{"max-connections", "max-queues"}
	for _, limit := range vhostLimitKeys {
		_, oldExists := existingVhostLimits[limit]
		_, newExists := newVhostLimits[limit]
		if oldExists && !newExists {
			limitsToDelete = append(limitsToDelete, limit)
		}
	}
	return limitsToDelete
}

func (r *VhostReconciler) getVhostLimits(client rabbitmqclient.Client, vhost string) (rabbithole.VhostLimitsValues, error) {
	vhostLimitsInfo, err := client.GetVhostLimits(vhost)
	if errors.Is(err, error(rabbithole404)) {
		return rabbithole.VhostLimitsValues{}, nil
	} else if err != nil {
		return rabbithole.VhostLimitsValues{}, err
	}
	if len(vhostLimitsInfo) == 0 {
		return rabbithole.VhostLimitsValues{}, nil
	}
	return vhostLimitsInfo[0].Value, nil
}
