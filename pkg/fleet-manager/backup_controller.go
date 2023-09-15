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
	"fmt"
	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
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
// 结论：轮询收集

// 问题2  删除的时候， 各个资源上面的 backup 资源也要跟着删除，不然下次就没法创建同命文件了，
// 思路：是的，所以创建的时候要加上 label，这样删除的时候把当前backup label的删除就行了

// 问题3 对 参数 转换的方法 进行测试， 这是本特性的核心工作
// 结论 ok

// 问题4  destination 的问题，
// 结论，fleet 为 必填项目， 这样就不需要 webhook 检查destination， 但是 其 fleet 的ns 唯一还是要做检查的。

// 问题5： 设置了集群，但是集群不在 fleet 中如何处理， 有的存在，有的不存在的情况下如何处理。
// 结论，只要存在 有一个集群设置出错，就应该报错，然后返回报错信息。

// new
// 问题6 可以一个 controller  控制多个 资源么，
// 结论： 当前的模板中是不行的，因为 	req ctrl.Request 只包含了 types.NamespacedName 信息，没有包含 kind 信息。 并且从controller 模型上，

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
	var currentObject client.Object
	var crdType string
	counter := 0

	log := ctrl.LoggerFrom(ctx)

	// Define the list of CRDs that this reconciler is responsible for
	crds := []struct {
		obj     client.Object
		typeStr string
	}{
		{&backupapi.Backup{}, "backup"},
		{&backupapi.Restore{}, "restore"},
		{&backupapi.Migrate{}, "migrate"},
	}

	// Loop through each CRD type and try to get the object with the given namespace and name
	for _, crd := range crds {
		err := b.Get(ctx, req.NamespacedName, crd.obj)
		if err == nil {
			counter++
			currentObject = crd.obj
			crdType = crd.typeStr
		} else if !apierrors.IsNotFound(err) {
			log.Error(err, fmt.Sprintf("Failed to get %s", crd.typeStr), "namespace", req.Namespace, "name", req.Name)
			return ctrl.Result{}, err
		}
	}

	// Check if multiple instances of CRDs (backup, restore, migrate) with the same namespace and name were found.
	// This scenario is prohibited in our current design as it could lead to ambiguous behavior during the reconciliation process.
	if counter > 1 {
		log.Error(nil, "Multiple CRDs(backup, restore, migrate) with the same namespace and name were found", "namespace", req.Namespace, "name", req.Name)
		return ctrl.Result{}, nil
	}

	// Check if no instances of the CRDs (backup, restore, migrate) were found with the specified namespace and name.
	if counter == 0 {
		log.Info("No CRD instance found for the given namespace and name", "namespace", req.Namespace, "name", req.Name)
		return ctrl.Result{}, nil
	}

	// Initialize patch helper
	patchHelper, err := patch.NewHelper(currentObject, b.Client)
	if err != nil {
		log.Error(err, "failed to init patch helper")
	}
	// Setup deferred function to handle patching the object at the end of the reconciler
	defer func() {
		patchOpts := []patch.Option{}
		if err := patchHelper.Patch(ctx, currentObject, patchOpts...); err != nil {
			reterr = utilerrors.NewAggregate([]error{reterr, errors.Wrapf(err, "failed to patch %s  %s", crdType, req.NamespacedName)})
		}
	}()

	// Check and add finalizer if not present
	if !controllerutil.ContainsFinalizer(currentObject, BackupFinalizer) {
		controllerutil.AddFinalizer(currentObject, BackupFinalizer)
		return ctrl.Result{}, nil
	}

	// Handle deletion timestamp being set, which indicates the object is being deleted
	if currentObject.GetDeletionTimestamp() != nil {
		switch crdType {
		case "backup":
			return b.reconcileDeleteBackup(ctx, currentObject.(*backupapi.Backup))
		case "restore":
			return b.reconcileDeleteRestore(ctx, currentObject.(*backupapi.Restore))
		case "migrate":
			return b.reconcileDeleteMigrate(ctx, currentObject.(*backupapi.Migrate))
		}
	}

	// Handle the main reconcile logic based on the type of the CRD object found
	switch crdType {
	case "backup":
		return b.reconcileBackup(ctx, currentObject.(*backupapi.Backup))
	case "restore":
		return b.reconcileRestore(ctx, currentObject.(*backupapi.Restore))
	case "migrate":
		return b.reconcileMigrate(ctx, currentObject.(*backupapi.Migrate))
	}

	log.Error(errors.New("unreachable code reached"), "This should not happen")
	return ctrl.Result{}, nil
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
