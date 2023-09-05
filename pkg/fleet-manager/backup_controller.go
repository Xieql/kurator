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
	sourcev1beta2 "github.com/fluxcd/source-controller/api/v1beta2"
	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/klog/v2"
	applicationapi "kurator.dev/kurator/pkg/apis/apps/v1alpha1"
	backupapi "kurator.dev/kurator/pkg/apis/backups/v1alpha1"
	fleetapi "kurator.dev/kurator/pkg/apis/fleet/v1alpha1"
	"sigs.k8s.io/cluster-api/util/patch"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	BackupFinalizer = "backup.kurator.dev"
	//	GitRepoKind       = sourcev1beta2.GitRepositoryKind
	//	HelmRepoKind      = sourcev1beta2.HelmRepositoryKind
	//	OCIRepoKind       = sourcev1beta2.OCIRepositoryKind
	//	KustomizationKind = kustomizev1beta2.KustomizationKind
	//	HelmReleaseKind   = helmv2b1.HelmReleaseKind
	//
	//	ApplicationLabel     = "apps.kurator.dev/app-name"
	//	ApplicationKind      = "Application"
	//	ApplicationFinalizer = "apps.kurator.dev"

)

// BackupManager reconciles a Backup object
type BackupManager struct {
	client.Client
	Scheme *runtime.Scheme
}

// SetupWithManager sets up the controller with the Manager.
func (b *BackupManager) SetupWithManager(ctx context.Context, mgr ctrl.Manager, options controller.Options) error {
	// we just consider current status of fleet, and we will not watch fleet.
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
			reterr = utilerrors.NewAggregate([]error{reterr, errors.Wrapf(err, "failed to patch application %s", req.NamespacedName)})
		}
	}()

	// Add finalizer if not exist to void the race condition.
	if !controllerutil.ContainsFinalizer(backup, BackupFinalizer) {
		controllerutil.AddFinalizer(backup, BackupFinalizer)
		return ctrl.Result{}, nil
	}

	// the destination must have one cluster at lease. (什么情况下填写了destination 但是没有选中任何集群：1 selector 无法选中任何集群 + 2 指定集群的配置 无法找到任何集群， 2 应该要报错)
	destinationClusters := b.getDestinationClusters(backup)
	if len(destinationClusters) == 0 {
		return ctrl.Result{}, errors.Wrapf(err, "the destination must have one cluster at lease. ")
	}

	// Handle deletion reconciliation loop.
	if backup.DeletionTimestamp != nil {
		return b.reconcileDelete(app)
	}

	// Handle normal loop.
	return b.reconcile(ctx, app, fleet)
}

func (b *BackupManager) reconcile(ctx context.Context, app *applicationapi.Application, fleet *fleetapi.Fleet) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)

	result, err := a.reconcileApplicationResources(ctx, app, fleet)
	if err != nil {
		log.Error(err, "failed to reconcileSyncResources")
	}
	if err != nil || result.RequeueAfter > 0 {
		return result, err
	}

	if err := a.reconcileStatus(ctx, app); err != nil {
		log.Error(err, "failed to reconcile status")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// reconcileApplicationResources handles the synchronization of resources associated with the current Application resource.
// The associated resources are categorized as 'source' and 'policy'.
// 'source' could be one of gitRepo, helmRepo, or ociRepo while 'policy' can be either kustomizations or helmReleases.
// Any change in Application configuration could potentially lead to creation, deletion, or modification of associated resources in the Kubernetes cluster.
func (a *ApplicationManager) reconcileApplicationResources(ctx context.Context, app *applicationapi.Application, fleet *fleetapi.Fleet) (ctrl.Result, error) {
	// Synchronize source resource based on application configuration
	if result, err := a.syncSourceResource(ctx, app); err != nil || result.RequeueAfter > 0 {
		return result, err
	}

	// Iterate over each policy in the application's spec.SyncPolicy
	for index, policy := range app.Spec.SyncPolicies {
		policyName := generatePolicyName(app, index)
		// A policy has a fleet, and a fleet has many clusters. Therefore, a policy may need to create or update multiple kustomizations/helmReleases for each cluster.
		// Synchronize policy resource based on current application, fleet, and policy configuration
		if result, err := a.syncPolicyResource(ctx, app, fleet, policy, policyName); err != nil || result.RequeueAfter > 0 {
			return result, err
		}
	}
	return ctrl.Result{}, nil
}

// reconcileStatus updates the status of resources associated with the current Application resource.
// It does this by fetching the current status of the source (either GitRepoKind or HelmRepoKind) and the sync policy from the API server,
// and updating the Application's status to reflect these current statuses.
func (a *ApplicationManager) reconcileStatus(ctx context.Context, app *applicationapi.Application) error {
	if err := a.reconcileSourceStatus(ctx, app); err != nil {
		return err
	}

	if err := a.reconcileSyncStatus(ctx, app); err != nil {
		return err
	}

	return nil
}

// reconcileSourceStatus reconciles the source status of the given application by fetching the status of the source resource (e.g. GitRepository, HelmRepository)
func (a *ApplicationManager) reconcileSourceStatus(ctx context.Context, app *applicationapi.Application) error {
	log := ctrl.LoggerFrom(ctx)

	sourceKey := client.ObjectKey{
		Name:      generateSourceName(app),
		Namespace: app.GetNamespace(),
	}

	if app.Status.SourceStatus == nil {
		app.Status.SourceStatus = &applicationapi.ApplicationSourceStatus{}
	}

	sourceKind := findSourceKind(app)
	// Depending on source kind in application specifications, fetch resource status and update application's source status
	switch sourceKind {
	case GitRepoKind:
		currentResource := &sourcev1beta2.GitRepository{}
		err := a.Client.Get(ctx, sourceKey, currentResource)
		if err != nil && !apierrors.IsNotFound(err) {
			log.Error(err, "failed to get GitRepository from the API server when reconciling status")
			return err
		}
		// if not found, return directly. new created GitRepository will be watched in subsequent loop
		if apierrors.IsNotFound(err) {
			return nil
		}
		app.Status.SourceStatus.GitRepoStatus = &currentResource.Status

	case HelmRepoKind:
		currentResource := &sourcev1beta2.HelmRepository{}
		err := a.Client.Get(ctx, sourceKey, currentResource)
		if err != nil && !apierrors.IsNotFound(err) {
			log.Error(err, "failed to get HelmRepository from the API server when reconciling status")
			return err
		}
		// if not found, return directly. new created HelmRepository will be watched in subsequent loop
		if apierrors.IsNotFound(err) {
			return nil
		}
		app.Status.SourceStatus.HelmRepoStatus = &currentResource.Status
	}
	return nil
}

// reconcileSyncStatus reconciles the sync status of the given application by finding all Kustomizations and HelmReleases associated with it,
// and updating the sync status of each resource in the application's SyncStatus field.
func (a *ApplicationManager) reconcileSyncStatus(ctx context.Context, app *applicationapi.Application) error {
	var syncStatus []*applicationapi.ApplicationSyncStatus

	// find all kustomization
	kustomizationList, err := a.getKustomizationList(ctx, app)
	if err != nil {
		return nil
	}
	// sync all kustomization status
	for _, kustomization := range kustomizationList.Items {
		kustomizationStatus := &applicationapi.ApplicationSyncStatus{
			Name:                kustomization.Name,
			KustomizationStatus: &kustomization.Status,
		}
		syncStatus = append(syncStatus, kustomizationStatus)
	}

	// find all helmRelease
	helmReleaseList, err := a.getHelmReleaseList(ctx, app)
	if err != nil {
		return err
	}
	// sync all helmRelease status
	for _, helmRelease := range helmReleaseList.Items {
		helmReleaseStatus := &applicationapi.ApplicationSyncStatus{
			Name:              helmRelease.Name,
			HelmReleaseStatus: &helmRelease.Status,
		}
		syncStatus = append(syncStatus, helmReleaseStatus)
	}

	app.Status.SyncStatus = syncStatus
	return nil
}

func (a *ApplicationManager) reconcileDelete(app *applicationapi.Application) (ctrl.Result, error) {
	controllerutil.RemoveFinalizer(app, ApplicationFinalizer)

	return ctrl.Result{}, nil
}

func (a *ApplicationManager) objectToApplicationFunc(o client.Object) []ctrl.Request {
	labels := o.GetLabels()
	if labels[ApplicationLabel] != "" {
		return []ctrl.Request{
			{
				NamespacedName: types.NamespacedName{
					Namespace: o.GetNamespace(),
					Name:      labels[ApplicationLabel],
				},
			},
		}
	}

	return nil
}
