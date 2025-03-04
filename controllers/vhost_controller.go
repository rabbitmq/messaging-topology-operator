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

func (r *VhostReconciler) DeclareFunc(_ context.Context, client rabbitmqclient.Client, obj topology.TopologyResource) error {
	vhost := obj.(*topology.Vhost)
	return validateResponse(client.PutVhost(vhost.Spec.Name, *internal.GenerateVhostSettings(vhost)))
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
