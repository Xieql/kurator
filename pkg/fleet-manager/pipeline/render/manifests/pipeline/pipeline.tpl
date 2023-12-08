apiVersion: tekton.dev/v1beta1
kind: Pipeline
metadata:
  name: git-clone-and-cat-readme
  namespace: kurator-pipeline
spec:
  description: |
    cat-branch-readme takes a git repository and a branch name and
    prints the README.md file from that branch. This is an example
    Pipeline demonstrating the following:
      - Using the git-clone catalog Task to clone a branch
      - Passing a cloned repo to subsequent Tasks using a Workspace.
      - Ordering Tasks in a Pipeline using "runAfter" so that
        git-clone completes before we try to read from the Workspace.
      - Using a volumeClaimTemplate Volume as a Workspace.
      - Avoiding hard-coded paths by using a Workspace's path
        variable instead.
  params:
    - name: repo-url
      type: string
      description: The git repository URL to clone from.
    - name: revision
      type: string
      description: The git branch to clone.
  workspaces:
    - name: kurator-pipeline-shared-data
      description: |
        This workspace will receive the cloned git repo and be passed
        to the next Task for the repo's README.md file to be read.
    - name: git-credentials
      description: |
        A Workspace containing a .gitconfig and .git-credentials file. These
        will be copied to the user's home before any git commands are run. Any
        other files in this Workspace are ignored.
  tasks:
    - name: fetch-repo
      taskRef:
        name: git-clone
      workspaces:
        - name: output
          workspace: kurator-pipeline-shared-data
        - name: basic-auth
          workspace: git-credentials
      params:
        - name: url
          value: $(params.repo-url)
        - name: revision
          value: $(params.revision)
    - name: cat-readme
      runAfter: ["fetch-repo"]  # Wait until the clone is done before reading the readme.
      workspaces:
        - name: source
          workspace: kurator-pipeline-shared-data
      taskSpec:
        workspaces:
          - name: source
        steps:
          - image: zshusers/zsh:4.3.15
            script: |
              #!/usr/bin/env zsh
              cat $(workspaces.source.path)/README.md