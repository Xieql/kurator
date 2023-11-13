## 创建 GitHub 的 webhook-secret

1 获取 PAT (webhook-secret 中的 token 字段）

github  个人头像 -》 setting -》 developer settings -》 personal access tokens -》 Tokens（classic） -》
generate new token -》 generate new token（classic） -》 sign in with authentication code
-> New personal access token (classic) ,make sue "**public_repo** Access public repositories" and "**admin:repo_hook** Full control of repository hooks" is selected
-》 click generate token-》 copy the token


2 获取一个随机数来 (webhook-secret 中的 secret字段）

```
openssl rand -hex 20
```

该随机数的作用：
当GitHub向指定的URL发送webhook请求时，它会使用这个secret值来生成一个签名，并将这个签名包含在请求的头部。服务器可以使用同一个secret来验证这个签名，以确保请求确实是从GitHub发出的。这增加了一个安全层，可以防止伪造或篡改的webhook请求。

3 创建 webhook-secret
```
kubectl create ns kurator-pipeline
kubectl create secret generic webhook-secret \
  --namespace=kurator-pipeline \
  --from-literal=token=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 \
  --from-literal=secret=505d4eff979cf04f57c84b936992048d568e296a
```

其中的token为前面已经创建好的 token
而 secret的值为前面的随机数。
