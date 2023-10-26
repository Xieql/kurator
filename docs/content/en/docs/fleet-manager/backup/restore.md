---
title: "Unified Restore"
linkTitle: "Unified Restore"
weight: 30
description: >
  A comprehensive guide on Kurator's Unified Restore solution, detailing the methodology and steps for data recovery in a distributed cloud-native infrastructure.
---

Unified Restore provides an easy-to-follow, one-click solution to restore application and cluster resources across all clusters in Fleet, based on backups created using the Unified Backup mechanism. 
This guarantees seamless recovery, ensuring minimal downtime and maximum operational efficiency.

## Introduction

## Restore Overview

The Kurator Unified Restore functionality leverages a specified Unified Backup object as a reference point. 
While the Restore has its distinct configurations, the type of backup it references (Immediate or Scheduled) influences the restoration process, leading to the distinctions described below.

### Restore from an **Immediate Backup**

- **Use Case**: Responding to sudden data losses or application malfunctions.
- **Referred Backup**: The specific **Immediate Backup** specified by the user.
- **Restore Result**: Direct restoration from the chosen backup in chosen clusters.

### Restore from a **Scheduled Backup**

- **Use Case**: Restoring to a recent state in scenarios such as post-accidental data modifications, compliance verifications, or disaster recovery after unforeseen system failures.
- **Referred Backup**: When a **Scheduled Backup** is selected, Kurator will automatically target the latest successful backup within that series.
- **Restore Result**: Restoration from the most recent successful backup, ensuring data accuracy and relevance.

### Advanced Backup Options

#### Specific Cluster Restore within Fleet:

Users can specify clusters as the restore destination. 
However, these selected clusters must be a subset of those included in the backup.
This is because the restore process relies on the data from the backup.

**Note**: To restore resources from one cluster to another not in the original backup, utilize the Unified Migration feature, detailed in a later section.

#### Resource Filtering:
During the restore process, users can apply a secondary filter to the data from the backup, enabling selective restoration. 
Define the scope using attributes like backup name, namespace, or label to ensure only desired data is restored. For details, refer to the [Fleet API](https://kurator.dev/docs/references/fleet-api/#fleet)



## How to Perform a Unified Restore

### Pre-requisites

Before diving into the restore steps, ensure you have successfully installed the backup plugin as per the [backup plugin installation guide](/docs/fleet-manager/backup/backup-plugin)

Also， fleet 和 attachedecluster 也使用前面的guide 所定义的。

同样的，我们还需要安装 测试用的  deploying a busybox example:

```console
kubectl apply -f examples/backup/busybox.yaml --kubeconfig=/root/.kube/kurator-member1.config
kubectl apply -f examples/backup/busybox.yaml --kubeconfig=/root/.kube/kurator-member2.config
```

准备完毕后，后续的操作按照 创建备份，模拟灾难， 执行恢复， 查看 restore 对象字段介绍 的步骤呈现

在这些操作中间。 
用户可以通过下面的方式确认pod 情况， 比如验证 pod 初始情况，取人pod已经由于模拟灾难丢失，确认pod 由于 统一恢复得到敷衍。
```console
kubectl get po -n kurator-backup --kubeconfig=/root/.kube/kurator-member1.config
kubectl get po -n kurator-backup --kubeconfig=/root/.kube/kurator-member2.config
```

### 1. Restore from an Immediate Backup

#### 创建即时备份备份

```console
kubectl apply -f examples/backup/backup-minimal.yaml
```

#### 模拟灾难

```console
kubectl apply -f examples/backup/backup-minimal.yaml
```

Wait for the namespace to be deleted.

#### 执行统一恢复配置

```console
kubectl apply -f examples/backup/restore-specific-ns.yaml
```

#### 查看 restore 对象

```console
kubectl get backups.backup.kurator.dev schedule-matchlabels -o yaml
```

预期结果如下：

```console
（）
```

Review the results:

```console
kubectl get backups.backup.kurator.dev specific-ns -o yaml
```

Given the output provided, let's dive deeper to understand the various elements and their implications:

- In the spec, the `destination` field is used. By default, if no specific cluster is set, it points to all clusters within the `fleet`.
- The `policy` provides a unified strategy for the backup. The current setting of `resourceFilter` indicates the backup of all resources under the specified namespace `kurator-backup`. The policy used here only touches upon the namespace. For more advanced filtering options, refer to the [Fleet API](https://kurator.dev/docs/references/fleet-api/#fleet)
- The `status` section displays the actual processing status of the two clusters within the fleet.


### 2. Restore from a Scheduled Backup

Label the pod intended for backup to facilitate subsequent label-based selections:

```console
kubectl label po busybox env=test -n kurator-backup --kubeconfig=/root/.kube/kurator-member2.config
```

Apply the scheduled backup:

```console
kubectl apply -f examples/backup/backup-schedule.yaml
```

Review the results:

```console
kubectl get backups.backup.kurator.dev schedule-matchlabels -o yaml
```

The expected result should be:

```console
apiVersion: backup.kurator.dev/v1alpha1
kind: Backup
metadata:
  annotations:
    kubectl.kubernetes.io/last-applied-configuration: |
      {"apiVersion":"backup.kurator.dev/v1alpha1","kind":"Backup","metadata":{"annotations":{},"name":"schedule-matchlabels","namespace":"default"},"spec":{"destination":{"clusters":[{"kind":"AttachedCluster","name":"kurator-member2"}],"fleet":"quickstart"},"policy":{"resourceFilter":{"labelSelector":{"matchLabels":{"env":"test"}}},"ttl":"240h"},"schedule":"0 0 * * *"}}
  creationTimestamp: "2023-10-25T02:55:30Z"
  finalizers:
  - backup.kurator.dev
  generation: 1
  name: schedule-matchlabels
  namespace: default
  resourceVersion: "8606170"
  uid: 4ac4eb1c-e2cf-48a2-b197-87397d72222a
spec:
  destination:
    clusters:
    - kind: AttachedCluster
      name: kurator-member2
    fleet: quickstart
  policy:
    resourceFilter:
      labelSelector:
        matchLabels:
          env: test
    ttl: 240h
  schedule: 0 0 * * *
status:
  backupDetails:
  - backupStatusInCluster: {}
    clusterKind: AttachedCluster
    clusterName: kurator-member2
```

Analyzing the provided output, let's dissect its sections for a clearer comprehension:

- **Cron Expression in `spec`**:
    - The `schedule` field within the `spec` uses a cron expression. This defines when a backup is to be performed.
    - Once set, the backup won't be executed immediately. Instead, it waits until the time specified by the cron expression.

- **Destination in `spec`**:
    - The `destination` field under `spec` points to `kurator-member2`. This means the backup is specifically for this cluster within its fleet.

- **Policy in `spec`**:
    - The `policy` section outlines the backup strategy. In this instance, it's set to backup resources that match certain labels.
    - For more advanced filtering options, refer to the [Fleet API](https://kurator.dev/docs/references/fleet-api/#fleet)

- **Status Section**:
    - The `status` section provides an overview of the backup status across clusters.
    - At the moment, it's empty. As backups are executed according to the cron schedule, this section will populate with relevant details.

### Cleanup

To remove the backup examples used for testing, execute:

```console
kubectl delete backups.backup.kurator.dev specific-ns schedule-matchlabels
```

> Please note: This command only deletes the current object in the k8s API.
For data security, deletion of the data in object storage should be performed using the tools provided by the object storage solution you adopted.
Kurator currently does not offer capabilities to delete data inside the object storage.