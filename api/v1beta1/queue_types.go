/*


Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// using runtime.RawExtension to represent queue arguments
// interface{} is not currently supported by controller runtime
// recommendation is to use json.RawMessage or runtime.RawExtension to represent interface{}
// See: https://github.com/kubernetes-sigs/controller-tools/issues/294

// QueueSpec defines the desired state of Queue
type QueueSpec struct {
	// default to vhost '/'
	// +kubebuilder:default:=/
	Vhost string `json:"vhost,omitempty"`
	Type  string `json:"type,omitempty"`
	// when set to false queues does not survive server restart
	Durable bool `json:"durable,omitempty"`
	// when set to true, queues that has at least one consumer before, are deleted after last consumer unsubscribes
	AutoDelete bool `json:"autoDelete,omitempty"`
	// queue arguments in the format of KEY: VALUE
	// x-delivery-limit: 10000
	// +kubebuilder:validation:Type=object
	// +kubebuilder:pruning:PreserveUnknownFields
	Arguments *runtime.RawExtension `json:"arguments,omitempty"`
	// Reference to the RabbitmqCluster that the queue will be created in
	// Required property
	// +kubebuilder:validation:Required
	RabbitmqClusterReference RabbitmqClusterReference `json:"rabbitmqClusterReference"`
}

type RabbitmqClusterReference struct {
	// +kubebuilder:validation:Required
	Name string `json:"name"`
	// +kubebuilder:validation:Required
	Namespace string `json:"namespace"`
}

// QueueStatus defines the observed state of Queue
type QueueStatus struct {
}

// +kubebuilder:object:root=true

// Queue is the Schema for the queues API
type Queue struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   QueueSpec   `json:"spec,omitempty"`
	Status QueueStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// QueueList contains a list of Queue
type QueueList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Queue `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Queue{}, &QueueList{})
}
