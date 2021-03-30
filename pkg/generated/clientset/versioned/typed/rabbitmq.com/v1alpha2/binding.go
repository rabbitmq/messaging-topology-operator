/*
RabbitMQ Messaging Topology Kubernetes Operator
Copyright 2021 VMware, Inc.

This product is licensed to you under the Mozilla Public License 2.0 license (the "License").  You may not use this product except in compliance with the Mozilla 2.0 License.

This product may include a number of subcomponents with separate copyright notices and license terms. Your use of these subcomponents is subject to the terms and conditions of the subcomponent's license, as noted in the LICENSE file.
*/

// Code generated by client-gen. DO NOT EDIT.

package v1alpha2

import (
	"context"
	"time"

	v1alpha2 "github.com/rabbitmq/messaging-topology-operator/api/v1alpha2"
	scheme "github.com/rabbitmq/messaging-topology-operator/pkg/generated/clientset/versioned/scheme"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	rest "k8s.io/client-go/rest"
)

// BindingsGetter has a method to return a BindingInterface.
// A group's client should implement this interface.
type BindingsGetter interface {
	Bindings(namespace string) BindingInterface
}

// BindingInterface has methods to work with Binding resources.
type BindingInterface interface {
	Create(ctx context.Context, binding *v1alpha2.Binding, opts v1.CreateOptions) (*v1alpha2.Binding, error)
	Update(ctx context.Context, binding *v1alpha2.Binding, opts v1.UpdateOptions) (*v1alpha2.Binding, error)
	UpdateStatus(ctx context.Context, binding *v1alpha2.Binding, opts v1.UpdateOptions) (*v1alpha2.Binding, error)
	Delete(ctx context.Context, name string, opts v1.DeleteOptions) error
	DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error
	Get(ctx context.Context, name string, opts v1.GetOptions) (*v1alpha2.Binding, error)
	List(ctx context.Context, opts v1.ListOptions) (*v1alpha2.BindingList, error)
	Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error)
	Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1alpha2.Binding, err error)
	BindingExpansion
}

// bindings implements BindingInterface
type bindings struct {
	client rest.Interface
	ns     string
}

// newBindings returns a Bindings
func newBindings(c *RabbitmqV1alpha2Client, namespace string) *bindings {
	return &bindings{
		client: c.RESTClient(),
		ns:     namespace,
	}
}

// Get takes name of the binding, and returns the corresponding binding object, and an error if there is any.
func (c *bindings) Get(ctx context.Context, name string, options v1.GetOptions) (result *v1alpha2.Binding, err error) {
	result = &v1alpha2.Binding{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("bindings").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do(ctx).
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of Bindings that match those selectors.
func (c *bindings) List(ctx context.Context, opts v1.ListOptions) (result *v1alpha2.BindingList, err error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	result = &v1alpha2.BindingList{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("bindings").
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Do(ctx).
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested bindings.
func (c *bindings) Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	opts.Watch = true
	return c.client.Get().
		Namespace(c.ns).
		Resource("bindings").
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Watch(ctx)
}

// Create takes the representation of a binding and creates it.  Returns the server's representation of the binding, and an error, if there is any.
func (c *bindings) Create(ctx context.Context, binding *v1alpha2.Binding, opts v1.CreateOptions) (result *v1alpha2.Binding, err error) {
	result = &v1alpha2.Binding{}
	err = c.client.Post().
		Namespace(c.ns).
		Resource("bindings").
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(binding).
		Do(ctx).
		Into(result)
	return
}

// Update takes the representation of a binding and updates it. Returns the server's representation of the binding, and an error, if there is any.
func (c *bindings) Update(ctx context.Context, binding *v1alpha2.Binding, opts v1.UpdateOptions) (result *v1alpha2.Binding, err error) {
	result = &v1alpha2.Binding{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("bindings").
		Name(binding.Name).
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(binding).
		Do(ctx).
		Into(result)
	return
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
func (c *bindings) UpdateStatus(ctx context.Context, binding *v1alpha2.Binding, opts v1.UpdateOptions) (result *v1alpha2.Binding, err error) {
	result = &v1alpha2.Binding{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("bindings").
		Name(binding.Name).
		SubResource("status").
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(binding).
		Do(ctx).
		Into(result)
	return
}

// Delete takes name of the binding and deletes it. Returns an error if one occurs.
func (c *bindings) Delete(ctx context.Context, name string, opts v1.DeleteOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("bindings").
		Name(name).
		Body(&opts).
		Do(ctx).
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *bindings) DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error {
	var timeout time.Duration
	if listOpts.TimeoutSeconds != nil {
		timeout = time.Duration(*listOpts.TimeoutSeconds) * time.Second
	}
	return c.client.Delete().
		Namespace(c.ns).
		Resource("bindings").
		VersionedParams(&listOpts, scheme.ParameterCodec).
		Timeout(timeout).
		Body(&opts).
		Do(ctx).
		Error()
}

// Patch applies the patch and returns the patched binding.
func (c *bindings) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1alpha2.Binding, err error) {
	result = &v1alpha2.Binding{}
	err = c.client.Patch(pt).
		Namespace(c.ns).
		Resource("bindings").
		Name(name).
		SubResource(subresources...).
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(data).
		Do(ctx).
		Into(result)
	return
}
