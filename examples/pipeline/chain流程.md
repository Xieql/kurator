
## 官方途径基本流程

###  前置条件

### 创建测试ns
```
k create ns chain-test
```

### 创建加密所需密钥。
注意：该密钥名称固定，且内容无法直接变更。如果需要变更，需要先删除再创建。

```
cosign generate-key-pair k8s://tekton-chains/signing-secrets
```

执行过程中会要求输入2次密码，测试的时候可以直接输入两次空格

这个命令 会在 ns 创建一个 公钥私钥的密钥

并且会在当前本地路径创建一个 cosign.pub的公钥文件

### 配置 镜像仓库认证

```
docker login ghcr.io -u xieql -p xxx
```

登录成功后会有一个提示

```
WARNING! Your password will be stored unencrypted in /root/.docker/config.json.
```

/root/.docker/config.json 这个路径就是 docker 的认证信息，我们通过这个信息来创建secret

创建用于 task 上传image 到 oci仓库所需的 secret

```
kubectl create secret generic registry-credentials --from-file=/root/.docker/config.json -n chain-test
```

创建用于 chain controller 上传 sig 和 att 到 oci仓库所需的secret

```
kubectl create secret docker-registry chain-credentials \
  --docker-server=ghcr.io \
  --docker-username=xieql \
  --docker-email=xieqianglong@huawei.com \
  --docker-password=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 \
  -n chain-test
```

通过serviceaccount 把这个认证传给 chain controller
```
k apply -f examples/pipeline/test-rbac.yaml
```



### 配置 tekton chain 相关参数

```
kubectl patch configmap chains-config -n tekton-chains -p='{"data":{"artifacts.taskrun.format": "slsa/v1"}}'
kubectl patch configmap chains-config -n tekton-chains -p='{"data":{"artifacts.taskrun.storage": "oci"}}'
kubectl patch configmap chains-config -n tekton-chains -p='{"data":{"artifacts.oci.storage": "oci"}}'
kubectl patch configmap chains-config -n tekton-chains -p='{"data":{"transparency.enabled": "true"}}'
```

### 创建 kaniko 测试例子


#### apply task 例子
```
kubectl apply -f https://github.com/tektoncd/chains/raw/main/examples/kaniko/kaniko.yaml -n chain-test
```

#### 创建 task run


```
echo "
apiVersion: tekton.dev/v1beta1
kind: TaskRun
metadata:
  name: kaniko-run
  namespace: chain-test
spec:
  serviceAccountName: example
  taskRef:
    name: kaniko-chains
  params:
  - name: IMAGE
    value: ghcr.io/xieql/kaniko-chains
  workspaces:
  - name: source
    emptyDir: {}
  - name: dockerconfig
    secret:
      secretName: registry-credentials
" | kubectl apply -f - -n chain-test

```


查看 taskrun 状态：

```
k get taskruns.tekton.dev -n chain-test -o yaml 
## 可以看到 chains.tekton.dev/signed: "true" 说明签名成功
apiVersion: v1
items:
- apiVersion: tekton.dev/v1
  kind: TaskRun
  metadata:
    annotations:
      chains.tekton.dev/signed: "true"

```

### 验证签名 

登录 ghcr 查看，可以看到对应pkg下的镜像以及对应的 .sig 签名 和 .att 出处证明

通过如下方式验证签名：（在之前cosign 过程中创建的 cosign.pub，用来作为签名校验的公钥）

```
cosign verify --key cosign.pub ghcr.io/xieql/kaniko-chains
cosign verify-attestation --key cosign.pub --type slsaprovenance ghcr.io/xieql/kaniko-chains
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
删除不掉？？？
似乎没有看到如何删除相关资源

无法删除的时候重装：

```
kubectl delete --f https://storage.googleapis.com/tekton-releases/chains/latest/release.yaml
```

```
kubectl apply --f https://storage.googleapis.com/tekton-releases/chains/latest/release.yaml
```




删除cosign密钥
```
kubectl delete secret signing-secrets -n tekton-chains
```

登出 docker

```
docker logout ghcr.io 
```