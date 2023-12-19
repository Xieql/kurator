apiVersion: tekton.dev/v1beta1
kind: Task
metadata:
  name: {{ .CustomTaskName }}
  namespace: {{ .PipelineNamespace }}
{{- if .OwnerReference }}
  ownerReferences:
  - apiVersion: "{{ .OwnerReference.APIVersion }}"
    kind: "{{ .OwnerReference.Kind }}"
    name: "{{ .OwnerReference.Name }}"
    uid: "{{ .OwnerReference.UID }}"
{{- end }}
spec:
  description: >-
    This task is a user-custom, single-step task.
    The workspace is automatically and exclusively created named "source",
    and assigned to the workspace of the pipeline in which this task is located.
  workspaces:
    - name: source
      description: The workspace where user to run user-custom task.
  steps:
    - name: {{ .CustomTaskName }}
      image: {{ .Image }}
      {{- if .Args }}
      args:
      {{- range .Args }}
        - {{ . }}
      {{- end }}
      {{- end }}
      {{- if .Env }}
      env:
      {{- range .Env }}
        - name: {{ .Name }}
          value: {{ .Value }}
      {{- end }}
      {{- end }}
      {{- if .Command }}
      command:
      {{- range .Command }}
        - {{ . }}
      {{- end }}
      {{- end }}
      {{- if .Script }}
      script: |
        {{ .Script }}
      {{- end }}
      {{- if .ResourceRequirements }}
      resources:
        {{- if .ResourceRequirements.Requests }}
        requests:
          {{- if .ResourceRequirements.Requests.Cpu }}
          cpu: {{ .ResourceRequirements.Requests.Cpu }}
          {{- end }}
          {{- if .ResourceRequirements.Requests.Memory }}
          memory: {{ .ResourceRequirements.Requests.Memory }}
          {{- end }}
        {{- end }}
        {{- if .ResourceRequirements.Limits }}
        limits:
          {{- if .ResourceRequirements.Limits.Cpu }}
          cpu: {{ .ResourceRequirements.Limits.Cpu }}
          {{- end }}
          {{- if .ResourceRequirements.Limits.Memory }}
          memory: {{ .ResourceRequirements.Limits.Memory }}
          {{- end }}
        {{- end }}
      {{- end }}
