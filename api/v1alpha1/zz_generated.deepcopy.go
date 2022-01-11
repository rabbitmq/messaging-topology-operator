//go:build !ignore_autogenerated
// +build !ignore_autogenerated

/*
RabbitMQ Messaging Topology Kubernetes Operator
Copyright 2021 VMware, Inc.

This product is licensed to you under the Mozilla Public License 2.0 license (the "License").  You may not use this product except in compliance with the Mozilla 2.0 License.

This product may include a number of subcomponents with separate copyright notices and license terms. Your use of these subcomponents is subject to the terms and conditions of the subcomponent's license, as noted in the LICENSE file.
*/

// Code generated by controller-gen. DO NOT EDIT.

package v1alpha1

import (
	"github.com/rabbitmq/messaging-topology-operator/api/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SuperStream) DeepCopyInto(out *SuperStream) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SuperStream.
func (in *SuperStream) DeepCopy() *SuperStream {
	if in == nil {
		return nil
	}
	out := new(SuperStream)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *SuperStream) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SuperStreamList) DeepCopyInto(out *SuperStreamList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]SuperStream, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SuperStreamList.
func (in *SuperStreamList) DeepCopy() *SuperStreamList {
	if in == nil {
		return nil
	}
	out := new(SuperStreamList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *SuperStreamList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SuperStreamSpec) DeepCopyInto(out *SuperStreamSpec) {
	*out = *in
	if in.RoutingKeys != nil {
		in, out := &in.RoutingKeys, &out.RoutingKeys
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	out.RabbitmqClusterReference = in.RabbitmqClusterReference
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SuperStreamSpec.
func (in *SuperStreamSpec) DeepCopy() *SuperStreamSpec {
	if in == nil {
		return nil
	}
	out := new(SuperStreamSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SuperStreamStatus) DeepCopyInto(out *SuperStreamStatus) {
	*out = *in
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make([]v1beta1.Condition, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.Partitions != nil {
		in, out := &in.Partitions, &out.Partitions
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SuperStreamStatus.
func (in *SuperStreamStatus) DeepCopy() *SuperStreamStatus {
	if in == nil {
		return nil
	}
	out := new(SuperStreamStatus)
	in.DeepCopyInto(out)
	return out
}