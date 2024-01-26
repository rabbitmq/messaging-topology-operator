package controllers

import (
	"context"
	topology "github.com/rabbitmq/messaging-topology-operator/api/v1beta1"
	"github.com/rabbitmq/messaging-topology-operator/rabbitmqclient"
)

var _ ReconcileFunc = &BindingReconciler{}
var _ ReconcileFunc = &ExchangeReconciler{}
var _ ReconcileFunc = &FederationReconciler{}
var _ ReconcileFunc = &PermissionReconciler{}
var _ ReconcileFunc = &PolicyReconciler{}
var _ ReconcileFunc = &OperatorPolicyReconciler{}
var _ ReconcileFunc = &QueueReconciler{}
var _ ReconcileFunc = &SchemaReplicationReconciler{}
var _ ReconcileFunc = &ShovelReconciler{}
var _ ReconcileFunc = &UserReconciler{}
var _ ReconcileFunc = &VhostReconciler{}

type ReconcileFunc interface {
	DeclareFunc(ctx context.Context, client rabbitmqclient.Client, resource topology.TopologyResource) error
	DeleteFunc(ctx context.Context, client rabbitmqclient.Client, resource topology.TopologyResource) error
}
