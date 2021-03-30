/*
RabbitMQ Messaging Topology Kubernetes Operator
Copyright 2021 VMware, Inc.

This product is licensed to you under the Mozilla Public License 2.0 license (the "License").  You may not use this product except in compliance with the Mozilla 2.0 License.

This product may include a number of subcomponents with separate copyright notices and license terms. Your use of these subcomponents is subject to the terms and conditions of the subcomponent's license, as noted in the LICENSE file.
*/

// Code generated by informer-gen. DO NOT EDIT.

package v1alpha2

import (
	internalinterfaces "github.com/rabbitmq/messaging-topology-operator/pkg/generated/informers/externalversions/internalinterfaces"
)

// Interface provides access to all the informers in this group version.
type Interface interface {
	// Bindings returns a BindingInformer.
	Bindings() BindingInformer
	// Exchanges returns a ExchangeInformer.
	Exchanges() ExchangeInformer
	// Policies returns a PolicyInformer.
	Policies() PolicyInformer
	// Queues returns a QueueInformer.
	Queues() QueueInformer
	// Users returns a UserInformer.
	Users() UserInformer
	// Vhosts returns a VhostInformer.
	Vhosts() VhostInformer
}

type version struct {
	factory          internalinterfaces.SharedInformerFactory
	namespace        string
	tweakListOptions internalinterfaces.TweakListOptionsFunc
}

// New returns a new Interface.
func New(f internalinterfaces.SharedInformerFactory, namespace string, tweakListOptions internalinterfaces.TweakListOptionsFunc) Interface {
	return &version{factory: f, namespace: namespace, tweakListOptions: tweakListOptions}
}

// Bindings returns a BindingInformer.
func (v *version) Bindings() BindingInformer {
	return &bindingInformer{factory: v.factory, namespace: v.namespace, tweakListOptions: v.tweakListOptions}
}

// Exchanges returns a ExchangeInformer.
func (v *version) Exchanges() ExchangeInformer {
	return &exchangeInformer{factory: v.factory, namespace: v.namespace, tweakListOptions: v.tweakListOptions}
}

// Policies returns a PolicyInformer.
func (v *version) Policies() PolicyInformer {
	return &policyInformer{factory: v.factory, namespace: v.namespace, tweakListOptions: v.tweakListOptions}
}

// Queues returns a QueueInformer.
func (v *version) Queues() QueueInformer {
	return &queueInformer{factory: v.factory, namespace: v.namespace, tweakListOptions: v.tweakListOptions}
}

// Users returns a UserInformer.
func (v *version) Users() UserInformer {
	return &userInformer{factory: v.factory, namespace: v.namespace, tweakListOptions: v.tweakListOptions}
}

// Vhosts returns a VhostInformer.
func (v *version) Vhosts() VhostInformer {
	return &vhostInformer{factory: v.factory, namespace: v.namespace, tweakListOptions: v.tweakListOptions}
}
