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

package controllers

import (
	"context"
	"fmt"
	"kurator.dev/kurator/pkg/apis/cluster/v1alpha1"

	"k8s.io/klog/v2"
	"sigs.k8s.io/cluster-api/util"

	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/predicates"
	"sigs.k8s.io/controller-runtime/pkg/source"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/cluster-api/controllers/external"
	controlplanev1 "sigs.k8s.io/cluster-api/controlplane/kubeadm/api/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
)

// CustomClusterReconciler reconciles a CustomCluster object
type CustomClusterController struct {
	client.Client
	APIReader client.Reader
	Scheme    *runtime.Scheme

	externalTracker external.ObjectTracker
}

//+kubebuilder:rbac:groups=kurator.dev.kurator.dev,resources=customclusters,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=kurator.dev.kurator.dev,resources=customclusters/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=kurator.dev.kurator.dev,resources=customclusters/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the CustomCluster object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.13.0/pkg/reconcile
func (r *CustomClusterController) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {

	log := ctrl.LoggerFrom(ctx)
	log.Info("***********~~~~~~~~~~reconcile begin~~~~~~~~~")

	// Fetch the KubeadmControlPlane instance.
	kcp := &controlplanev1.KubeadmControlPlane{}
	if err := r.Client.Get(ctx, req.NamespacedName, kcp); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// Fetch the Cluster.
	cluster, err := util.GetOwnerCluster(ctx, r.Client, kcp.ObjectMeta)
	if err != nil {
		log.Error(err, "Failed to retrieve owner Cluster from the API Server")
		return ctrl.Result{}, err
	}
	if cluster == nil {
		log.Info("Cluster Controller has not yet set OwnerRef")
		return ctrl.Result{}, nil
	}

	// TODO: check cluster status

	log = log.WithValues("Cluster", klog.KObj(cluster))
	ctx = ctrl.LoggerInto(ctx, log)

	return r.reconcile(ctx, kcp, cluster)
}

func (r *CustomClusterController) reconcile(ctx context.Context, kcp *controlplanev1.KubeadmControlPlane, cluster *clusterv1.Cluster) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)
	log.Info("~~~~~~~~~~reconcile begin~~~~~~~~~")

	// TODO: 根据 Cluster KCP CustomMachine 及 SSH key secret生成kubespray参数 Configmap
	log.Info("~~~~~~~~~~create configmap from ssh key secret / kcp /customMachine~~~~~~~~~~~~~")

	// 创建Pod （挂载SSH证书以及配置参数） 执行ansible-playbook创建集群
	log.Info("~~~~~~~~~~create job/pod to exec ansible-playbook~~~~~~~~")

	log.Info("~~~~~~~~~~exec the job~~~~~~~~")

	log.Info("~~~~~~~~~~reconcile end~~~~~~~~~~")
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *CustomClusterController) SetupWithManager(ctx context.Context, mgr ctrl.Manager, options controller.Options) error {
	c, err := ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.CustomMachine{}).
		WithOptions(options).
		Build(r)
	if err != nil {
		return fmt.Errorf("failed setting up with a controller manager: %v", err)
	}

	err = c.Watch(
		&source.Kind{Type: &clusterv1.Cluster{}},
		handler.EnqueueRequestsFromMapFunc(r.ClusterToKubeadmControlPlane),
		predicates.All(ctrl.LoggerFrom(ctx),
			predicates.ResourceHasFilterLabel(ctrl.LoggerFrom(ctx), ""), // TODO: add filter to distinguish from Cluster on AWS
			predicates.ClusterUnpausedAndInfrastructureReady(ctrl.LoggerFrom(ctx)),
		),
	)
	if err != nil {
		return fmt.Errorf("failed adding Watch for Clusters to controller manager: %v", err)
	}

	return nil
}

func (r *CustomClusterController) ClusterToKubeadmControlPlane(o client.Object) []ctrl.Request {
	c, ok := o.(*clusterv1.Cluster)
	if !ok {
		panic(fmt.Sprintf("Expected a Cluster but got a %T", o))
	}

	controlPlaneRef := c.Spec.ControlPlaneRef
	if controlPlaneRef != nil && controlPlaneRef.Kind == "KubeadmControlPlane" {
		return []ctrl.Request{{NamespacedName: client.ObjectKey{Namespace: controlPlaneRef.Namespace, Name: controlPlaneRef.Name}}}
	}

	return nil
}

func (r *CustomClusterController) ClusterToCustomCluster(o client.Object) []ctrl.Request {
	c, ok := o.(*clusterv1.Cluster)
	if !ok {
		panic(fmt.Sprintf("Expected a Cluster but got a %T", o))
	}

	controlPlaneRef := c.Spec.ControlPlaneRef
	if controlPlaneRef != nil && controlPlaneRef.Kind == "KubeadmControlPlane" {
		return []ctrl.Request{{NamespacedName: client.ObjectKey{Namespace: controlPlaneRef.Namespace, Name: controlPlaneRef.Name}}}
	}

	return nil
}

func (r *CustomClusterController) CustomMachineToCustomCluster(o client.Object) []ctrl.Request {
	c, ok := o.(*v1alpha1.CustomMachine)
	if !ok {
		panic(fmt.Sprintf("Expected a Cluster but got a %T", o))
	}

	machineOwner := c.Spec.MachineOwner
	if machineOwner != nil && machineOwner.Kind == "KubeadmControlPlane" {
		return []ctrl.Request{{NamespacedName: client.ObjectKey{Namespace: machineOwner.Namespace, Name: machineOwner.Name}}}
	}

	return nil
}

func (r *CustomClusterController) KubeadmControlPlaneToCustomCluster(o client.Object) []ctrl.Request {
	c, ok := o.(*clusterv1.Cluster)
	if !ok {
		panic(fmt.Sprintf("Expected a Cluster but got a %T", o))
	}

	controlPlaneRef := c.Spec.ControlPlaneRef
	if controlPlaneRef != nil && controlPlaneRef.Kind == "KubeadmControlPlane" {
		return []ctrl.Request{{NamespacedName: client.ObjectKey{Namespace: controlPlaneRef.Namespace, Name: controlPlaneRef.Name}}}
	}

	return nil
}
