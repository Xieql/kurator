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
	"strings"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	velerov1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"sigs.k8s.io/cluster-api/util/patch"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	backupapi "kurator.dev/kurator/pkg/apis/backups/v1alpha1"
)

// RestoreManager reconciles a Restore object
type RestoreManager struct {
	client.Client
	Scheme *runtime.Scheme
	logger logr.Logger
}

// SetupWithManager sets up the controller with the Manager.
func (r *RestoreManager) SetupWithManager(ctx context.Context, mgr ctrl.Manager, options controller.Options) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&backupapi.Restore{}).
		WithOptions(options).
		Complete(r)
}

func (r *RestoreManager) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, reterr error) {
	r.logger = ctrl.LoggerFrom(ctx).WithValues("restore", req.NamespacedName)

	restore := &backupapi.Restore{}

	if err := r.Client.Get(ctx, req.NamespacedName, restore); err != nil {
		if apierrors.IsNotFound(err) {
			r.logger.Info("restore is not exist", "restore", req)
			return ctrl.Result{}, nil
		}

		// Error reading the object - requeue the request.
		return ctrl.Result{}, err
	}

	// Initialize patch helper
	patchHelper, err := patch.NewHelper(restore, r.Client)
	if err != nil {
		r.logger.Error(err, "failed to init patch helper")
	}
	// Setup deferred function to handle patching the object at the end of the reconciler
	defer func() {
		if err := patchHelper.Patch(ctx, restore); err != nil {
			reterr = utilerrors.NewAggregate([]error{reterr, errors.Wrapf(err, "failed to patch %s  %s", restore.Name, req.NamespacedName)})
		}
	}()

	// Check and add finalizer if not present
	if !controllerutil.ContainsFinalizer(restore, RestoreFinalizer) {
		controllerutil.AddFinalizer(restore, RestoreFinalizer)
	}

	// Handle deletion
	if restore.GetDeletionTimestamp() != nil {
		return r.reconcileDeleteRestore(ctx, restore)
	}

	// Handle the main reconcile logic
	return r.reconcileRestore(ctx, restore)
}

// reconcileRestore handles the main reconcile logic for a Restore object.
func (r *RestoreManager) reconcileRestore(ctx context.Context, restore *backupapi.Restore) (ctrl.Result, error) {
	fleetName, destinationClusters, err := r.fetchRestoreDestinationClusters(ctx, restore)
	if err != nil {
		r.logger.Error(err, "failed to fetch destination clusters for restore", "restoreName", restore.Name)
		return ctrl.Result{}, err
	}

	// Apply restore resource in target clusters
	result, err := r.reconcileRestoreResources(ctx, restore, destinationClusters, fleetName)
	if err != nil || result.Requeue || result.RequeueAfter > 0 {
		return result, err
	}

	// Collect target clusters velero restore resource status to current restore
	if restore.Status.Details == nil {
		restore.Status.Details = []*backupapi.RestoreDetails{}
	}
	restore.Status.Details, err = syncVeleroRestoreStatus(ctx, destinationClusters, restore.Status.Details, RestoreKind, restore.Namespace, restore.Name)
	if err != nil {
		r.logger.Error(err, "failed to sync velero restore status for restore", "restoreName", restore.Name)
		return ctrl.Result{}, err
	}

	// Determine whether to requeue the reconciliation based on the completion status of all Velero restore resources.
	// If all restore are complete, exit directly without requeuing.
	// Otherwise, requeue the reconciliation after StatusSyncInterval.
	if allRestoreCompleted(restore.Status.Details) {
		return ctrl.Result{}, nil
	} else {
		return ctrl.Result{RequeueAfter: StatusSyncInterval}, nil
	}
}

// reconcileRestoreResources converts the restore resources into velero restore resources that can be used by Velero on the target clusters, and applies each of these resources to the respective target clusters.
func (r *RestoreManager) reconcileRestoreResources(ctx context.Context, restore *backupapi.Restore, destinationClusters map[ClusterKey]*fleetCluster, fleetName string) (ctrl.Result, error) {
	restoreLabels := generateVeleroInstanceLabel(RestoreNameLabel, restore.Name, fleetName)

	// Add tasks of syncVeleroObj func
	var tasks []func() error
	for clusterKey, clusterAccess := range destinationClusters {
		veleroBackupName := generateVeleroResourceName(clusterKey.Name, BackupKind, restore.Namespace, restore.Spec.BackupName)
		veleroRestoreName := generateVeleroResourceName(clusterKey.Name, RestoreKind, restore.Namespace, restore.Name)
		veleroRestore := buildVeleroRestoreInstance(&restore.Spec, restoreLabels, veleroBackupName, veleroRestoreName)

		task := newSyncVeleroTaskFunc(ctx, clusterAccess, veleroRestore)
		tasks = append(tasks, task)
	}

	// Parallel process syncVeleroObj func
	errs := parallelProcess(tasks)
	// Check for errors
	var errorList []string
	for _, err := range errs {
		if err != nil {
			r.logger.Error(err, "Error encountered during create velero restore instance for restore", "restoreName", restore.Name)
			errorList = append(errorList, err.Error())
		}
	}

	if len(errorList) > 0 {
		// Return all errs
		return ctrl.Result{}, fmt.Errorf("encountered %d errors during processing: %s", len(errorList), strings.Join(errorList, "; "))
	}

	return ctrl.Result{}, nil
}

// reconcileDeleteRestore handles the deletion process of a Restore object.
func (r *RestoreManager) reconcileDeleteRestore(ctx context.Context, restore *backupapi.Restore) (ctrl.Result, error) {
	// Fetch destination clusters
	_, destinationClusters, err := r.fetchRestoreDestinationClusters(ctx, restore)
	if err != nil {
		r.logger.Error(err, "failed to fetch destination clusters when delete restore", "restoreName", restore.Name)
		controllerutil.RemoveFinalizer(restore, RestoreFinalizer)
		r.logger.Info("Removed finalizer due to fetch destination clusters error", "restoreName", restore.Name)
		return ctrl.Result{}, err
	}

	restoreList := &velerov1.RestoreList{}
	// Delete all related velero restore instance
	if err := deleteResourcesInClusters(ctx, VeleroNamespace, RestoreNameLabel, restore.Name, destinationClusters, restoreList); err != nil {
		r.logger.Error(err, "failed to delete velero restore Instances when delete restore", "restoreName", restore.Name)
		return ctrl.Result{}, err
	}

	// Remove finalizer
	controllerutil.RemoveFinalizer(restore, RestoreFinalizer)

	return ctrl.Result{}, nil
}
