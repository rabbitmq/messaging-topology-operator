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

// OperatorPolicyLister helps list OperatorPolicies.
// All objects returned here must be treated as read-only.
type OperatorPolicyLister interface {
	// List lists all OperatorPolicies in the indexer.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*v1beta1.OperatorPolicy, err error)
	// OperatorPolicies returns an object that can list and get OperatorPolicies.
	OperatorPolicies(namespace string) OperatorPolicyNamespaceLister
	OperatorPolicyListerExpansion
}

// operatorPolicyLister implements the OperatorPolicyLister interface.
type operatorPolicyLister struct {
	indexer cache.Indexer
}

// NewOperatorPolicyLister returns a new OperatorPolicyLister.
func NewOperatorPolicyLister(indexer cache.Indexer) OperatorPolicyLister {
	return &operatorPolicyLister{indexer: indexer}
}

// List lists all OperatorPolicies in the indexer.
func (s *operatorPolicyLister) List(selector labels.Selector) (ret []*v1beta1.OperatorPolicy, err error) {
	err = cache.ListAll(s.indexer, selector, func(m interface{}) {
		ret = append(ret, m.(*v1beta1.OperatorPolicy))
	})
	return ret, err
}

// OperatorPolicies returns an object that can list and get OperatorPolicies.
func (s *operatorPolicyLister) OperatorPolicies(namespace string) OperatorPolicyNamespaceLister {
	return operatorPolicyNamespaceLister{indexer: s.indexer, namespace: namespace}
}

// OperatorPolicyNamespaceLister helps list and get OperatorPolicies.
// All objects returned here must be treated as read-only.
type OperatorPolicyNamespaceLister interface {
	// List lists all OperatorPolicies in the indexer for a given namespace.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*v1beta1.OperatorPolicy, err error)
	// Get retrieves the OperatorPolicy from the indexer for a given namespace and name.
	// Objects returned here must be treated as read-only.
	Get(name string) (*v1beta1.OperatorPolicy, error)
	OperatorPolicyNamespaceListerExpansion
}

// operatorPolicyNamespaceLister implements the OperatorPolicyNamespaceLister
// interface.
type operatorPolicyNamespaceLister struct {
	indexer   cache.Indexer
	namespace string
}

// List lists all OperatorPolicies in the indexer for a given namespace.
func (s operatorPolicyNamespaceLister) List(selector labels.Selector) (ret []*v1beta1.OperatorPolicy, err error) {
	err = cache.ListAllByNamespace(s.indexer, s.namespace, selector, func(m interface{}) {
		ret = append(ret, m.(*v1beta1.OperatorPolicy))
	})
	return ret, err
}

// Get retrieves the OperatorPolicy from the indexer for a given namespace and name.
func (s operatorPolicyNamespaceLister) Get(name string) (*v1beta1.OperatorPolicy, error) {
	obj, exists, err := s.indexer.GetByKey(s.namespace + "/" + name)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.NewNotFound(v1beta1.Resource("operatorpolicy"), name)
	}
	return obj.(*v1beta1.OperatorPolicy), nil
}
