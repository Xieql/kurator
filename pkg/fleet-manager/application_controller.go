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
	kustomizev1 "github.com/fluxcd/kustomize-controller/api/v1"
	sourcev1 "github.com/fluxcd/source-controller/api/v1"
	sourceapi "github.com/fluxcd/source-controller/api/v1beta2"
	//helmv2beta1 "github.com/fluxcd/helm-controller/api/v2beta1"
	fluxmeta "github.com/fluxcd/pkg/apis/meta"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	clusterv1alpha1 "kurator.dev/kurator/pkg/apis/cluster/v1alpha1"
	fleetapi "kurator.dev/kurator/pkg/apis/fleet/v1alpha1"

	"github.com/pkg/errors"
	apiserrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/klog/v2"
	applicationapi "kurator.dev/kurator/pkg/apis/apps/v1alpha1"
	"sigs.k8s.io/cluster-api/util/patch"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
)

const (
	gitRepoKind  = "GitRepository"
	helmRepoKind = "HelmRepository"
	ociRepoKind  = "OCIRepository"

	ApplicationLabel = "apps.kurator.dev/app-name"
	ApplicationKind  = "Application"
)

// ApplicationManager reconciles an Application object
type ApplicationManager struct {
	client.Client
	Scheme *runtime.Scheme
}

// SetupWithManager sets up the controller with the Manager.
func (a *ApplicationManager) SetupWithManager(ctx context.Context, mgr ctrl.Manager, options controller.Options) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&applicationapi.Application{}).
		WithOptions(options).
		Complete(a)
}

func (a *ApplicationManager) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, reterr error) {
	app := &applicationapi.Application{}
	if err := a.Get(ctx, req.NamespacedName, app); err != nil {
		if apiserrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, errors.Wrapf(err, "failed to get application %s", req.NamespacedName)
	}

	log := ctrl.LoggerFrom(ctx)
	log = log.WithValues("attachedCluster", klog.KObj(app))
	log.Info("~~~~~~ app Reconcile", "the fleet is ", app.Spec.SyncPolicy[0].Destination.Fleet)

	patchHelper, err := patch.NewHelper(app, a.Client)
	if err != nil {
		return ctrl.Result{}, errors.Wrapf(err, "failed to init patch helper for application %s", req.NamespacedName)
	}

	defer func() {
		patchOpts := []patch.Option{}
		if err := patchHelper.Patch(ctx, app, patchOpts...); err != nil {
			reterr = utilerrors.NewAggregate([]error{reterr, errors.Wrapf(err, "failed to patch application %s", req.NamespacedName)})
		}
	}()

	//// Add finalizer if not exist to void the race condition.
	//if !controllerutil.ContainsFinalizer(fleet, FleetFinalizer) {
	//	app.Status.Phase = fleetapi.RunningPhase
	//	controllerutil.AddFinalizer(fleet, FleetFinalizer)
	//	return ctrl.Result{}, nil
	//} 自动化创建kind

	// Handle deletion reconciliation loop.
	if app.DeletionTimestamp != nil {
		//if fleet.Status.Phase != fleetapi.TerminatingPhase {
		//	fleet.Status.Phase = fleetapi.TerminatingPhase
		//}

		return a.reconcileDelete(ctx, app)
	}

	// todo :
	//type ApplicationSyncPolicy struct {
	//	// Name defines the name of the sync policy.
	//	// If unspecified, a name of format `<application name>-<index>` will be generated.
	//	// +optional
	//	name

	// Handle normal loop.
	return a.reconcile(ctx, app)
}

func (a *ApplicationManager) reconcile(ctx context.Context, app *applicationapi.Application) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)

	log.Info("~~~~~~ app reconcile")

	if err := a.createResourcesFromApplication(ctx, app); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (a *ApplicationManager) createResourcesFromApplication(ctx context.Context, app *applicationapi.Application) error {
	// find the only source kind from application
	kind, err := findSourceKindFromApplication(app)
	if err != nil {
		return err
	}
	// create a source resource for flux using the application's spec field.
	if err := a.createSourceResource(ctx, app, kind); err != nil {
		return err
	}
	// create all Kustomization or HelmRelease resources for all clusters recorded in the fleet field of the application's spec.SyncPolicy.
	for _, policy := range app.Spec.SyncPolicy {
		// a policy have a fleet, a fleet have many clusters, so a policy may need create many kustomization for each cluster
		if err := a.createSyncPolicyResource(ctx, app, policy, kind); err != nil {
			return err
		}
	}
	return nil
}

func (a *ApplicationManager) createSyncPolicyResource(ctx context.Context, app *applicationapi.Application, syncPolicy *applicationapi.ApplicationSyncPolicy, kind string) error {
	log := ctrl.LoggerFrom(ctx)

	fleetKey := client.ObjectKey{
		Namespace: app.Namespace,
		Name:      syncPolicy.Destination.Fleet,
	}
	// get the fleet obj
	fleet := &fleetapi.Fleet{}
	if err := a.Client.Get(ctx, fleetKey, fleet); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("fleet does not exist", "fleet", fleetKey)
			return nil
		}
		log.Error(err, "failed to find fleet", "fleet", fleetKey)
		// Error reading the object - requeue the request.
		return err
	}

	if kind == gitRepoKind {
		if err := a.createKustomizationsForFleet(ctx, app, syncPolicy, fleet); err != nil {
			log.Error(err, "failed to create gitRepo from application")
			return err
		}
	}

	//if kind == helmRepoKind {
	//	if err := a.createHelmRealeaseApplication(ctx, app,syncPolicy); err != nil {
	//		log.Error(err, "failed to create helmRepo from application")
	//	}
	//}
	return nil
}

func (a *ApplicationManager) createKustomizationsForFleet(ctx context.Context, app *applicationapi.Application, syncPolicy *applicationapi.ApplicationSyncPolicy, fleet *fleetapi.Fleet) error {
	log := ctrl.LoggerFrom(ctx)
	kustomization := syncPolicy.Kustomization

	// get clusters in fleet
	var clusterList clusterv1alpha1.ClusterList
	if err := a.Client.List(ctx, &clusterList,
		client.InNamespace(fleet.Namespace),
		client.MatchingLabels{FleetLabel: fleet.Name}); err != nil {
		log.Error(err, "failed to list cluster for fleet", "fleet", fleet.Name)
		return err
	}

	// create kustomization for each cluster in current fleet
	for _, cluster := range clusterList.Items {
		secretRef := fluxmeta.SecretKeyReference{
			Name: cluster.GetSecretName(),
			Key:  cluster.GetSecretKey(),
		}
		kubeConfig := &fluxmeta.KubeConfigReference{
			SecretRef: secretRef,
		}
		if err := a.createKustomizationForCluster(ctx, app, kustomization, kubeConfig, syncPolicy.Name+cluster.Name); err != nil {
			return err
		}
	}

	// get attachedClusters in fleet
	var attachedClusterList clusterv1alpha1.AttachedClusterList
	if err := a.Client.List(ctx, &attachedClusterList,
		client.InNamespace(fleet.Namespace),
		client.MatchingLabels{FleetLabel: fleet.Name}); err != nil {
		log.Error(err, "failed to list attachedClusterList for fleet", "fleet", fleet.Name)
		return err
	}

	// create kustomization for each attachedClusterList in current fleet
	for _, attachedCluster := range attachedClusterList.Items {
		secretRef := fluxmeta.SecretKeyReference{
			Name: attachedCluster.GetSecretName(),
			Key:  attachedCluster.GetSecretKey(),
		}
		kubeConfig := &fluxmeta.KubeConfigReference{
			SecretRef: secretRef,
		}
		kustomizationName := generateKustomizationName(app, syncPolicy, attachedCluster.Kind, attachedCluster.Name)

		if err := a.createKustomizationForCluster(ctx, app, kustomization, kubeConfig, kustomizationName); err != nil {
			return err
		}
	}
	return nil
}

func (a *ApplicationManager) createKustomizationForCluster(ctx context.Context, app *applicationapi.Application, kustomization *applicationapi.Kustomization, kubeConfig *fluxmeta.KubeConfigReference, kustomizationName string) error {
	fluxKustomization := &kustomizev1.Kustomization{
		ObjectMeta: metav1.ObjectMeta{
			Name:      kustomizationName,
			Namespace: app.Namespace,
			Labels: map[string]string{
				ApplicationLabel: app.Name,
			},
			OwnerReferences: []metav1.OwnerReference{generateApplicationOwnerRef(app)},
		},
		Spec: kustomizev1.KustomizationSpec{
			DependsOn:     kustomization.DependsOn,
			Interval:      kustomization.Interval,
			RetryInterval: kustomization.RetryInterval,
			KubeConfig:    kubeConfig,
			Path:          kustomization.Path,
			Prune:         kustomization.Prune,
			Patches:       kustomization.Patches,
			Images:        kustomization.Images,
			SourceRef: kustomizev1.CrossNamespaceSourceReference{
				Kind: gitRepoKind,
				Name: app.Name,
			},
			Suspend:         kustomization.Suspend,
			TargetNamespace: kustomization.TargetNamespace,
			Timeout:         kustomization.Timeout,
			Force:           kustomization.Force,
			Components:      kustomization.Components,
		},
	}

	// the CommonMetadata is not directly refer to kustomizev1, so specific handle in case CommonMetadata is nil
	if kustomization.CommonMetadata != nil {
		fluxKustomization.Spec.CommonMetadata = &kustomizev1.CommonMetadata{
			Annotations: kustomization.CommonMetadata.Annotations,
			Labels:      kustomization.CommonMetadata.Labels,
		}
	}
	if err := a.Client.Create(ctx, fluxKustomization); err != nil {
		return err
	}
	return nil
}

func (a *ApplicationManager) createSourceResource(ctx context.Context, app *applicationapi.Application, kind string) error {
	log := ctrl.LoggerFrom(ctx)

	if kind == gitRepoKind {
		if err := a.createGitRepoFromApplication(ctx, app); err != nil {
			log.Error(err, "failed to create gitRepo from application")
		}
	}
	if kind == helmRepoKind {
		if err := a.createHelmRepoFromApplication(ctx, app); err != nil {
			log.Error(err, "failed to create helmRepo from application")
		}
	}
	if kind == ociRepoKind {
		if err := a.createOCIRepoFromApplication(ctx, app); err != nil {
			log.Error(err, "failed to create ociRepo from application")
		}
	}
	return nil
}

func (a *ApplicationManager) createGitRepoFromApplication(ctx context.Context, app *applicationapi.Application) error {
	source := &sourcev1.GitRepository{
		ObjectMeta: metav1.ObjectMeta{
			Name:      app.Name,
			Namespace: app.Namespace,
			Labels: map[string]string{
				ApplicationLabel: app.Name,
			},
			OwnerReferences: []metav1.OwnerReference{generateApplicationOwnerRef(app)},
		},
		Spec: *app.Spec.Source.GitRepo,
	}
	if err := a.Client.Create(ctx, source); err != nil {
		return err
	}
	return nil
}
func (a *ApplicationManager) createHelmRepoFromApplication(ctx context.Context, app *applicationapi.Application) error {
	source := &sourceapi.HelmRepository{
		ObjectMeta: metav1.ObjectMeta{
			Name:      app.Name,
			Namespace: app.Namespace,
			Labels: map[string]string{
				ApplicationLabel: app.Name,
			},
			OwnerReferences: []metav1.OwnerReference{generateApplicationOwnerRef(app)},
		},
		Spec: *app.Spec.Source.HelmRepo,
	}
	if err := a.Client.Create(ctx, source); err != nil {
		return err
	}
	return nil
}

func (a *ApplicationManager) createOCIRepoFromApplication(ctx context.Context, app *applicationapi.Application) error {
	source := &sourceapi.OCIRepository{
		ObjectMeta: metav1.ObjectMeta{
			Name:      app.Name,
			Namespace: app.Namespace,
			Labels: map[string]string{
				ApplicationLabel: app.Name,
			},
			OwnerReferences: []metav1.OwnerReference{generateApplicationOwnerRef(app)},
		},
		Spec: *app.Spec.Source.OCIRepo,
	}
	if err := a.Client.Create(ctx, source); err != nil {
		return err
	}
	return nil
}

// findSourceKindFromApplication finds the source kind from the given application.
// Only one kind of source can be specified. If no source or more than one source is specified, an error is returned.
func findSourceKindFromApplication(app *applicationapi.Application) (string, error) {
	validCount := 0
	validKind := ""
	gitRepo := app.Spec.Source.GitRepo
	if gitRepo != nil {
		validCount++
		validKind = gitRepoKind
	}
	helmRepo := app.Spec.Source.HelmRepo
	if helmRepo != nil {
		validCount++
		validKind = helmRepoKind
	}
	ociRepo := app.Spec.Source.OCIRepo
	if ociRepo != nil {
		validCount++
		validKind = ociRepoKind
	}
	if validCount != 1 {
		return "", fmt.Errorf("only one kind of source can be specified. The current specified count is %d", validCount)
	}
	return validKind, nil
}

func (a *ApplicationManager) reconcileDelete(ctx context.Context, app *applicationapi.Application) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)

	log.Info("~~~~~~ app reconcileDelete")

	return ctrl.Result{}, nil
}

// appName + policyName + clusterName
func generateKustomizationName(app *applicationapi.Application, syncPolicy *applicationapi.ApplicationSyncPolicy, kind, clusterName string) string {
	name := app.Name + "-" + syncPolicy.Name + "-" + kind + "-" + clusterName

	return name
}

func generateApplicationOwnerRef(app *applicationapi.Application) metav1.OwnerReference {
	ownerRef := metav1.OwnerReference{
		APIVersion: applicationapi.GroupVersion.String(),
		Kind:       ApplicationKind,
		Name:       app.Name,
		UID:        app.UID,
	}
	return ownerRef
}
