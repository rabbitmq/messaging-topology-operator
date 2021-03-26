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

// UserListsGetter has a method to return a UserListInterface.
// A group's client should implement this interface.
type UserListsGetter interface {
	UserLists(namespace string) UserListInterface
}

// UserListInterface has methods to work with UserList resources.
type UserListInterface interface {
	Create(ctx context.Context, userList *v1alpha2.UserList, opts v1.CreateOptions) (*v1alpha2.UserList, error)
	Update(ctx context.Context, userList *v1alpha2.UserList, opts v1.UpdateOptions) (*v1alpha2.UserList, error)
	Delete(ctx context.Context, name string, opts v1.DeleteOptions) error
	DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error
	Get(ctx context.Context, name string, opts v1.GetOptions) (*v1alpha2.UserList, error)
	List(ctx context.Context, opts v1.ListOptions) (*v1alpha2.UserListList, error)
	Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error)
	Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1alpha2.UserList, err error)
	UserListExpansion
}

// userLists implements UserListInterface
type userLists struct {
	client rest.Interface
	ns     string
}

// newUserLists returns a UserLists
func newUserLists(c *RabbitmqV1alpha2Client, namespace string) *userLists {
	return &userLists{
		client: c.RESTClient(),
		ns:     namespace,
	}
}

// Get takes name of the userList, and returns the corresponding userList object, and an error if there is any.
func (c *userLists) Get(ctx context.Context, name string, options v1.GetOptions) (result *v1alpha2.UserList, err error) {
	result = &v1alpha2.UserList{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("userlists").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do(ctx).
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of UserLists that match those selectors.
func (c *userLists) List(ctx context.Context, opts v1.ListOptions) (result *v1alpha2.UserListList, err error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	result = &v1alpha2.UserListList{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("userlists").
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Do(ctx).
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested userLists.
func (c *userLists) Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	opts.Watch = true
	return c.client.Get().
		Namespace(c.ns).
		Resource("userlists").
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Watch(ctx)
}

// Create takes the representation of a userList and creates it.  Returns the server's representation of the userList, and an error, if there is any.
func (c *userLists) Create(ctx context.Context, userList *v1alpha2.UserList, opts v1.CreateOptions) (result *v1alpha2.UserList, err error) {
	result = &v1alpha2.UserList{}
	err = c.client.Post().
		Namespace(c.ns).
		Resource("userlists").
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(userList).
		Do(ctx).
		Into(result)
	return
}

// Update takes the representation of a userList and updates it. Returns the server's representation of the userList, and an error, if there is any.
func (c *userLists) Update(ctx context.Context, userList *v1alpha2.UserList, opts v1.UpdateOptions) (result *v1alpha2.UserList, err error) {
	result = &v1alpha2.UserList{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("userlists").
		Name(userList.Name).
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(userList).
		Do(ctx).
		Into(result)
	return
}

// Delete takes name of the userList and deletes it. Returns an error if one occurs.
func (c *userLists) Delete(ctx context.Context, name string, opts v1.DeleteOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("userlists").
		Name(name).
		Body(&opts).
		Do(ctx).
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *userLists) DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error {
	var timeout time.Duration
	if listOpts.TimeoutSeconds != nil {
		timeout = time.Duration(*listOpts.TimeoutSeconds) * time.Second
	}
	return c.client.Delete().
		Namespace(c.ns).
		Resource("userlists").
		VersionedParams(&listOpts, scheme.ParameterCodec).
		Timeout(timeout).
		Body(&opts).
		Do(ctx).
		Error()
}

// Patch applies the patch and returns the patched userList.
func (c *userLists) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1alpha2.UserList, err error) {
	result = &v1alpha2.UserList{}
	err = c.client.Patch(pt).
		Namespace(c.ns).
		Resource("userlists").
		Name(name).
		SubResource(subresources...).
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(data).
		Do(ctx).
		Into(result)
	return
}
