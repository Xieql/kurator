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
	apiserrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/klog/v2"
	applicationapi "kurator.dev/kurator/pkg/apis/apps/v1alpha1"
	"sigs.k8s.io/cluster-api/util/patch"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
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
	log.Info("~~~~~~ app Reconcile")

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
	//}

	// Handle deletion reconciliation loop.
	if app.DeletionTimestamp != nil {
		//if fleet.Status.Phase != fleetapi.TerminatingPhase {
		//	fleet.Status.Phase = fleetapi.TerminatingPhase
		//}

		return a.reconcileDelete(ctx, app)
	}

	// Handle normal loop.
	return a.reconcile(ctx, app)
}

func (a *ApplicationManager) reconcile(ctx context.Context, app *applicationapi.Application) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)

	log.Info("~~~~~~ app reconcile")

	return ctrl.Result{}, nil
}

func (a *ApplicationManager) reconcileDelete(ctx context.Context, app *applicationapi.Application) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)

	log.Info("~~~~~~ app reconcileDelete")

	return ctrl.Result{}, nil
}
