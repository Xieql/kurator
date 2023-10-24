---
title: "Kurator Unified Backup"
linkTitle: "Unified Backup with Kurator"
weight: 20
description: >
  A comprehensive guide on Kurator's Unified Backup solution, providing an overview and practical implementation steps.
---

Unified Backup offers a concise, one-click solution to back up resources across all clusters in Fleet, 
supporting varying granularities from individual namespaces to multiple namespaces, and up to entire clusters.
This ensures consistent data protection and facilitates swift recovery in a distributed cloud-native infrastructure.

## Introduction

### Backup Types

#### Scheduled Backup:

- **Use Case**: Regularly back up dynamic data to safeguard against accidental losses, maintain compliance, and enable efficient disaster recovery.
- **Functionality**: Set automated backups at specific intervals using cron expressions.

#### Immediate Backup:

- **Use Case**: Essential for sudden changes, like after a significant data update or before implementing system alterations.
- **Functionality**: Triggers a one-time backup on demand.

### Advanced Backup Options

- **Specific Cluster Backup within Fleet**: Users can choose a particular cluster within the Fleet for backup.
- **Resource Filtering**: Kurator offers filtering options for more precise backups, allowing users to define criteria based on attributes like name, namespace, or label.

> **Note**: Volume Snapshot support and advanced Velero features are not implemented currently. For detailed information, refer to the official Velero documentation.


## 如何使用kurator进行统一备份的实操

