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

// Destination defines a fleet or specific clusters.
type Destination struct {
	// Fleet is the name of fleet.
	// The field, in combination with ClusterSelector, can determine a set of clusters.
	// In addition to this approach, users can also directly specify clusters through the field Clusters.
	// +optional
	Fleet string `json:"fleet,omitempty"`

	// ClusterSelector specifies the selectors to select the clusters within the fleet.
	// If unspecified, all clusters in the fleet will be selected.
	// The field will only take effect when Fleet is set.
	// +optional
	ClusterSelector *ClusterSelector `json:"clusterSelector,omitempty"`

	// Clusters determine a set of clusters as destination clusters.
	// The field will only take effect when Fleet is not set.
	// +optional
	Clusters []*corev1.ObjectReference `json:"clusters,omitempty"`
}

type ClusterSelector struct {
	// MatchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels map is equivalent to an element of matchExpressions, whose key field is "key", the operator is "In", and the values array contains only "value".
	// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/#label-selectors
	// +optional
	MatchLabels map[string]string `json:"matchLabels,omitempty"`
}

// Note: partly copied from https://github.com/"github.com/vmware-tanzu/velero/pkg/apis/backup_types.go
type ResourceFilter struct {
	// IncludedNamespaces is a slice of namespace names to include objects from.
	// If empty, all namespaces are included.
	// +optional
	// +nullable
	IncludedNamespaces []string `json:"includedNamespaces,omitempty"`

	// ExcludedNamespaces contains a list of namespaces that are not included in the backup.
	// +optional
	// +nullable
	ExcludedNamespaces []string `json:"excludedNamespaces,omitempty"`

	// IncludedResources is a slice of resource names to include in the backup.
	// For example, we can populate this string array with "deployments" and "configmaps", then we will select all resources of type deployments and configmaps,
	// If empty, all resources are included.
	// Cannot work with IncludedClusterScopedResources, ExcludedClusterScopedResources, IncludedNamespaceScopedResources and ExcludedNamespaceScopedResources.
	// +optional
	// +nullable
	IncludedResources []string `json:"includedResources,omitempty"`

	// ExcludedResources is a slice of resource names that are not included in the backup.
	// Cannot work with IncludedClusterScopedResources, ExcludedClusterScopedResources, IncludedNamespaceScopedResources and ExcludedNamespaceScopedResources.
	// +optional
	// +nullable
	ExcludedResources []string `json:"excludedResources,omitempty"`

	// IncludeClusterResources specifies whether cluster-scoped resources should be included for consideration in the backup.
	// Cannot work with IncludedClusterScopedResources, ExcludedClusterScopedResources, IncludedNamespaceScopedResources and ExcludedNamespaceScopedResources.
	// +optional
	// +nullable
	IncludeClusterResources *bool `json:"includeClusterResources,omitempty"`

	// IncludedClusterScopedResources is a slice of cluster-scoped resource type names to include in the backup.
	// For example, we can populate this string array with "storageclasses" and "clusterroles", then we will select all resources of type storageclasses and clusterroles,
	// If set to "*", all cluster-scoped resource types are included.
	// The default value is empty, which means only related cluster-scoped resources are included.
	// Cannot work with IncludedResources, ExcludedResources and IncludeClusterResources.
	// +optional
	// +nullable
	IncludedClusterScopedResources []string `json:"includedClusterScopedResources,omitempty"`

	// ExcludedClusterScopedResources is a slice of cluster-scoped resource type names to exclude from the backup.
	// If set to "*", all cluster-scoped resource types are excluded. The default value is empty.
	// Cannot work with IncludedResources, ExcludedResources and IncludeClusterResources.
	// +optional
	// +nullable
	ExcludedClusterScopedResources []string `json:"excludedClusterScopedResources,omitempty"`

	// IncludedNamespaceScopedResources is a slice of namespace-scoped resource type names to include in the backup.
	// For example, we can populate this string array with "deployments" and "configmaps", then we will select all resources of type deployments and configmaps,
	// The default value is "*".
	// Cannot work with IncludedResources, ExcludedResources and IncludeClusterResources.
	// +optional
	// +nullable
	IncludedNamespaceScopedResources []string `json:"includedNamespaceScopedResources,omitempty"`

	// ExcludedNamespaceScopedResources is a slice of namespace-scoped resource type names to exclude from the backup.
	// If set to "*", all namespace-scoped resource types are excluded. The default value is empty.
	// Cannot work with IncludedResources, ExcludedResources and IncludeClusterResources.
	// +optional
	// +nullable
	ExcludedNamespaceScopedResources []string `json:"excludedNamespaceScopedResources,omitempty"`

	// LabelSelector is a metav1.LabelSelector to filter with when adding individual objects to the backup.
	// If empty or nil, all objects are included. Optional.
	// +optional
	// +nullable
	LabelSelector *metav1.LabelSelector `json:"labelSelector,omitempty"`

	// OrLabelSelectors is list of metav1.LabelSelector to filter with when adding individual objects to the backup.
	// If multiple provided they will be joined by the OR operator.
	// LabelSelector as well as OrLabelSelectors cannot co-exist in backup request, only one of them can be used.
	// +optional
	// +nullable
	OrLabelSelectors []*metav1.LabelSelector `json:"orLabelSelectors,omitempty"`
}
