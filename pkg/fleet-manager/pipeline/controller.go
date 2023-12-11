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

package pipeline

import (
	"context"
	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"sigs.k8s.io/cluster-api/util/patch"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	pipelineapi "kurator.dev/kurator/pkg/apis/pipeline/v1alpha1"
)

const (
	PipelineFinalizer = "pipeline.kurator.dev"
)

// PipelineManager reconciles a Pipeline object
type PipelineManager struct {
	client.Client
	Scheme *runtime.Scheme
}

// SetupWithManager sets up the controller with the Manager.
func (p *PipelineManager) SetupWithManager(ctx context.Context, mgr ctrl.Manager, options controller.Options) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&pipelineapi.Pipeline{}).
		WithOptions(options).
		Complete(p)
}

func (p *PipelineManager) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, reterr error) {
	log := ctrl.LoggerFrom(ctx)
	pipeline := &pipelineapi.Pipeline{}
	log.Info("~~~~~~~~~~~~~~~~~~~Reconcile ", "pipeline", ctx)

	if err := p.Client.Get(ctx, req.NamespacedName, pipeline); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("pipeline object not found", "pipeline", req)
			return ctrl.Result{}, nil
		}

		// Error reading the object - requeue the request.
		return ctrl.Result{}, err
	}

	// Initialize patch helper
	patchHelper, err := patch.NewHelper(pipeline, p.Client)
	if err != nil {
		return ctrl.Result{}, errors.Wrapf(err, "failed to init patch helper for pipeline %s", req.NamespacedName)
	}
	// Setup deferred function to handle patching the object at the end of the reconciler
	defer func() {
		patchOpts := []patch.Option{}
		if err := patchHelper.Patch(ctx, pipeline, patchOpts...); err != nil {
			reterr = utilerrors.NewAggregate([]error{reterr, errors.Wrapf(err, "failed to patch %s  %s", pipeline.Name, req.NamespacedName)})
		}
	}()

	// Check and add finalizer if not present
	if !controllerutil.ContainsFinalizer(pipeline, PipelineFinalizer) {
		controllerutil.AddFinalizer(pipeline, PipelineFinalizer)
		return ctrl.Result{}, nil
	}

	// Handle deletion
	if pipeline.GetDeletionTimestamp() != nil {
		return p.reconcileDeletePipeline(ctx, pipeline)
	}

	// Handle the main reconcile logic
	return p.reconcilePipeline(ctx, pipeline)
}

// reconcilePipeline handles the main reconcile logic for a Pipeline object.
func (p *PipelineManager) reconcilePipeline(ctx context.Context, pipeline *pipelineapi.Pipeline) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)
	log.Info("~~~~~~~~~~~~~~~~~~~reconcilePipeline ", "pipeline", ctx)

	// Apply Tekton rbac, tasks, pipeline and trigger resource from Kurator pipeline
	result, err := p.reconcileTektonResources(ctx, pipeline)
	if err != nil || result.Requeue || result.RequeueAfter > 0 {
		return result, err
	}
	// update status
	return p.reconcilePipelineStatus(ctx, pipeline)
}

// reconcilePipelineResources converts the pipeline resources into Tekton resource and apply them.
func (p *PipelineManager) reconcileTektonResources(ctx context.Context, pipeline *pipelineapi.Pipeline) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)
	log.Info("~~~~~~~~~~~~~~~~~~~reconcileTektonResources ", "pipeline", ctx)

	return ctrl.Result{}, nil
}

// reconcilePipelineStatus updates status of each pipeline resource.
func (p *PipelineManager) reconcilePipelineStatus(ctx context.Context, pipeline *pipelineapi.Pipeline) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)
	log.Info("~~~~~~~~~~~~~~~~~~~reconcilePipelineStatus ", "pipeline", ctx)

	// Remove finalizer
	pipeline.Status.Phase = pipelineapi.ReadyPhase

	return ctrl.Result{}, nil
}

// reconcileDeletePipeline handles the deletion process of a Pipeline object.
func (p *PipelineManager) reconcileDeletePipeline(ctx context.Context, pipeline *pipelineapi.Pipeline) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)
	log.Info("~~~~~~~~~~~~~~~~~~~reconcileDeletePipeline ", "pipeline", ctx)

	// Remove finalizer
	controllerutil.RemoveFinalizer(pipeline, PipelineFinalizer)

	return ctrl.Result{}, nil
}
