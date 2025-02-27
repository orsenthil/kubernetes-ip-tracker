/*
Copyright 2025.

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

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PodTrackerSpec defines the desired state of PodTracker
type PodTrackerSpec struct {
	// Namespace to track pods in, if empty tracks all namespaces
	// +optional
	Namespace string `json:"namespace,omitempty"`
}

// PodInfo contains information about a pod
type PodInfo struct {
	// PodName is the name of the pod
	PodName string `json:"podName"`

	// Namespace is the namespace of the pod
	Namespace string `json:"namespace"`

	// IP is the IP address of the pod
	IP string `json:"ip"`

	// NodeName is the name of the node running the pod
	NodeName string `json:"nodeName"`

	// CreationTimestamp is when the pod was created
	CreationTimestamp metav1.Time `json:"creationTimestamp"`

	// Phase is the current lifecycle phase of the pod
	Phase string `json:"phase"`
}

// PodTrackerStatus defines the observed state of PodTracker
type PodTrackerStatus struct {
	// PodIPs contains a list of all tracked pods and their IPs
	// +optional
	PodIPs []PodInfo `json:"podIPs,omitempty"`

	// LastUpdateTime is the last time the resource was updated
	// +optional
	LastUpdateTime metav1.Time `json:"lastUpdateTime,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:scope=Cluster
//+kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
//+kubebuilder:printcolumn:name="Pods",type=integer,JSONPath=`.status.podIPs.length()`

// PodTracker is the Schema for the podtrackers API
type PodTracker struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PodTrackerSpec   `json:"spec,omitempty"`
	Status PodTrackerStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// PodTrackerList contains a list of PodTracker
type PodTrackerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PodTracker `json:"items"`
}

func init() {
	SchemeBuilder.Register(&PodTracker{}, &PodTrackerList{})
}
