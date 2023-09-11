/*
Copyright Kurator Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package fleet

import (
	"context"
	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/klog/v2"
	"sigs.k8s.io/cluster-api/util/patch"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	backupapi "kurator.dev/kurator/pkg/apis/backups/v1alpha1"
	fleetapi "kurator.dev/kurator/pkg/apis/fleet/v1alpha1"
)

// 应该早一点吧 plugin 安装的 文档写好，不然自己都忘记了

// 问题1 ： 如何监控其他集群的资源，是否需要做特殊的配置。
// 思路： 看 karmada 的 代码

// 问题2  删除的时候， 各个资源上面的 backup 资源也要跟着删除，不然下次就没法创建同命文件了

// 问题3 对 参数 转换的方法 进行测试， 这是本特性的核心工作

// 问题4  destination 的问题， fleet 与 指定集群如果都设置了，只需要处理fleet

// TODO： 尽管已经api说明中提醒了，但是 还是需要有 webhook 提示下
// 会报NewBadRequest 但是体验不好
//var allWarnings []string
//if in.Spec.Destination != nil && in.Spec.Destination.Fleet != "" && len(in.Spec.Destination.Clusters) > 0 {
//allWarnings = append(allWarnings, "Both Fleet and Clusters are set. Fleet will take precedence over Clusters.")
//}
//if len(allWarnings) > 0 {
//return apierrors.NewBadRequest(fmt.Sprintf("Warnings: %v", allWarnings))
//}
// 会报 event：
//type ApplicationWebhook struct {
//	Client        client.Reader
//	EventRecorder record.EventRecorder
//}

//func (wh *ApplicationWebhook) SetupWebhookWithManager(mgr ctrl.Manager) error {
//	wh.EventRecorder = mgr.GetEventRecorderFor("application-webhook")
//	return ctrl.NewWebhookManagedBy(mgr).
//		For(&v1alpha1.Application{}).
//		WithValidator(wh).
//		Complete()
//}

//if in.Spec.Destination != nil && in.Spec.Destination.Fleet != "" && len(in.Spec.Destination.Clusters) > 0 {
//	wh.EventRecorder.Event(in, corev1.EventTypeWarning, "ConfigurationWarning", "Both Fleet and Clusters are set. Fleet will take precedence over Clusters.")
//}
// 这样，当 Fleet 和 Clusters 同时被设置时，一个警告事件将被创建并记录在 Kubernetes 事件中，而不会影响用户的请求。用户可以通过检查事件来了解到这个警告。

// 问题5： 设置了集群，但是集群不在 fleet 中如何处理， 有的存在，有的不存在的情况下如何处理。

const (
	BackupLabel     = "backup.kurator.dev/backup-name"
	BackupKind      = "Backups"
	BackupFinalizer = "backup.kurator.dev"
)

// BackupManager reconciles a Backup object
type BackupManager struct {
	client.Client
	Scheme *runtime.Scheme
}

// SetupWithManager sets up the controller with the Manager.
func (b *BackupManager) SetupWithManager(ctx context.Context, mgr ctrl.Manager, options controller.Options) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&backupapi.Backup{}).
		WithOptions(options).
		Complete(b)
}

func (b *BackupManager) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, reterr error) {
	backup := &backupapi.Backup{}
	if err := b.Get(ctx, req.NamespacedName, backup); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, errors.Wrapf(err, "failed to get backup %s", req.NamespacedName)
	}

	log := ctrl.LoggerFrom(ctx)
	log = log.WithValues("backup", klog.KObj(backup))

	patchHelper, err := patch.NewHelper(backup, b.Client)
	if err != nil {
		return ctrl.Result{}, errors.Wrapf(err, "failed to init patch helper for backup %s", req.NamespacedName)
	}

	defer func() {
		patchOpts := []patch.Option{}
		if err := patchHelper.Patch(ctx, backup, patchOpts...); err != nil {
			reterr = utilerrors.NewAggregate([]error{reterr, errors.Wrapf(err, "failed to patch backup %s", req.NamespacedName)})
		}
	}()

	// Add finalizer if not exist to void the race condition.
	if !controllerutil.ContainsFinalizer(backup, BackupFinalizer) {
		controllerutil.AddFinalizer(backup, BackupFinalizer)
		return ctrl.Result{}, nil
	}

	// fetch destination clusters
	clusters, err := fetchDestinationCluster(ctx, b.Client, backup.Spec.Destination)

	// Handle deletion reconciliation loop.
	if backup.DeletionTimestamp != nil {
		return b.reconcileDelete(backup)
	}

	// Handle normal loop.
	return b.reconcile(ctx, backup, fleet)
}

func (b *BackupManager) reconcile(ctx context.Context, backup *backupapi.Backup, fleet *fleetapi.Fleet) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)

	// apply backup resource in target clusters
	result, err := b.reconcileBackupResources(ctx, backup, fleet)
	if err != nil {
		log.Error(err, "failed to reconcileBackupResources")
	}
	if err != nil || result.RequeueAfter > 0 {
		return result, err
	}

	if err := b.reconcileStatus(ctx, app); err != nil {
		log.Error(err, "failed to reconcile status")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// reconcileBackupResources handles the synchronization of resources associated with the current Backup resource.
// The associated resources are categorized as 'source' and 'policy'.
// 'source' could be one of gitRepo, helmRepo, or ociRepo while 'policy' can be either kustomizations or helmReleases.
// Any change in Backup configuration could potentially lead to creation, deletion, or modification of associated resources in the Kubernetes cluster.
func (b *BackupManager) reconcileBackupResources(ctx context.Context, backup *backupapi.Backup, fleet *fleetapi.Fleet) (ctrl.Result, error) {
	// Synchronize source resource based on backup configuration
	if result, err := b.syncSourceResource(ctx, backup); err != nil || result.RequeueAfter > 0 {
		return result, err
	}

	// Iterate over each policy in the backup's spec.SyncPolicy
	for index, policy := range backup.Spec.SyncPolicies {
		policyName := generatePolicyName(backup, index)
		// A policy has a fleet, and a fleet has many clusters. Therefore, a policy may need to create or update multiple kustomizations/helmReleases for each cluster.
		// Synchronize policy resource based on current backup, fleet, and policy configuration
		if result, err := b.syncPolicyResource(ctx, backup, fleet, policy, policyName); err != nil || result.RequeueAfter > 0 {
			return result, err
		}
	}
	return ctrl.Result{}, nil
}

// reconcileStatus updates the status of resources associated with the current Backup resource.
// It does this by fetching the current status of the source (either GitRepoKind or HelmRepoKind) and the sync policy from the API server,
// and updating the Backup's status to reflect these current statuses.
func (b *BackupManager) reconcileStatus(ctx context.Context, backup *backupapi.Backup) error {

	if err := b.reconcileSyncStatus(ctx, backup); err != nil {
		return err
	}

	return nil
}

// reconcileSyncStatus reconciles the sync status of the given backup by finding all Kustomizations and HelmReleases associated with it,
// and updating the sync status of each resource in the backup's SyncStatus field.
func (b *BackupManager) reconcileSyncStatus(ctx context.Context, backup *backupapi.Backup) error {
	var syncStatus []*backupapi.BackupSyncStatus

	// find all kustomization
	kustomizationList, err := b.getKustomizationList(ctx, backup)
	if err != nil {
		return nil
	}
	// sync all kustomization status
	for _, kustomization := range kustomizationList.Items {
		kustomizationStatus := &backupapi.BackupSyncStatus{
			Name:                kustomization.Name,
			KustomizationStatus: &kustomization.Status,
		}
		syncStatus = append(syncStatus, kustomizationStatus)
	}

	// find all helmRelease
	helmReleaseList, err := b.getHelmReleaseList(ctx, backup)
	if err != nil {
		return err
	}
	// sync all helmRelease status
	for _, helmRelease := range helmReleaseList.Items {
		helmReleaseStatus := &backupapi.BackupSyncStatus{
			Name:              helmRelease.Name,
			HelmReleaseStatus: &helmRelease.Status,
		}
		syncStatus = append(syncStatus, helmReleaseStatus)
	}

	backup.Status.SyncStatus = syncStatus
	return nil
}

// 删除的时候， 各个资源上面的 backup 资源也要跟着删除，不然下次就没法创建同命文件了
func (b *BackupManager) reconcileDelete(backup *backupapi.Backup) (ctrl.Result, error) {
	controllerutil.RemoveFinalizer(backup, BackupFinalizer)

	return ctrl.Result{}, nil
}

func (b *BackupManager) objectToBackupFunc(o client.Object) []ctrl.Request {
	labels := o.GetLabels()
	if labels[BackupLabel] != "" {
		return []ctrl.Request{
			{
				NamespacedName: types.NamespacedName{
					Namespace: o.GetNamespace(),
					Name:      labels[BackupLabel],
				},
			},
		}
	}

	return nil
}
