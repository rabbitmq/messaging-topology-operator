/*
RabbitMQ Messaging Topology Kubernetes Operator
Copyright 2021 VMware, Inc.

This product is licensed to you under the Mozilla Public License 2.0 license (the "License").  You may not use this product except in compliance with the Mozilla 2.0 License.

This product may include a number of subcomponents with separate copyright notices and license terms. Your use of these subcomponents is subject to the terms and conditions of the subcomponent's license, as noted in the LICENSE file.
*/

package controllers

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"

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
	return fmt.Sprintf("%s.%s.svc.cluster.local", svc.Name, svc.Namespace)
}
