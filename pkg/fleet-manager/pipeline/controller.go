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
	"fmt"
	"github.com/pkg/errors"
	"io/fs"
	v1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	pipelineapi "kurator.dev/kurator/pkg/apis/pipeline/v1alpha1"
	"kurator.dev/kurator/pkg/fleet-manager/pipeline/render"
	"kurator.dev/kurator/pkg/fleet-manager/pipeline/render/manifests"
	"kurator.dev/kurator/pkg/infra/util"
	"sigs.k8s.io/cluster-api/util/patch"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
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
	log.Info("~~~~~~~~~~~~~~~~~~~!!!!! ", "pipeline", ctx)

	rbacConfig := render.RBACConfig{
		PipelineName:      pipeline.Name,
		PipelineNamespace: pipeline.Name,
	}

	// rbac 必须先于其他资源创建。之后，pipeline、task、triggers 等资源在创建阶段，不严格要求创建顺序。在使用阶段，需要确保所有资源创建完成
	if !p.isRBACResourceReady(ctx, rbacConfig) {
		res, err := p.reconcileCreateRBAC(ctx, rbacConfig)
		if err != nil || res.Requeue || res.RequeueAfter > 0 {
			return res, err
		}
	}

	// Apply Tekton tasks,
	res, err := p.reconcileCreateTasks(ctx, pipeline)
	if err != nil || res.Requeue || res.RequeueAfter > 0 {
		return res, err
	}

	// Apply Tekton pipeline
	res, err = p.reconcileCreatePipeline(ctx, pipeline)
	if err != nil || res.Requeue || res.RequeueAfter > 0 {
		return res, err
	}

	// Apply Tekton trigger
	res, err = p.reconcileCreateTrigger(ctx, pipeline)
	if err != nil || res.Requeue || res.RequeueAfter > 0 {
		return res, err
	}

	// update status
	return p.reconcilePipelineStatus(ctx, pipeline)
}

// reconcileCreateRBAC converts the pipeline resources into Tekton resource and apply them.
func (p *PipelineManager) reconcileCreateRBAC(ctx context.Context, rbacConfig render.RBACConfig) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)
	log.Info("~~~~~~~~~~~~~~~~~~~reconcileCreateRBAC ", "pipeline", ctx)
	log.Info("~~~~~~~~~~~~~~~~~~~WalkDir ", "pipeline", ctx)

	manifestFileSystem := manifests.BuiltinOrDir("rbac/")

	err := fs.WalkDir(manifestFileSystem, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		fmt.Println(path)
		return nil
	})
	if err != nil {
		return ctrl.Result{}, err
	}

	rbac, err := render.RenderRBAC(manifestFileSystem, rbacConfig)
	if err != nil {
		return ctrl.Result{}, err
	}

	// apply rbac resources
	if _, err := util.PatchResources(rbac); err != nil {
		return ctrl.Result{}, errors.Wrapf(err, "failed to apply rbac resources")
	}

	return ctrl.Result{}, nil
}

// reconcileCreateTasks converts the pipeline resources into Tekton resource and apply them.
func (p *PipelineManager) reconcileCreateTasks(ctx context.Context, pipeline *pipelineapi.Pipeline) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)
	log.Info("~~~~~~~~~~~~~~~~~~~reconcileCreateRBAC ", "pipeline", ctx)

	for _, task := range pipeline.Spec.Tasks {
		if task.PredefinedTask != nil {
			err := p.createPredefinedTask(ctx, task, pipeline)
			return ctrl.Result{}, err
		} else {
			err := p.createCustomTask(ctx, task, pipeline)
			if err != nil {
				return ctrl.Result{}, err
			}
		}
	}

	return ctrl.Result{}, nil
}

// createPredefinedTask converts the pipeline resources into Tekton resource and apply them.
func (p *PipelineManager) createPredefinedTask(ctx context.Context, task pipelineapi.PipelineTask, pipeline *pipelineapi.Pipeline) error {
	log := ctrl.LoggerFrom(ctx)
	log.Info("~~~~~~~~~~~~~~~~~~~createPredefinedTask ", "pipeline", ctx)

	cfg := render.PredefinedTaskConfig{
		PipelineName:      pipeline.Name,
		PipelineNamespace: pipeline.Namespace,
		TaskName:          task.Name,
		TemplateName:      string(task.PredefinedTask.Name),
		Params:            nil,
	}

	manifestFileSystem := manifests.BuiltinOrDir("PredefinedTask/")
	taskResource, err := render.RenderPredefinedTask(manifestFileSystem, cfg)
	if err != nil {
		return err
	}
	// apply rbac resources
	if _, err := util.PatchResources(taskResource); err != nil {
		return errors.Wrapf(err, "failed to apply rbac resources")
	}

	return nil
}

// createCustomTask converts the pipeline resources into Tekton resource and apply them.
func (p *PipelineManager) createCustomTask(ctx context.Context, task pipelineapi.PipelineTask, pipeline *pipelineapi.Pipeline) error {
	log := ctrl.LoggerFrom(ctx)
	log.Info("~~~~~~~~~~~~~~~~~~~createCustomTask ", "pipeline", ctx)

	manifestFileSystem := manifests.BuiltinOrDir("custom-task/")
	taskResource, err := render.RenderCustomTaskWithPipeline(manifestFileSystem, task.Name, pipeline.Name, pipeline.Namespace, *task.CustomTask)
	if err != nil {
		return err
	}
	// apply custom task resources
	if _, err := util.PatchResources(taskResource); err != nil {
		return errors.Wrapf(err, "failed to apply rbac resources")
	}

	return nil
}

// reconcileCreatePipeline converts the pipeline resources into Tekton resource and apply them.
func (p *PipelineManager) reconcileCreatePipeline(ctx context.Context, pipeline *pipelineapi.Pipeline) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)
	log.Info("~~~~~~~~~~~~~~~~~~~reconcileCreatePipeline ", "pipeline", ctx)
	manifestFileSystem := manifests.BuiltinOrDir("pipeline/")
	pipelineResource, err := render.RenderPipelineWithTasks(manifestFileSystem, pipeline.Name, pipeline.Namespace, pipeline.Spec.Tasks)
	if err != nil {
		return ctrl.Result{}, err
	}

	// apply pipeline resources
	if _, err := util.PatchResources(pipelineResource); err != nil {
		return ctrl.Result{}, errors.Wrapf(err, "failed to apply rbac resources")
	}

	return ctrl.Result{}, nil
}

// reconcileCreateTrigger converts the pipeline resources into Tekton resource and apply them.
func (p *PipelineManager) reconcileCreateTrigger(ctx context.Context, pipeline *pipelineapi.Pipeline) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)
	log.Info("~~~~~~~~~~~~~~~~~~~reconcileCreateTrigger ", "pipeline", ctx)
	manifestFileSystem := manifests.BuiltinOrDir("trigger/")

	cfg := render.TriggerConfig{
		PipelineName:      pipeline.Name,
		PipelineNamespace: pipeline.Namespace,
	}
	triggerResource, err := render.RenderTrigger(manifestFileSystem, cfg)
	if err != nil {
		return ctrl.Result{}, err
	}

	// apply pipeline resources
	if _, err := util.PatchResources(triggerResource); err != nil {
		return ctrl.Result{}, errors.Wrapf(err, "failed to apply rbac resources")
	}

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

// isRBACResourceReady checks if the necessary RBAC resources are ready in the specified namespace.
func (p *PipelineManager) isRBACResourceReady(ctx context.Context, rbacConfig render.RBACConfig) bool {
	// Check for the existence of the ServiceAccount
	sa := &v1.ServiceAccount{}
	err := p.Client.Get(ctx, types.NamespacedName{Name: rbacConfig.PipelineName, Namespace: rbacConfig.PipelineNamespace}, sa)
	if err != nil {
		return false
	}

	// Check for the existence of the RoleBinding for broad resources
	broadResourceRoleBinding := &rbacv1.RoleBinding{}
	err = p.Client.Get(ctx, types.NamespacedName{Name: rbacConfig.BroadResourceRoleBindingName(), Namespace: rbacConfig.PipelineNamespace}, broadResourceRoleBinding)
	if err != nil {
		return false
	}

	// Check for the existence of the RoleBinding for secret resources
	secretResourceRoleBinding := &rbacv1.RoleBinding{}
	err = p.Client.Get(ctx, types.NamespacedName{Name: rbacConfig.SecretRoleBindingName(), Namespace: rbacConfig.PipelineNamespace}, secretResourceRoleBinding)
	if err != nil {
		return false
	}

	// If all resources are found, return true
	return true
}
