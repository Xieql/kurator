apiVersion: tekton.dev/v1beta1
kind: Pipeline
metadata:
  name: {{ .PipelineName}}
  namespace: {{ .PipelineNamespace }}
spec:
  description: |
    This is a universal pipeline with the following settings: 
      1. No parameters are passed because all user parameters have already been rendered into the corresponding tasks. 
      2. All tasks are strictly executed in the order defined by the user, with each task starting only after the previous one is completed. 
      3. There is only one workspace, which is used by all tasks. The PVC for this workspace will be configured in the trigger.
  params:
  workspaces:
    - name: kurator-pipeline-shared-data
      description: |
        This workspace is used by all tasks
    - name: git-credentials
      description: |
        A Workspace containing a .gitconfig and .git-credentials file. These
        will be copied to the user's home before any git commands are run. Any
        other files in this Workspace are ignored.
  tasks:
    - name: git-clone
      # Key points about 'git-clone':
      # - Fundamental for all tasks.
      # - Closely integrated with the trigger.
      # - Always the first task in the pipeline.
      # - Cannot be modified via templates.
      taskRef:
        name: git-clone-{{ .PipelineName }}
      workspaces:
        - name: source
          workspace: kurator-pipeline-shared-data
        - name: basic-auth
          workspace: git-credentials
      params:
        - name: url
          value: $(params.repo-url)
        - name: revision
          value: $(params.revision)
{{ .TasksInfo }}