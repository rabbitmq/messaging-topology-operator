package controllers

import (
	"context"
	"errors"

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
	logger.Info("generated vhost settings", "vhost", vhost.Spec.Name, "settings", settings)
	err := validateResponse(client.PutVhost(vhost.Spec.Name, *settings))
	if err != nil {
		return err
	}

	vhostLimits := internal.GenerateVhostLimits(vhost.Spec.VhostLimits)
	logger.Info("generated vhost limits", "vhost", vhost.Spec.Name, "limits", vhostLimits)
	if len(vhostLimits) > 0 {
		err = validateResponse(client.PutVhostLimits(vhost.Spec.Name, vhostLimits))
	}
	return err
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
