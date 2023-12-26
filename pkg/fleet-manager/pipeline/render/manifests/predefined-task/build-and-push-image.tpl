apiVersion: tekton.dev/v1beta1
kind: Task
metadata:
  name: {{ .PredefinedTaskName }}
  namespace: {{ .PipelineNamespace }}
  labels:
    app.kubernetes.io/version: "0.6"
  annotations:
    tekton.dev/pipelines.minVersion: "0.17.0"
    tekton.dev/categories: Image Build
    tekton.dev/tags: image-build
    tekton.dev/displayName: "Build and upload container image using Kaniko"
    tekton.dev/platforms: "linux/amd64,linux/arm64,linux/ppc64le"
{{- if .OwnerReference }}
  ownerReferences:
  - apiVersion: "{{ .OwnerReference.APIVersion }}"
    kind: "{{ .OwnerReference.Kind }}"
    name: "{{ .OwnerReference.Name }}"
    uid: "{{ .OwnerReference.UID }}"
{{- end }}
spec:
  description: >-
    This Task builds a simple Dockerfile with kaniko and pushes to a registry.
    This Task stores the image name and digest as results, allowing Tekton Chains to pick up
    that an image was built & sign it.
  params:
  - name: IMAGE # This is a parameter that must be set.
    description: Name (reference) of the image to build.
    default: {{ default `Unknown` .Params.image }}
  - name: DOCKERFILE
    description: Path to the Dockerfile to build.
    default: {{ default `./Dockerfile` .Params.dockerfile }}
  - name: CONTEXT
    description: The build context used by Kaniko.
    default: {{ default `./` .Params.context }}
  - name: EXTRA_ARGS # more details see https://github.com/GoogleContainerTools/kaniko?tab=readme-ov-file#additional-flags
    type: array
    default: {{ default `[]` .Params.extra_args }}
  - name: BUILDER_IMAGE
    description: The image on which builds will run (default is v1.19.2)
    default: {{ default `gcr.io/kaniko-project/executor@sha256:f913ab076f92f1bdca336ab8514fea6e76f0311e52459cce5ec090c120885c8b` .Params.builder_image }}
  workspaces:
  - name: source
    description: Holds the context and Dockerfile
  - name: dockerconfig
    description: Includes a docker `config.json`
    optional: true
    mountPath: /kaniko/.docker
  results:
  - name: IMAGE_DIGEST
    description: Digest of the image just built.
  - name: IMAGE_URL
    description: URL of the image just built.
  steps:
  - name: build-and-push
    workingDir: $(workspaces.source.path)
    image: $(params.BUILDER_IMAGE)
    args:
    - $(params.EXTRA_ARGS)
    - --dockerfile=$(params.DOCKERFILE)
    - --context=$(workspaces.source.path)/$(params.CONTEXT) # The user does not need to care the workspace and the source.
    - --destination=$(params.IMAGE)
    - --digest-file=$(results.IMAGE_DIGEST.path)
    - --whitelist-var-run=false
    # kaniko assumes it is running as root, which means this example fails on platforms
    # that default to run containers as random uid (like OpenShift). Adding this securityContext
    # makes it explicit that it needs to run as root.
    securityContext:
      runAsUser: 0
  - name: write-url
    image: docker.io/library/bash:5.1.4@sha256:c523c636b722339f41b6a431b44588ab2f762c5de5ec3bd7964420ff982fb1d9
    script: |
      set -e
      image="$(params.IMAGE)"
      echo -n "${image}" | tee "$(results.IMAGE_URL.path)"
