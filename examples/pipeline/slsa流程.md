
## 官方途径基本流程

###  前置条件

### 创建测试ns
```
k create ns kurator-pipeline
```

### 创建签名校验所需密钥。

安装 cosign 工具，参考 https://docs.sigstore.dev/system_config/installation/  一句话介绍 cosign，pipeline 会使用cosign 进行签名 和校验


注意：该密钥名称固定，且内容无法直接变更。如果需要变更，需要先删除再创建。

```
cosign generate-key-pair k8s://tekton-chains/signing-secrets
```

执行过程中会要求输入2次密码，测试的时候可以直接输入两次空格

这个命令 会在 ns 创建一个 公钥私钥的密钥

并且会在当前本地路径创建一个 cosign.pub的公钥文件

### 配置 git仓库认证

```
kubectl create secret generic git-credentials \
  --namespace=kurator-pipeline \
  --from-literal=.gitconfig=$'[credential "https://github.com"]\n\thelper = store' \
  --from-literal=.git-credentials='https://Xieql:xxx@github.com'
```

### 配置 镜像仓库认证

首先docker login 登录得到密码文件 config.json.
然后通过 这个密码文件创建两个 不同格式的 分别用于 task任务自身 以及 chain controller 的两个secret

#### docker login

```
docker login ghcr.io -u xieql -p xxx
```

登录成功后会有一个提示

```
WARNING! Your password will be stored unencrypted in /root/.docker/config.json.
```

/root/.docker/config.json 这个路径就是 docker 的认证信息，我们通过这个信息来创建secret

#### 创建 task 所需镜像仓库认证的secret

创建用于 task 上传image 到 oci仓库所需的 secret

```
kubectl create secret generic docker-credentials --from-file=/root/.docker/config.json -n kurator-pipeline

```

该 secret 作为 task 的 workspace 的参数，从而 task 获取认证的权限


#### 创建 chain controller 所需镜像仓库的secret

创建用于 chain controller 上传 sig 和 att 到 oci仓库所需的secret

```
kubectl create secret generic chain-credentials \
    --from-file=.dockerconfigjson=/root/.docker/config.json \
    --type=kubernetes.io/dockerconfigjson \
    -n kurator-pipeline

```

关联 该 secret 到 service account, 已经写在 rbac.yaml

chain-credentials 的认证信息 将 通过service account 传给 chain controller


### 配置 tekton chain 相关参数

```
kubectl patch configmap chains-config -n tekton-chains -p='{"data":{"artifacts.taskrun.format": "slsa/v1"}}'
kubectl patch configmap chains-config -n tekton-chains -p='{"data":{"artifacts.taskrun.storage": "oci"}}'
kubectl patch configmap chains-config -n tekton-chains -p='{"data":{"artifacts.oci.storage": "oci"}}'
kubectl patch configmap chains-config -n tekton-chains -p='{"data":{"transparency.enabled": "true"}}'
```

### 创建 kurator pipeline 测试例子

#### apply kurator pipeline 例子

```
echo 'apiVersion: pipeline.kurator.dev/v1alpha1
kind: Pipeline
metadata:
  name: quick-start
  namespace: kurator-pipeline
spec:
  description: "this is a quick-start pipeline, it shows how to use customTask and predefined Task in a pipeline"
  tasks:
    - name: git-clone
      predefinedTask:
        name: git-clone
        params:
          git-secret-name: git-credentials
    - name: cat-readme
      customTask:
        image: zshusers/zsh:4.3.15
        command:
          - /bin/sh
          - -c
        args:
          - "cat $(workspaces.source.path)/README.md"
    - name: go-test
      predefinedTask:
        name: go-test
        params:
          packages: ./...
    - name: go-lint
      predefinedTask:
        name: go-lint
        params:
          packages: "./..."
          flags: "--disable=errcheck,unused,gosimple,staticcheck --verbose --timeout 10m"
    - name: build-and-push-image
      predefinedTask:
        name: build-and-push-image
        params:
          image: "ghcr.io/<username>/<image-name>"'| kubectl apply -f -
```

请注意，请将这里的 <username> 改成你的 github 用户名，将 image-name 改为你的镜像名称，例如 kurator-test:0.6.0

## 暴露服务

```
kubectl port-forward --address 0.0.0.0 service/el-quick-start-listener 30002:8080 -n kurator-pipeline
```

## 配置 webhook
参考 https://kurator.dev/docs/pipeline/operation/ 了解如何给上述服务配置 webhook

## 触发 pipeline 

To trigger the pipeline, you might try pushing some content to the repository, such as a modification to the README. Information about the received event can be observed in the window where the forward service is running.

```
Forwarding from 0.0.0.0:30002 -> 8080
Handling connection for 30002
```

## 查看pipelin执行结果

After the pipeline is triggered, the system will create individual pods for each task in the pipeline, executing them in order. You can view the current task execution status with a specific command.

```
$ kubectl  get pod -n kurator-pipeline | grep quick-start
el-quick-start-listener-fbf79b78-tdtjz             1/1     Running     0          12m
quick-start-run-q45w6-build-and-push-image-pod     0/2     Completed   0          3m57s
quick-start-run-q45w6-cat-readme-pod               0/1     Completed   0          6m40s
quick-start-run-q45w6-git-clone-pod                0/1     Completed   0          6m50s
quick-start-run-q45w6-go-lint-pod                  0/1     Completed   0          5m11s
quick-start-run-q45w6-go-test-pod                  0/1     Completed   0          6m37s
```

同样的，你也可以在 `kurator pipeline execution list  -n kurator-pipeline  --kubeconfig /root/.kube/kurator-host.config` 命令 拿到事件触发的  pipeline execution 名称后，
通过 通过下面命令查看该pipeline 各个日志的执行结果

```
kurator pipeline execution logs <pipeline-execution>  -n kurator-pipeline --tail 10 --kubeconfig /root/.kube/kurator-host.config
INFO[2024-01-04 11:47:34] Fetching logs for TaskRun: quick-start-run-frb7l-git-clone 
INFO[2024-01-04 11:47:34] Fetching logs for container 'step-clone' in Pod 'quick-start-run-frb7l-git-clone-pod' 
INFO[2024-01-04 11:47:34] Logs from container 'step-clone':
+ cd /workspace/source/
+ git rev-parse HEAD
+ RESULT_SHA=1858f8e5129516d6e7d9ad993b1ec41cef922d18
+ EXIT_CODE=0
+ '[' 0 '!=' 0 ]
+ git log -1 '--pretty=%ct'
+ RESULT_COMMITTER_DATE=1703581193
+ printf '%s' 1703581193
+ printf '%s' 1858f8e5129516d6e7d9ad993b1ec41cef922d18
+ printf '%s' https://github.com/Xieql/podinfo 
INFO[2024-01-04 11:47:34] Fetching logs for TaskRun: quick-start-run-frb7l-build-and-push-image 
INFO[2024-01-04 11:47:34] Fetching logs for container 'step-build-and-push' in Pod 'quick-start-run-frb7l-build-and-push-image-pod' 
INFO[2024-01-04 11:47:34] Logs from container 'step-build-and-push':
INFO[0161] RUN chown -R app:app ./                      
INFO[0161] Cmd: /bin/sh                                 
INFO[0161] Args: [-c chown -R app:app ./]               
INFO[0161] Running: [/bin/sh -c chown -R app:app ./]    
INFO[0161] Taking snapshot of full filesystem...        
INFO[0162] USER app                                     
INFO[0162] Cmd: USER                                    
INFO[0162] CMD ["./podinfo"]                            
INFO[0162] Pushing image to ghcr.io/xieql/kurator-final:0.4.1 
INFO[0188] Pushed ghcr.io/xieql/kurator-final@sha256:73c1ad5046233adb70aae2ee5df6e00f2c521e89cc980a954dc024d12add8daf  
INFO[2024-01-04 11:47:34] Fetching logs for container 'step-write-url' in Pod 'quick-start-run-frb7l-build-and-push-image-pod' 
INFO[2024-01-04 11:47:34] Logs from container 'step-write-url':
ghcr.io/xieql/kurator-final:0.4.1 
INFO[2024-01-04 11:47:34] Fetching logs for TaskRun: quick-start-run-frb7l-cat-readme 
INFO[2024-01-04 11:47:34] Fetching logs for container 'step-cat-readme-quick-start' in Pod 'quick-start-run-frb7l-cat-readme-pod' 
INFO[2024-01-04 11:47:34] Logs from container 'step-cat-readme-quick-start':
To delete podinfo's Helm repository and release from your cluster run:

flux -n default delete source helm podinfo
flux -n default delete helmrelease podinfo

If you wish to manage the lifecycle of your applications in a **GitOps** manner, check out
this [workflow example](https://github.com/fluxcd/flux2-kustomize-helm-example)
for multi-env deployments with Flux, Kustomize and Helm. 
INFO[2024-01-04 11:47:34] Fetching logs for TaskRun: quick-start-run-frb7l-go-test 
INFO[2024-01-04 11:47:34] Fetching logs for container 'step-unit-test' in Pod 'quick-start-run-frb7l-go-test-pod' 
INFO[2024-01-04 11:47:34] Logs from container 'step-unit-test':
--- PASS: TestInfoHandler (0.00s)
=== RUN   TestStatusHandler
--- PASS: TestStatusHandler (0.00s)
=== RUN   TestTokenHandler
--- PASS: TestTokenHandler (0.00s)
=== RUN   TestVersionHandler
--- PASS: TestVersionHandler (0.00s)
PASS
coverage: 14.4% of statements
ok  	github.com/stefanprodan/podinfo/pkg/api	1.088s	coverage: 14.4% of statements 
INFO[2024-01-04 11:47:34] Fetching logs for TaskRun: quick-start-run-frb7l-go-lint 
INFO[2024-01-04 11:47:34] Fetching logs for container 'step-lint' in Pod 'quick-start-run-frb7l-go-lint-pod' 
INFO[2024-01-04 11:47:34] Logs from container 'step-lint':
level=info msg="[config_reader] Config search paths: [./ /workspace/src /workspace / /root]"
level=info msg="[lintersdb] Active 2 linters: [govet ineffassign]"
level=info msg="[loader] Go packages loading at mode 575 (deps|imports|types_sizes|compiled_files|files|name|exports_file) took 1m0.435700461s"
level=info msg="[runner/filename_unadjuster] Pre-built 0 adjustments in 4.381259ms"
level=info msg="[linters_context/goanalysis] analyzers took 1.651993451s with top 10 stages: inspect: 842.409271ms, ctrlflow: 402.369985ms, printf: 382.21663ms, ineffassign: 12.079525ms, slog: 3.20068ms, lostcancel: 1.346293ms, copylocks: 1.333706ms, directive: 1.287065ms, bools: 976.125µs, composites: 435.523µs"
level=info msg="[runner] processing took 2.828µs with stages: max_same_issues: 349ns, skip_dirs: 325ns, nolint: 302ns, cgo: 230ns, exclude-rules: 208ns, max_from_linter: 146ns, source_code: 143ns, path_prettifier: 133ns, filename_unadjuster: 133ns, autogenerated_exclude: 131ns, skip_files: 124ns, identifier_marker: 118ns, max_per_file_from_linter: 72ns, severity-rules: 59ns, path_shortener: 56ns, sort_results: 53ns, diff: 51ns, exclude: 50ns, uniq_by_line: 50ns, fixer: 50ns, path_prefixer: 45ns"
level=info msg="[runner] linters took 3.81614172s with stages: goanalysis_metalinter: 3.816076131s"
level=info msg="File cache stats: 0 entries of total size 0B"
level=info msg="Memory: 644 samples, avg is 35.5MB, max is 435.9MB"
level=info msg="Execution took 1m4.265454941s" 
```

## 在github中 查看已上传的镜像、签名与 出处证明

当上述例子的pipelin 执行 完毕后，
登录你的 github，进入 package 页面，你会发现新增了 所指定的image，点击后可以看到类似下面的内容：



## 验证签名

登录 ghcr 查看，可以看到对应pkg下的镜像以及对应的 .sig 签名 和 .att 出处证明

通过如下方式验证签名：（在之前cosign 过程中创建的 cosign.pub，用来作为签名校验的公钥）

```
cosign verify --key cosign.pub ghcr.io/xieql/kaniko-chains2
cosign verify-attestation --key cosign.pub --type slsaprovenance ghcr.io/xieql/kaniko-chains2
```

如果验证失败，会有明确的错误信息（签名不匹配、前面无效等）。
如果验证成功，则会显示关于签名的详细信息（包括镜像的docker）。

以下是验证成功显示结果的参考


```
#cosign verify --key cosign.pub ghcr.io/xieql/kaniko-chains

Verification for ghcr.io/xieql/kaniko-chains:latest --
The following checks were performed on each of these signatures:
  - The cosign claims were validated
  - The claims were present in the transparency log
  - The signatures were integrated into the transparency log when the certificate was valid
  - The signatures were verified against the specified public key

[{"critical":{"identity":{"docker-reference":"ghcr.io/xieql/kaniko-chains"},"image":{"docker-manifest-digest":"sha256:a4e1fb3e11f3c0ad167ed9868b7c6fcfffd7923a61e8bd15fbfdf8cda109cb58"},"type":"cosign container image signature"},"optional":null}]
```

```
# cosign verify-attestation --key cosign.pub --type slsaprovenance ghcr.io/xieql/kaniko-chains

Verification for ghcr.io/xieql/kaniko-chains --
The following checks were performed on each of these signatures:
  - The cosign claims were validated
  - The claims were present in the transparency log
  - The signatures were integrated into the transparency log when the certificate was valid
  - The signatures were verified against the specified public key
{"payloadType":"application/vnd.in-toto+json","payload":"eyJfdHlwZSI6Imh0dHBzOi8vaW4tdG90by5pby9TdGF0ZW1lbnQvdjAuMSIsInByZWRpY2F0ZVR5cGUiOiJodHRwczovL3Nsc2EuZGV2L3Byb3ZlbmFuY2UvdjAuMiIsInN1YmplY3QiOlt7Im5hbWUiOiJnaGNyLmlvL3hpZXFsL2thbmlrby1jaGFpbnMiLCJkaWdlc3QiOnsic2hhMjU2IjoiYTRlMWZiM2UxMWYzYzBhZDE2N2VkOTg2OGI3YzZmY2ZmZmQ3OTIzYTYxZThiZDE1ZmJmZGY4Y2RhMTA5Y2I1OCJ9fV0sInByZWRpY2F0ZSI6eyJidWlsZGVyIjp7ImlkIjoiaHR0cHM6Ly90ZWt0b24uZGV2L2NoYWlucy92MiJ9LCJidWlsZFR5cGUiOiJ0ZWt0b24uZGV2L3YxYmV0YTEvVGFza1J1biIsImludm9jYXRpb24iOnsiY29uZmlnU291cmNlIjp7fSwicGFyYW1ldGVycyI6eyJCVUlMREVSX0lNQUdFIjoiZ2NyLmlvL2thbmlrby1wcm9qZWN0L2V4ZWN1dG9yOnYxLjUuMUBzaGEyNTY6YzYxNjY3MTdmN2ZlMGI3ZGE0NDkwOGM5ODYxMzdlY2ZlYWIyMWYzMWVjMzk5MmY2ZTEyOGZmZjhhOTRiZThhNSIsIkNPTlRFWFQiOiIuLyIsIkRPQ0tFUkZJTEUiOiIuL0RvY2tlcmZpbGUiLCJFWFRSQV9BUkdTIjoiIiwiSU1BR0UiOiJnaGNyLmlvL3hpZXFsL2thbmlrby1jaGFpbnMifSwiZW52aXJvbm1lbnQiOnsiYW5ub3RhdGlvbnMiOnsicGlwZWxpbmUudGVrdG9uLmRldi9yZWxlYXNlIjoiMzA1NDBmYyJ9LCJsYWJlbHMiOnsiYXBwLmt1YmVybmV0ZXMuaW8vbWFuYWdlZC1ieSI6InRla3Rvbi1waXBlbGluZXMiLCJ0ZWt0b24uZGV2L3Rhc2siOiJrYW5pa28tY2hhaW5zIn19fSwiYnVpbGRDb25maWciOnsic3RlcHMiOlt7ImVudHJ5UG9pbnQiOiJzZXQgLWVcbmVjaG8gXCJGUk9NIGFscGluZUBzaGEyNTY6NjllNzBhNzlmMmQ0MWFiNWQ2MzdkZTk4YzFlMGIwNTUyMDZiYTQwYTgxNDVlN2JkZGI1NWNjYzA0ZTEzY2Y4ZlwiIHwgdGVlIC4vRG9ja2VyZmlsZVxuIiwiYXJndW1lbnRzIjpudWxsLCJlbnZpcm9ubWVudCI6eyJjb250YWluZXIiOiJhZGQtZG9ja2VyZmlsZSIsImltYWdlIjoib2NpOi8vZG9ja2VyLmlvL2xpYnJhcnkvYmFzaEBzaGEyNTY6OWUyMWJiNGUzNzUzYWZlODk5ZjJmY2ZmODZmZmUxOGU4Mjg0M2Q4Y2JlZTA2NDc2MzVmM2NiMDE3MTVhY2E1YiJ9LCJhbm5vdGF0aW9ucyI6bnVsbH0seyJlbnRyeVBvaW50IjoiIiwiYXJndW1lbnRzIjpbIiIsIi0tZG9ja2VyZmlsZT0uL0RvY2tlcmZpbGUiLCItLWNvbnRleHQ9L3dvcmtzcGFjZS9zb3VyY2UvLi8iLCItLWRlc3RpbmF0aW9uPWdoY3IuaW8veGllcWwva2FuaWtvLWNoYWlucyIsIi0tZGlnZXN0LWZpbGU9L3Rla3Rvbi9yZXN1bHRzL0lNQUdFX0RJR0VTVCJdLCJlbnZpcm9ubWVudCI6eyJjb250YWluZXIiOiJidWlsZC1hbmQtcHVzaCIsImltYWdlIjoib2NpOi8vZ2NyLmlvL2thbmlrby1wcm9qZWN0L2V4ZWN1dG9yQHNoYTI1NjpjNjE2NjcxN2Y3ZmUwYjdkYTQ0OTA4Yzk4NjEzN2VjZmVhYjIxZjMxZWMzOTkyZjZlMTI4ZmZmOGE5NGJlOGE1In0sImFubm90YXRpb25zIjpudWxsfSx7ImVudHJ5UG9pbnQiOiJzZXQgLWVcbmVjaG8gZ2hjci5pby94aWVxbC9rYW5pa28tY2hhaW5zIHwgdGVlIC90ZWt0b24vcmVzdWx0cy9JTUFHRV9VUkxcbiIsImFyZ3VtZW50cyI6bnVsbCwiZW52aXJvbm1lbnQiOnsiY29udGFpbmVyIjoid3JpdGUtdXJsIiwiaW1hZ2UiOiJvY2k6Ly9kb2NrZXIuaW8vbGlicmFyeS9iYXNoQHNoYTI1Njo5ZTIxYmI0ZTM3NTNhZmU4OTlmMmZjZmY4NmZmZTE4ZTgyODQzZDhjYmVlMDY0NzYzNWYzY2IwMTcxNWFjYTViIn0sImFubm90YXRpb25zIjpudWxsfV19LCJtZXRhZGF0YSI6eyJidWlsZFN0YXJ0ZWRPbiI6IjIwMjMtMTItMThUMDc6MzE6MTJaIiwiYnVpbGRGaW5pc2hlZE9uIjoiMjAyMy0xMi0xOFQwNzozMToyOFoiLCJjb21wbGV0ZW5lc3MiOnsicGFyYW1ldGVycyI6ZmFsc2UsImVudmlyb25tZW50IjpmYWxzZSwibWF0ZXJpYWxzIjpmYWxzZX0sInJlcHJvZHVjaWJsZSI6ZmFsc2V9LCJtYXRlcmlhbHMiOlt7InVyaSI6Im9jaTovL2RvY2tlci5pby9saWJyYXJ5L2Jhc2giLCJkaWdlc3QiOnsic2hhMjU2IjoiOWUyMWJiNGUzNzUzYWZlODk5ZjJmY2ZmODZmZmUxOGU4Mjg0M2Q4Y2JlZTA2NDc2MzVmM2NiMDE3MTVhY2E1YiJ9fSx7InVyaSI6Im9jaTovL2djci5pby9rYW5pa28tcHJvamVjdC9leGVjdXRvciIsImRpZ2VzdCI6eyJzaGEyNTYiOiJjNjE2NjcxN2Y3ZmUwYjdkYTQ0OTA4Yzk4NjEzN2VjZmVhYjIxZjMxZWMzOTkyZjZlMTI4ZmZmOGE5NGJlOGE1In19XX19","signatures":[{"keyid":"SHA256:c7r0wGda29N/7MedQV3skMFusKa4uZO2hTTXTp+RkWI","sig":"MEQCICn0QNAxGUsX3S4DOAqJNYmApnvo6OHoOxoKFdlU+WyvAiBZJMH2zGVpb/11swpr5vDoJxFhVzvjDWngmc6+ILoL4g=="}]}
```


如何查看这个 payload 的内容

赋值这个payload 的 经过base64编码的json字符串，通过如下方式解析。
```
echo 'eyJfdHlwZSI6Imh0dHBzOi8vaW4tdG90by5pby9TdGF0ZW1lbnQvdjAuMSIsInByZWRpY2F0ZVR5cGUiOiJodHRwczovL3Nsc2EuZGV2L3Byb3ZlbmFuY2UvdjAuMiIsInN1YmplY3QiOlt7Im5hbWUiOiJnaGNyLmlvL3hpZXFsL2thbmlrby1jaGFpbnMiLCJkaWdlc3QiOnsic2hhMjU2IjoiYTRlMWZiM2UxMWYzYzBhZDE2N2VkOTg2OGI3YzZmY2ZmZmQ3OTIzYTYxZThiZDE1ZmJmZGY4Y2RhMTA5Y2I1OCJ9fV0sInByZWRpY2F0ZSI6eyJidWlsZGVyIjp7ImlkIjoiaHR0cHM6Ly90ZWt0b24uZGV2L2NoYWlucy92MiJ9LCJidWlsZFR5cGUiOiJ0ZWt0b24uZGV2L3YxYmV0YTEvVGFza1J1biIsImludm9jYXRpb24iOnsiY29uZmlnU291cmNlIjp7fSwicGFyYW1ldGVycyI6eyJCVUlMREVSX0lNQUdFIjoiZ2NyLmlvL2thbmlrby1wcm9qZWN0L2V4ZWN1dG9yOnYxLjUuMUBzaGEyNTY6YzYxNjY3MTdmN2ZlMGI3ZGE0NDkwOGM5ODYxMzdlY2ZlYWIyMWYzMWVjMzk5MmY2ZTEyOGZmZjhhOTRiZThhNSIsIkNPTlRFWFQiOiIuLyIsIkRPQ0tFUkZJTEUiOiIuL0RvY2tlcmZpbGUiLCJFWFRSQV9BUkdTIjoiIiwiSU1BR0UiOiJnaGNyLmlvL3hpZXFsL2thbmlrby1jaGFpbnMifSwiZW52aXJvbm1lbnQiOnsiYW5ub3RhdGlvbnMiOnsicGlwZWxpbmUudGVrdG9uLmRldi9yZWxlYXNlIjoiMzA1NDBmYyJ9LCJsYWJlbHMiOnsiYXBwLmt1YmVybmV0ZXMuaW8vbWFuYWdlZC1ieSI6InRla3Rvbi1waXBlbGluZXMiLCJ0ZWt0b24uZGV2L3Rhc2siOiJrYW5pa28tY2hhaW5zIn19fSwiYnVpbGRDb25maWciOnsic3RlcHMiOlt7ImVudHJ5UG9pbnQiOiJzZXQgLWVcbmVjaG8gXCJGUk9NIGFscGluZUBzaGEyNTY6NjllNzBhNzlmMmQ0MWFiNWQ2MzdkZTk4YzFlMGIwNTUyMDZiYTQwYTgxNDVlN2JkZGI1NWNjYzA0ZTEzY2Y4ZlwiIHwgdGVlIC4vRG9ja2VyZmlsZVxuIiwiYXJndW1lbnRzIjpudWxsLCJlbnZpcm9ubWVudCI6eyJjb250YWluZXIiOiJhZGQtZG9ja2VyZmlsZSIsImltYWdlIjoib2NpOi8vZG9ja2VyLmlvL2xpYnJhcnkvYmFzaEBzaGEyNTY6OWUyMWJiNGUzNzUzYWZlODk5ZjJmY2ZmODZmZmUxOGU4Mjg0M2Q4Y2JlZTA2NDc2MzVmM2NiMDE3MTVhY2E1YiJ9LCJhbm5vdGF0aW9ucyI6bnVsbH0seyJlbnRyeVBvaW50IjoiIiwiYXJndW1lbnRzIjpbIiIsIi0tZG9ja2VyZmlsZT0uL0RvY2tlcmZpbGUiLCItLWNvbnRleHQ9L3dvcmtzcGFjZS9zb3VyY2UvLi8iLCItLWRlc3RpbmF0aW9uPWdoY3IuaW8veGllcWwva2FuaWtvLWNoYWlucyIsIi0tZGlnZXN0LWZpbGU9L3Rla3Rvbi9yZXN1bHRzL0lNQUdFX0RJR0VTVCJdLCJlbnZpcm9ubWVudCI6eyJjb250YWluZXIiOiJidWlsZC1hbmQtcHVzaCIsImltYWdlIjoib2NpOi8vZ2NyLmlvL2thbmlrby1wcm9qZWN0L2V4ZWN1dG9yQHNoYTI1NjpjNjE2NjcxN2Y3ZmUwYjdkYTQ0OTA4Yzk4NjEzN2VjZmVhYjIxZjMxZWMzOTkyZjZlMTI4ZmZmOGE5NGJlOGE1In0sImFubm90YXRpb25zIjpudWxsfSx7ImVudHJ5UG9pbnQiOiJzZXQgLWVcbmVjaG8gZ2hjci5pby94aWVxbC9rYW5pa28tY2hhaW5zIHwgdGVlIC90ZWt0b24vcmVzdWx0cy9JTUFHRV9VUkxcbiIsImFyZ3VtZW50cyI6bnVsbCwiZW52aXJvbm1lbnQiOnsiY29udGFpbmVyIjoid3JpdGUtdXJsIiwiaW1hZ2UiOiJvY2k6Ly9kb2NrZXIuaW8vbGlicmFyeS9iYXNoQHNoYTI1Njo5ZTIxYmI0ZTM3NTNhZmU4OTlmMmZjZmY4NmZmZTE4ZTgyODQzZDhjYmVlMDY0NzYzNWYzY2IwMTcxNWFjYTViIn0sImFubm90YXRpb25zIjpudWxsfV19LCJtZXRhZGF0YSI6eyJidWlsZFN0YXJ0ZWRPbiI6IjIwMjMtMTItMThUMDc6MzE6MTJaIiwiYnVpbGRGaW5pc2hlZE9uIjoiMjAyMy0xMi0xOFQwNzozMToyOFoiLCJjb21wbGV0ZW5lc3MiOnsicGFyYW1ldGVycyI6ZmFsc2UsImVudmlyb25tZW50IjpmYWxzZSwibWF0ZXJpYWxzIjpmYWxzZX0sInJlcHJvZHVjaWJsZSI6ZmFsc2V9LCJtYXRlcmlhbHMiOlt7InVyaSI6Im9jaTovL2RvY2tlci5pby9saWJyYXJ5L2Jhc2giLCJkaWdlc3QiOnsic2hhMjU2IjoiOWUyMWJiNGUzNzUzYWZlODk5ZjJmY2ZmODZmZmUxOGU4Mjg0M2Q4Y2JlZTA2NDc2MzVmM2NiMDE3MTVhY2E1YiJ9fSx7InVyaSI6Im9jaTovL2djci5pby9rYW5pa28tcHJvamVjdC9leGVjdXRvciIsImRpZ2VzdCI6eyJzaGEyNTYiOiJjNjE2NjcxN2Y3ZmUwYjdkYTQ0OTA4Yzk4NjEzN2VjZmVhYjIxZjMxZWMzOTkyZjZlMTI4ZmZmOGE5NGJlOGE1In19XX19' | base64 --decode | jq
```

得到的结果如下

这个 payload 包含了关于镜像 ghcr.io/xieql/kaniko-chains 的构建过程的详细信息，如使用的构建器（Tekton Chains）、构建步骤、使用的环境和参数，以及构建的起始和结束时间。

这些内容对于实现 SLSA（Supply chain Levels for Software Artifacts）安全标准非常重要，
因为它们提供了完整的构建透明度，确保了构建过程的可追溯性和可审计性，从而增强了软件供应链的安全性。通过这些详细信息，可以验证构建过程的完整性和一致性，从而提高对软件构建和部署过程的信任。

```
{
  "_type": "https://in-toto.io/Statement/v0.1",
  "predicateType": "https://slsa.dev/provenance/v0.2",
  "subject": [
    {
      "name": "ghcr.io/xieql/kaniko-chains",
      "digest": {
        "sha256": "a4e1fb3e11f3c0ad167ed9868b7c6fcfffd7923a61e8bd15fbfdf8cda109cb58"
      }
    }
  ],
  "predicate": {
    "builder": {
      "id": "https://tekton.dev/chains/v2"
    },
    "buildType": "tekton.dev/v1beta1/TaskRun",
    "invocation": {
      "configSource": {},
      "parameters": {
        "BUILDER_IMAGE": "gcr.io/kaniko-project/executor:v1.5.1@sha256:c6166717f7fe0b7da44908c986137ecfeab21f31ec3992f6e128fff8a94be8a5",
        "CONTEXT": "./",
        "DOCKERFILE": "./Dockerfile",
        "EXTRA_ARGS": "",
        "IMAGE": "ghcr.io/xieql/kaniko-chains"
      },
      "environment": {
        "annotations": {
          "pipeline.tekton.dev/release": "30540fc"
        },
        "labels": {
          "app.kubernetes.io/managed-by": "tekton-pipelines",
          "tekton.dev/task": "kaniko-chains"
        }
      }
    },
    "buildConfig": {
      "steps": [
        {
          "entryPoint": "set -e\necho \"FROM alpine@sha256:69e70a79f2d41ab5d637de98c1e0b055206ba40a8145e7bddb55ccc04e13cf8f\" | tee ./Dockerfile\n",
          "arguments": null,
          "environment": {
            "container": "add-dockerfile",
            "image": "oci://docker.io/library/bash@sha256:9e21bb4e3753afe899f2fcff86ffe18e82843d8cbee0647635f3cb01715aca5b"
          },
          "annotations": null
        },
        {
          "entryPoint": "",
          "arguments": [
            "",
            "--dockerfile=./Dockerfile",
            "--context=/workspace/source/./",
            "--destination=ghcr.io/xieql/kaniko-chains",
            "--digest-file=/tekton/results/IMAGE_DIGEST"
          ],
          "environment": {
            "container": "build-and-push",
            "image": "oci://gcr.io/kaniko-project/executor@sha256:c6166717f7fe0b7da44908c986137ecfeab21f31ec3992f6e128fff8a94be8a5"
          },
          "annotations": null
        },
        {
          "entryPoint": "set -e\necho ghcr.io/xieql/kaniko-chains | tee /tekton/results/IMAGE_URL\n",
          "arguments": null,
          "environment": {
            "container": "write-url",
            "image": "oci://docker.io/library/bash@sha256:9e21bb4e3753afe899f2fcff86ffe18e82843d8cbee0647635f3cb01715aca5b"
          },
          "annotations": null
        }
      ]
    },
    "metadata": {
      "buildStartedOn": "2023-12-18T07:31:12Z",
      "buildFinishedOn": "2023-12-18T07:31:28Z",
      "completeness": {
        "parameters": false,
        "environment": false,
        "materials": false
      },
      "reproducible": false
    },
    "materials": [
      {
        "uri": "oci://docker.io/library/bash",
        "digest": {
          "sha256": "9e21bb4e3753afe899f2fcff86ffe18e82843d8cbee0647635f3cb01715aca5b"
        }
      },
      {
        "uri": "oci://gcr.io/kaniko-project/executor",
        "digest": {
          "sha256": "c6166717f7fe0b7da44908c986137ecfeab21f31ec3992f6e128fff8a94be8a5"
        }
      }
    ]
  }
}
```


## clean up

删除ns
```
k delete ns chain-test
```

删除cosign密钥
```
kubectl delete secret signing-secrets -n tekton-chains
```

登出 docker

```
docker logout ghcr.io 
```
