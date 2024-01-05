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
	return renderTemplate(TriggerTemplateContent, TriggerTemplateName, cfg)
}

const TriggerTemplateContent = `apiVersion: triggers.tekton.dev/v1alpha1
kind: TriggerTemplate
metadata:
  name: {{ .PipelineName }}-triggertemplate
  namespace: {{ .PipelineNamespace }}
{{- if .OwnerReference }}
  ownerReferences:
  - apiVersion: "{{ .OwnerReference.APIVersion }}"
    kind: "{{ .OwnerReference.Kind }}"
    name: "{{ .OwnerReference.Name }}"
    uid: "{{ .OwnerReference.UID }}"
{{- end }}
spec:
  params:
  - name: gitrevision
    description: The git revision
  - name: gitrepositoryurl
    description: The git repository url
  - name: namespace
    description: The namespace to create the resources
  resourceTemplates:
  - apiVersion: tekton.dev/v1beta1
    kind: PipelineRun
    metadata:
      generateName: {{ .PipelineName }}-run-
      namespace: $(tt.params.namespace)
    spec:
      serviceAccountName: {{ .ServiceAccountName }}
      pipelineRef:
        name: {{ .PipelineName }}
      params:
      - name: revision
        value: $(tt.params.gitrevision)
      - name: repo-url
        value: $(tt.params.gitrepositoryurl)
      workspaces:
      - name: kurator-pipeline-shared-data # there only one pvc workspace in each pipeline, and the name is kurator-pipeline-shared-data
        volumeClaimTemplate:
          spec:
            accessModes:
              - {{ default "ReadWriteOnce" .AccessMode }}
            resources:
              requests:
                storage: {{ default "1Gi" .StorageRequest }}
{{- if .VolumeMode }}
            volumeMode: {{ .VolumeMode }}
{{- end }}
{{- if .StorageClassName }}
            storageClassName: {{ .StorageClassName }}
{{- end }}
      - name: git-credentials
        secret:
          secretName: git-credentials
      - name: docker-credentials
        secret:
          secretName: docker-credentials  # auth for task
---
apiVersion: triggers.tekton.dev/v1alpha1
kind: TriggerBinding
metadata:
  name: {{ .PipelineName }}-triggerbinding
  namespace: {{ .PipelineNamespace}}
{{- if .OwnerReference }}
  ownerReferences:
  - apiVersion: "{{ .OwnerReference.APIVersion }}"
    kind: "{{ .OwnerReference.Kind }}"
    name: "{{ .OwnerReference.Name }}"
    uid: "{{ .OwnerReference.UID }}"
{{- end }}
spec:
  params:
  - name: gitrevision
    value: $(body.head_commit.id)
  - name: namespace
    value: {{ .PipelineNamespace}}
  - name: gitrepositoryurl
    value: "https://github.com/$(body.repository.full_name)"
---
apiVersion: triggers.tekton.dev/v1alpha1
kind: EventListener
metadata:
  name: {{ .PipelineName }}-listener
  namespace: {{ .PipelineNamespace}}
{{- if .OwnerReference }}
  ownerReferences:
  - apiVersion: "{{ .OwnerReference.APIVersion }}"
    kind: "{{ .OwnerReference.Kind }}"
    name: "{{ .OwnerReference.Name }}"
    uid: "{{ .OwnerReference.UID }}"
{{- end }}
spec:
  serviceAccountName: {{ .ServiceAccountName }}
  triggers:
  - bindings:
    - ref: {{ .PipelineName }}-triggerbinding
    template:
      ref: {{ .PipelineName }}-triggertemplate
`
