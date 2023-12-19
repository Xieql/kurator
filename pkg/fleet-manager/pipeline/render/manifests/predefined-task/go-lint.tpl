apiVersion: tekton.dev/v1beta1
kind: Task
metadata:
  name: {{ .PredefinedTaskName }}
  namespace: {{ .PipelineNamespace }}
  labels:
    app.kubernetes.io/version: "0.2"
  annotations:
    tekton.dev/pipelines.minVersion: "0.12.1"
    tekton.dev/categories: Code Quality
    tekton.dev/tags: lint
    tekton.dev/displayName: "golangci lint"
    tekton.dev/platforms: "linux/amd64"
{{- if .OwnerReference }}
  ownerReferences:
  - apiVersion: "{{ .OwnerReference.APIVersion }}"
    kind: "{{ .OwnerReference.Kind }}"
    name: "{{ .OwnerReference.Name }}"
    uid: "{{ .OwnerReference.UID }}"
{{- end }}
spec:
  description: >-
    This Task is Golang task to validate Go projects.

  params:
  - name: package
    description: base package (and its children) under validation
    default: "{{ default `.` .Params.package }}"
  - name: context
    description: path to the directory to use as context.
    default: "{{ default `.` .Params.context }}"
  - name: flags
    description: flags to use for the lint command
    default: "{{ default `--verbose` .Params.flags }}"
  - name: version
    description: golangci-lint version to use
    default: "{{ default `latest` .Params.version }}"
  - name: GOOS
    description: "running program's operating system target"
    default: "{{ default `linux` .Params.GOOS }}"
  - name: GOARCH
    description: "running program's architecture target"
    default: "{{ default `amd64` .Params.GOARCH }}"
  - name: GO111MODULE
    description: "value of module support"
    default: "{{ default `auto` .Params.GO111MODULE }}"
  - name: GOCACHE
    description: "Go caching directory path"
    default: "{{ default `` .Params.GOCACHE }}"
  - name: GOMODCACHE
    description: "Go mod caching directory path"
    default: "{{ default `` .Params.GOMODCACHE }}"
  - name: GOLANGCI_LINT_CACHE
    description: "golangci-lint cache path"
    default: "{{ default `` .Params.GOLANGCI_LINT_CACHE }}"
  workspaces:
  - name: source
    mountPath: /workspace/src/$(params.package)
  steps:
  - name: lint
    image: docker.io/golangci/golangci-lint:$(params.version)
    workingDir: $(workspaces.source.path)/$(params.context)
    script: |
      golangci-lint run $(params.flags)
    env:
    - name: GOPATH
      value: /workspace
    - name: GOOS
      value: "$(params.GOOS)"
    - name: GOARCH
      value: "$(params.GOARCH)"
    - name: GO111MODULE
      value: "$(params.GO111MODULE)"
    - name: GOCACHE
      value: "$(params.GOCACHE)"
    - name: GOMODCACHE
      value: "$(params.GOMODCACHE)"
    - name: GOLANGCI_LINT_CACHE
      value: "$(params.GOLANGCI_LINT_CACHE)"
