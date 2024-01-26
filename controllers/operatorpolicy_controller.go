/*
RabbitMQ Messaging Topology Kubernetes Operator
Copyright 2021 VMware, Inc.

This product is licensed to you under the Mozilla Public License 2.0 license (the "License").  You may not use this product except in compliance with the Mozilla 2.0 License.

This product may include a number of subcomponents with separate copyright notices and license terms. Your use of these subcomponents is subject to the terms and conditions of the subcomponent's license, as noted in the LICENSE file.
*/

package controllers

import (
	"context"
	"errors"
	"fmt"

	topology "github.com/rabbitmq/messaging-topology-operator/api/v1beta1"
	"github.com/rabbitmq/messaging-topology-operator/internal"
	"github.com/rabbitmq/messaging-topology-operator/rabbitmqclient"
	ctrl "sigs.k8s.io/controller-runtime"
)

// +kubebuilder:rbac:groups=rabbitmq.com,resources=operatorpolicies,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rabbitmq.com,resources=operatorpolicies/finalizers,verbs=update
// +kubebuilder:rbac:groups=rabbitmq.com,resources=operatorpolicies/status,verbs=get;update;patch

type OperatorPolicyReconciler struct{}

// DeclareFunc creates or updates a given operator policy using rabbithole client.PutOperatorPolicy
func (r *OperatorPolicyReconciler) DeclareFunc(_ context.Context, client rabbitmqclient.Client, obj topology.TopologyResource) error {
	policy := obj.(*topology.OperatorPolicy)
	generateOperatorPolicy, err := internal.GenerateOperatorPolicy(policy)
	if err != nil {
		return fmt.Errorf("failed to generate OperatorPolicy: %w", err)
	}
	return validateResponse(client.PutOperatorPolicy(policy.Spec.Vhost, policy.Spec.Name, *generateOperatorPolicy))
}

// DeleteFunc deletes operator policy from rabbitmq server
// if server responds with '404' Not Found, it logs and does not requeue on error
func (r *OperatorPolicyReconciler) DeleteFunc(ctx context.Context, client rabbitmqclient.Client, obj topology.TopologyResource) error {
	logger := ctrl.LoggerFrom(ctx)
	policy := obj.(*topology.OperatorPolicy)
	err := validateResponseForDeletion(client.DeleteOperatorPolicy(policy.Spec.Vhost, policy.Spec.Name))
	if errors.Is(err, NotFound) {
		logger.Info("cannot find operator policy in rabbitmq server; already deleted", "operatorpolicy", policy.Spec.Name)
	} else if err != nil {
		return err
	}
	return nil
}
