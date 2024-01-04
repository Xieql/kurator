apiVersion: triggers.tekton.dev/v1alpha1
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
      - name: kurator-pipeline-shared-data # there only one pvc workspace in each pipeline, and the name is `kurator-pipeline-shared-data`
        volumeClaimTemplate:
          spec:
            accessModes:
              -  {{ default `ReadWriteOnce` .AccessMode }}
            resources:
              requests:
                storage: {{ default `1Gi` .StorageRequest }}
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
