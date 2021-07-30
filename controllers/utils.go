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
	"io/ioutil"
	"net/http"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"strings"

	corev1 "k8s.io/api/core/v1"
)

// TODO: check possible status code response from RabbitMQ
// validate status code above 300 might not be all failure case
func validateResponse(res *http.Response, err error) error {
	if err != nil {
		return err
	}
	if res == nil {
		return errors.New("failed to validate empty HTTP response")
	}

	if res.StatusCode >= http.StatusMultipleChoices {
		body, _ := ioutil.ReadAll(res.Body)
		res.Body.Close()
		return fmt.Errorf("request failed with status code %d and body %q", res.StatusCode, body)
	}
	return nil
}

// return a custom error if status code is 404
// used in all controllers when deleting objects from rabbitmq server
var NotFound = errors.New("not found")

func validateResponseForDeletion(res *http.Response, err error) error {
	if res != nil && res.StatusCode == http.StatusNotFound {
		return NotFound
	}
	return validateResponse(res, err)
}

// serviceDNSAddress returns the cluster-local DNS entry associated
// with the provided Service
func serviceDNSAddress(svc *corev1.Service) string {
	// NOTE: this does not use the `cluster.local` suffix, because that is not
	// uniform across clusters. See the `clusterDomain` KubeletConfiguration
	// value for how this can be changed for a cluster.
	return fmt.Sprintf("%s.%s.svc", svc.Name, svc.Namespace)
}

func addFinalizerIfNeeded(ctx context.Context, client client.Client, obj client.Object) error {
	finalizer := deletionFinalizer(obj.GetObjectKind().GroupVersionKind().Kind)
	if obj.GetDeletionTimestamp().IsZero() && !controllerutil.ContainsFinalizer(obj, finalizer) {
		controllerutil.AddFinalizer(obj, finalizer)
		if err := client.Update(ctx, obj); err != nil {
			return fmt.Errorf("failed to add deletionFinalizer: %w", err)
		}
	}
	return nil
}

func removeFinalizer(ctx context.Context, client client.Client, obj client.Object) error {
	finalizer := deletionFinalizer(obj.GetObjectKind().GroupVersionKind().Kind)
	controllerutil.RemoveFinalizer(obj, finalizer)
	if err := client.Update(ctx, obj); err != nil {
		return fmt.Errorf("failed to delete finalizer: %w", err)
	}
	return nil
}

// deletionFinalizer returns generated deletion finalizer
// finalizers follow the format of deletion.finalizers.kind-plural-form.rabbitmq.com
// for example: deletion.finalizers.bindings.rabbitmq.com and deletion.finalizers.policies.rabbitmq.com
func deletionFinalizer(kind string) string {
	var plural string
	if kind == "Policy" {
		plural = "policies"
	} else {
		plural = strings.ToLower(kind) + "s"
	}
	return fmt.Sprintf("deletion.finalizers.%s.%s", plural, "rabbitmq.com")
}
