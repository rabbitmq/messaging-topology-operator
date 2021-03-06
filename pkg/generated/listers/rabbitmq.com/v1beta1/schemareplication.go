/*
RabbitMQ Messaging Topology Kubernetes Operator
Copyright 2021 VMware, Inc.

This product is licensed to you under the Mozilla Public License 2.0 license (the "License").  You may not use this product except in compliance with the Mozilla 2.0 License.

This product may include a number of subcomponents with separate copyright notices and license terms. Your use of these subcomponents is subject to the terms and conditions of the subcomponent's license, as noted in the LICENSE file.
*/

// Code generated by lister-gen. DO NOT EDIT.

package v1beta1

import (
	v1beta1 "github.com/rabbitmq/messaging-topology-operator/api/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
)

// SchemaReplicationLister helps list SchemaReplications.
// All objects returned here must be treated as read-only.
type SchemaReplicationLister interface {
	// List lists all SchemaReplications in the indexer.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*v1beta1.SchemaReplication, err error)
	// SchemaReplications returns an object that can list and get SchemaReplications.
	SchemaReplications(namespace string) SchemaReplicationNamespaceLister
	SchemaReplicationListerExpansion
}

// schemaReplicationLister implements the SchemaReplicationLister interface.
type schemaReplicationLister struct {
	indexer cache.Indexer
}

// NewSchemaReplicationLister returns a new SchemaReplicationLister.
func NewSchemaReplicationLister(indexer cache.Indexer) SchemaReplicationLister {
	return &schemaReplicationLister{indexer: indexer}
}

// List lists all SchemaReplications in the indexer.
func (s *schemaReplicationLister) List(selector labels.Selector) (ret []*v1beta1.SchemaReplication, err error) {
	err = cache.ListAll(s.indexer, selector, func(m interface{}) {
		ret = append(ret, m.(*v1beta1.SchemaReplication))
	})
	return ret, err
}

// SchemaReplications returns an object that can list and get SchemaReplications.
func (s *schemaReplicationLister) SchemaReplications(namespace string) SchemaReplicationNamespaceLister {
	return schemaReplicationNamespaceLister{indexer: s.indexer, namespace: namespace}
}

// SchemaReplicationNamespaceLister helps list and get SchemaReplications.
// All objects returned here must be treated as read-only.
type SchemaReplicationNamespaceLister interface {
	// List lists all SchemaReplications in the indexer for a given namespace.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*v1beta1.SchemaReplication, err error)
	// Get retrieves the SchemaReplication from the indexer for a given namespace and name.
	// Objects returned here must be treated as read-only.
	Get(name string) (*v1beta1.SchemaReplication, error)
	SchemaReplicationNamespaceListerExpansion
}

// schemaReplicationNamespaceLister implements the SchemaReplicationNamespaceLister
// interface.
type schemaReplicationNamespaceLister struct {
	indexer   cache.Indexer
	namespace string
}

// List lists all SchemaReplications in the indexer for a given namespace.
func (s schemaReplicationNamespaceLister) List(selector labels.Selector) (ret []*v1beta1.SchemaReplication, err error) {
	err = cache.ListAllByNamespace(s.indexer, s.namespace, selector, func(m interface{}) {
		ret = append(ret, m.(*v1beta1.SchemaReplication))
	})
	return ret, err
}

// Get retrieves the SchemaReplication from the indexer for a given namespace and name.
func (s schemaReplicationNamespaceLister) Get(name string) (*v1beta1.SchemaReplication, error) {
	obj, exists, err := s.indexer.GetByKey(s.namespace + "/" + name)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.NewNotFound(v1beta1.Resource("schemareplication"), name)
	}
	return obj.(*v1beta1.SchemaReplication), nil
}
