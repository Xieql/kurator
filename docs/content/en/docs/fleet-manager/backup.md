---
title: "Unified Backup, Restore, and Migration Feature Guide"
linkTitle: "Unified Backup, Restore, and Migration Feature Guide"
weight: 45
description: >
  The easiest way to manage multi cluster Unified Backup, Restore, and Migration with Fleet.
---

Kurator, an open-source distributed cloud-native platform, introduces a unified solution for backup, restore, and migration across multiple clusters in Fleet.
This feature, integrated with [Velero](https://velero.io/), simplifies the process of managing backups, restoring data, and migrating resources across clusters through a streamlined one-click operation.

Check out the key advantages below.

- **Simplified Multi-cluster Operations**: Streamlining management and operational tasks across various clusters for easier resource handling.

- **Backup Support with Scheduled and Immediate Options**: Automate regular backups for data protection and compliance, along with immediate backup options for on-demand needs.

- **One-stop, Flexible Disaster Recovery Solution**: Providing a robust and flexible solution for disaster recovery, allowing tailored recovery of specific resources in specific clusters to ensure operational continuity in adverse scenarios.

- **Effortless Cluster Resource Migration**: Facilitating smooth migration of resources across multiple clusters for workload balancing or transitions to new environments.

- **Unified Status View**: Offering a clear, unified view of resources and backup statuses across clusters, enhancing visibility and control.

Before proceeding, ensure you have configured the backup plugin as per the [Backup Plugin Configuration Guide](/docs/fleet-manager/backup-plugin) to avail the unified backup feature.

## Backup, Restore and Migration WorkFlow

Before diving into the specifics, let's take a moment to understand the workflow that underpins the backup, restore and migration processes within Kurator.

{{< image width="100%"
    link="./image/backup-workflow.svg"
    >}}

The diagram illustrates the following steps:

1. User Creates Custom Resource:

   The user creates a custom resource that defines the backup, restore, or migration requirements.

1. Kurator Watches Custom Resource:

   Kurator continuously monitors the custom resource for new configurations, ensuring real-time responsiveness.

1. Kurator Executes Backup in Source Cluster:

   Based on the configuration in the custom resource, Kurator initiates the backup process in the source cluster and capture the current state.

1. Backup Engine Uploads Cluster Resources from Source Cluster:
   
   The backup engine then uploads the filtered resources from the source cluster to the object storage.

1. Kurator Restores to Destination Clusters (Optional):

   If the workflow includes restoration, Kurator will initiate the restore process in the target destination clusters using the backup data and capture the current state.

1. Backup Engine Downloads Resources to Destination Cluster (Optional):

   The backup engine retrieves the filtered resources from the object storage and restores the resources to the target clusters, completing the restore or migration process.

With the above workflow, Kurator provides a streamlined and automated process to manage backup, restore, or migration tasks with minimal user intervention, ensuring data protection, disaster recovery, and smooth transitions between clusters.

### Note on Workflow Variations:

- **Backup**

  User initiates step 1 (defining backup configuration). Kurator automatically handles steps 2, 3, and 4. 
  This process supports backup across multiple clusters.

- **Backup and Restore**: 

  - Backup: User initiates step 1 (defining backup configuration), Kurator handles steps 2, 3, and 4 automatically.
  
  - Restore: After the completion of the backup, the user initiates step 1 again (defining restore configuration). Kurator automatically handles steps 5 and 6. 
    
    Please note, the restore operation will recover data to the original cluster where the backup was taken.

- **Migration**
  
  User initiates step 1 (defining migration configuration), Kurator handles steps 2 to 6 automatically.
  
  The migration process involves moving data from one source cluster to one or multiple target clusters.

## Create a Fleet with the Backup Plugin Enabled

### Use Minio


If you opt to use Minio, apply the following configuration:

```console
kubectl apply -f - <<EOF
apiVersion: fleet.kurator.dev/v1alpha1
kind: Fleet
metadata:
  name: quickstart
  namespace: default
spec:
  clusters:
    - name: kurator-member1
      kind: AttachedCluster
    - name: kurator-member2
      kind: AttachedCluster
  plugin:
    backup:
      storage:
        location:
          bucket: velero
          provider: aws
          endpoint: http://172.18.255.200:9000
          region: minio
        secretName: minio-credentials
EOF
```

> **Note**: The `endpoint`(`http://172.18.255.200:9000`) in `fleet-minio.yaml` is depends on your minio service ip, check your minio service ip in [Minio installation guide](/docs/setup/install-minio).

### Use OBS

If you choose to use OBS, apply the following configuration:

```console
kubectl apply -f - <<EOF
apiVersion: fleet.kurator.dev/v1alpha1
kind: Fleet
metadata:
  name: quickstart
  namespace: default
spec:
  clusters:
    - name: kurator-member1
      kind: AttachedCluster
    - name: kurator-member2
      kind: AttachedCluster
  plugin:
    backup:
      storage:
        location:
          bucket: kurator-obs
          provider: huaweicloud
          endpoint: http://obs.cn-south-1.myhuaweicloud.com
          region: cn-south-1
        secretName: obs-credentials
EOF
```

### Fleet Backup Plugin Configuration Explained

Let's delve into the `spec` section of the above Fleet:

- `clusters`: Contains the two `AttachedCluster` objects created earlier, indicating that the backup plugin will be installed on these two clusters.
  
- `plugin`: The `backup` indicates the description of a backup plugin. Currently, it only contains storage related configurations. For more configuration options, please refer to the [Fleet API](https://kurator.dev/docs/references/fleet-api/). 

   Within this storage configuration, users should ensure that the details in the location field are accurate. This information primarily differentiates the configurations in fleet-obs.yaml from fleet-minio.yaml. Additionally, the secretName refers to the name of the secret that was established earlier within the same namespace.

## Verify the Installation

To ensure that the backup plugin is successfully installed and running, follow the steps below:

### 1. Check Velero Pods

Run the following commands:

```console
kubectl get po -A -n velero --kubeconfig=/root/.kube/kurator-member1.config
kubectl get po -A -n velero --kubeconfig=/root/.kube/kurator-member2.config
```

Initially, you should observe:

```plaintext
velero               velero-velero-kurator-member1-upgrade-crds-bm28q        0/1     Init:0/1   0          31s
velero               velero-velero-kurator-member2-upgrade-crds-lg7gd        0/1     Init:0/1   0          28s
```

After waiting for about 2 or more minutes, check again to ensure all pods are in the `Running` state:

```plaintext
velero               node-agent-hn7h5                                        1/1     Running   0          85s
velero               velero-velero-kurator-member1-755d5675ff-sbrkg          1/1     Running   0          85s
velero               node-agent-2mrnj                                        1/1     Running   0          116s
velero               velero-velero-kurator-member2-c5b87598b-4zfsc           1/1     Running   0          116s
```

### 2. Confirm Connection to the Object Storage

Run the following commands to ensure backup has successfully connected to the object storage:

```console
kubectl get backupstoragelocations.velero.io -A --kubeconfig=/root/.kube/kurator-member1.config
kubectl get backupstoragelocations.velero.io -A --kubeconfig=/root/.kube/kurator-member2.config
```

When the `PHASE` for all is `Available`, it indicates that the object storage is accessible:

```plaintext
NAMESPACE   NAME      PHASE       LAST VALIDATED   AGE     DEFAULT
velero      default   Available   57s              8m23s   true
```

## Cleanup

This section guides you through the process of cleaning up the fleets and plugins.

### 1. Cleanup the Backup Plugin

If you only need to remove the backup plugin, simply edit the current fleet and remove the corresponding description:

```console
kubectl edit fleet.fleet.kurator.dev quickstart
```

To check the results of the deletion, you can observe that the Velero components have been removed:

```console
kubectl get po -A --kubeconfig=/root/.kube/kurator-member1.config
kubectl get po -A --kubeconfig=/root/.kube/kurator-member2.config
```

If you wish to reinstall the components later, you can simply edit the fleet and add the necessary configurations.

### 2. Cleanup the Fleet

When the fleet is deleted, all associated plugins will also be removed:

```console
kubectl delete fleet.fleet.kurator.dev quickstart
```
