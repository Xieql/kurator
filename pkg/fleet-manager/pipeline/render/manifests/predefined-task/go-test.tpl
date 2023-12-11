apiVersion: tekton.dev/v1beta1
kind: Task
metadata:
  name: golang-test-{{.PipelineName}}
  namespace: {{ .PipelineNamespace }}
  labels:
    app.kubernetes.io/version: "0.2"
  annotations:
    tekton.dev/pipelines.minVersion: "0.12.1"
    tekton.dev/categories: Testing
    tekton.dev/tags: test
    tekton.dev/displayName: "golang test"
    tekton.dev/platforms: "linux/amd64,linux/s390x,linux/ppc64le"
spec:
  description: >-
    This Task is Golang task to test Go projects.

  params:
  - name: package
    description: package (and its children) under test
    default: "{{ default `.` .Params.package }}"
  - name: packages
    description: "packages to test (default: ./...)"
    default: "{{ default `./...` .Params.packages }}"
  - name: context
    description: path to the directory to use as context.
    default: "{{ default `.` .Params.context }}"
  - name: version
    description: golang version to use for tests
    default: "{{ default `latest` .Params.version }}"
  - name: flags
    description: flags to use for the test command
    default: "{{ default `-race -cover -v` .Params.flags }}"
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
  workspaces:
  - name: source
  steps:
  - name: unit-test
    image: docker.io/library/golang:$(params.version)
    workingDir: $(workspaces.source.path)
    script: |
      if [ ! -e $GOPATH/src/$(params.package)/go.mod ];then
         SRC_PATH="$GOPATH/src/$(params.package)"
         mkdir -p $SRC_PATH
         cp -R "$(workspaces.source.path)/$(params.context)"/* $SRC_PATH
         cd $SRC_PATH
      fi
      go test $(params.flags) $(params.packages)
    env:
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
