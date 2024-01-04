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

package render

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	pipelineapi "kurator.dev/kurator/pkg/apis/pipeline/v1alpha1"
)

const (
	TriggerTemplateFile = "trigger/trigger.tpl"
	TriggerTemplateName = "pipeline trigger template"
)

// VolumeClaimTemplate is the configuration for the volume claim template in pipeline execution.
// For more details, see https://github.com/kubernetes/api/blob/master/core/v1/types.go
type VolumeClaimTemplate struct {
	// AccessMode determines the access modes for the volume, e.g., ReadWriteOnce.
	// This affects how the volume can be mounted.
	// "ReadWriteOnce" can be mounted in read/write mode to exactly 1 host
	// "ReadOnlyMany" can be mounted in read-only mode to many hosts
	// "ReadWriteMany" can be mounted in read/write mode to many hosts
	// "ReadWriteOncePod" can be mounted in read/write mode to exactly 1 pod, cannot be used in combination with other access modes
	AccessMode corev1.PersistentVolumeAccessMode `json:"accessMode,omitempty"`

	// StorageRequest defines the storage size required for this PVC, e.g., 1Gi, 100Mi.
	// It specifies the storage capacity needed as part of ResourceRequirements.
	// +kubebuilder:validation:Pattern="^[0-9]+(\\.[0-9]+)?(Gi|Mi)$"
	StorageRequest string `json:"requestsStorage,omitempty"`

	// StorageClassName specifies the StorageClass name to which this persistent volume belongs, e.g., manual.
	// It allows the PVC to use the characteristics defined by the StorageClass.
	StorageClassName string `json:"storageClassName,omitempty"`

	// VolumeMode specifies whether the volume should be used with a formatted filesystem (Filesystem)
	// or remain in raw block state (Block). The Filesystem value is implied when not included.
	// "Block"  means the volume will not be formatted with a filesystem and will remain a raw block device.
	// "Filesystem"  means the volume will be or is formatted with a filesystem.
	VolumeMode corev1.PersistentVolumeMode `json:"volumeMode,omitempty"`
}

type TriggerConfig struct {
	PipelineName      string
	PipelineNamespace string
	OwnerReference    *metav1.OwnerReference
	AccessMode        string
	StorageRequest    string
	StorageClassName  string
	VolumeMode        string
}

// ServiceAccountName is the service account used by trigger
func (cfg TriggerConfig) ServiceAccountName() string {
	return cfg.PipelineName
}

func RenderTriggerWithPipeline(pipeline *pipelineapi.Pipeline) ([]byte, error) {
	config := TriggerConfig{
		PipelineName:      pipeline.Name,
		PipelineNamespace: pipeline.Namespace,
		OwnerReference:    GeneratePipelineOwnerRef(pipeline),
		AccessMode:        string(pipeline.Spec.SharedWorkspace.AccessMode),
		StorageRequest:    pipeline.Spec.SharedWorkspace.StorageRequest,
		StorageClassName:  pipeline.Spec.SharedWorkspace.StorageClassName,
		VolumeMode:        string(pipeline.Spec.SharedWorkspace.VolumeMode),
	}

	return RenderTrigger(config)
}

func RenderTrigger(cfg TriggerConfig) ([]byte, error) {
	return renderTemplate(TriggerTemplateFile, TriggerTemplateName, cfg)
}

const TriggerTemplateContent = `apiVersion: v1
kind: ServiceAccount
metadata:
  name: "{{ .ServiceAccountName }}"
  namespace: "{{ .PipelineNamespace }}"
{{- if .OwnerReference }}
  ownerReferences:
  - apiVersion: "{{ .OwnerReference.APIVersion }}"
    kind: "{{ .OwnerReference.Kind }}"
    name: "{{ .OwnerReference.Name }}"
    uid: "{{ .OwnerReference.UID }}"
{{- end }}
secrets:
  - name: "chain-credentials"
    namespace: "{{ .PipelineNamespace }}"
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: "{{ .BroadResourceRoleBindingName }}"
  namespace: "{{ .PipelineNamespace }}"
{{- if .OwnerReference }}
  ownerReferences:
  - apiVersion: "{{ .OwnerReference.APIVersion }}"
    kind: "{{ .OwnerReference.Kind }}"
    name: "{{ .OwnerReference.Name }}"
    uid: "{{ .OwnerReference.UID }}"
{{- end }}
subjects:
- kind: ServiceAccount
  name: "{{ .ServiceAccountName }}"
  namespace: "{{ .PipelineNamespace }}"
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: tekton-triggers-eventlistener-roles # add role for handle broad-resource, such as eventListener, triggers, configmaps and so on. tekton-triggers-eventlistener-roles is provided by Tekton
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: "{{ .SecretRoleBindingName }}"
  namespace: "{{ .PipelineNamespace }}"
{{- if .OwnerReference }}
  ownerReferences:
  - apiVersion: "{{ .OwnerReference.APIVersion }}"
    kind: "{{ .OwnerReference.Kind }}"
    name: "{{ .OwnerReference.Name }}"
    uid: "{{ .OwnerReference.UID }}"
{{- end }}
subjects:
- kind: ServiceAccount
  name: "{{ .ServiceAccountName }}"
  namespace: "{{ .PipelineNamespace }}"
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: tekton-triggers-eventlistener-clusterroles # add role for handle secret, clustertriggerbinding and clusterinterceptors. tekton-triggers-eventlistener-clusterroles is provided by Tekton
`
