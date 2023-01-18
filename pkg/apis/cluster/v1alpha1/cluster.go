/*
Copyright Kurator Authors.

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

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +kubebuilder:resource:path=customclusters,shortName=cc
// +kubebuilder:subresource:status

// CustomCluster is the schema for existing node based Kubernetes Cluster API.
type CustomCluster struct {
	metav1.TypeMeta `json:",inline"`

	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Specification of the desired behavior of the kurator cluster.
	Spec CustomClusterSpec `json:"spec,omitempty"`

	// Current status of the kurator cluster
	Status CustomClusterStatus `json:"status,omitempty"`
}

// CustomClusterSpec defines the desired state of a kurator cluster.
type CustomClusterSpec struct {
	// TODO: any UCS specific configurations that does not exist in upstream cluster api

	// MachineRef is the reference of nodes for provisioning a kurator cluster.
	// +optional
	MachineRef *corev1.ObjectReference `json:"machineRef,omitempty"`

	// ClusterRef is the reference of the cluster from clusterAPI.
	// +optional
	ClusterRef *corev1.ObjectReference `json:"clusterRef,omitempty"`
}

type CustomClusterPhase string

const (
	PendingPhase     CustomClusterPhase = "Pending"
	RunningPhase     CustomClusterPhase = "Running"
	SucceededPhase   CustomClusterPhase = "Succeeded"
	TerminatingPhase CustomClusterPhase = "Terminating"

	InitFailedPhase      CustomClusterPhase = "InitFailed"
	TerminateFailedPhase CustomClusterPhase = "TerminateFailed"

	FailedPhase CustomClusterPhase = "Failed"
)

// CustomClusterStatus represents the current status of the cluster.
type CustomClusterStatus struct {

	// Phase represents the current phase of cluster actuation.
	// E.g.  Running, Succeed, Terminating, Failed etc.
	// +optional
	Phase CustomClusterPhase `json:"phase,omitempty"`

	// Message indicating details about why the pod is in this condition.
	// +optional
	Message string `json:"message,omitempty"`
}

// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// CustomClusterList contains a list of CustomCluster.
type CustomClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []CustomCluster `json:"items"`
}
