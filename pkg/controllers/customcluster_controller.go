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
	"regexp"
	"strings"
	"text/template"
	"time"

	"github.com/coreos/go-semver/semver"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	controlplanev1 "sigs.k8s.io/cluster-api/controlplane/kubeadm/api/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"kurator.dev/kurator/pkg/apis/infra/v1alpha1"
)

// CustomClusterController reconciles a CustomCluster object.
type CustomClusterController struct {
	client.Client
	APIReader client.Reader
	Scheme    *runtime.Scheme
}

type customClusterManageCMD string
type customClusterManageAction string

// NodeInfo represents the information of the node on VMs
type NodeInfo struct {
	// NodeName, also called as HostName, is the unique identifier that distinguishes it from other nodes under the same cluster. kubespray uses the Hostname as the parameter to delete the node
	NodeName  string
	PublicIP  string
	PrivateIP string
}

// ClusterInfo represents the information of the cluster on VMs
type ClusterInfo struct {
	WorkerNodes []NodeInfo
	KubeVersion string
}

// KubesprayImageSupport specifies the configuration that is currently supported in the KubesprayVersion.
type KubesprayImageSupport struct {
	KubesprayVersion string
	MinK8sVersion    string
	MaxK8sVersion    string
	// Kubespray does not mention the concept of midVersion. It is set for upgrade by Kurator.
	// As Kubespray always supports a difference of exactly two minor versions between max and min, midVersion is set to the version in the mid between the two.
	MidK8sVersion string
}

// kubesprayImageSupportMap records the mapping between kubespray image versions and their corresponding support information.
var kubesprayImageSupportMap = map[string]KubesprayImageSupport{
	"v2.19.0": {
		KubesprayVersion: "v2.19.0",
		MinK8sVersion:    "v1.21.0",
		MidK8sVersion:    "v1.22.0",
		MaxK8sVersion:    "v1.23.7",
	},
	"v2.20.0": {
		KubesprayVersion: "v2.20.0",
		MinK8sVersion:    "v1.22.0",
		MidK8sVersion:    "v1.23.0",
		MaxK8sVersion:    "v1.24.6",
	},
	"v2.21.0": {
		KubesprayVersion: "v2.21.0",
		MinK8sVersion:    "v1.23.0",
		MidK8sVersion:    "v1.24.0",
		MaxK8sVersion:    "v1.25.6",
	},
}

const (
	RequeueAfter = time.Second * 5

	ClusterHostsName  = "cluster-hosts"
	ClusterConfigName = "cluster-config"
	SecreteName       = "cluster-secret"

	ClusterKind       = "Cluster"
	CustomClusterKind = "CustomCluster"

	KubesprayCMDPrefix                                     = "ansible-playbook -i inventory/" + ClusterHostsName + " --private-key /root/.ssh/ssh-privatekey "
	CustomClusterInitAction      customClusterManageAction = "init"
	KubesprayInitCMD             customClusterManageCMD    = KubesprayCMDPrefix + "cluster.yml -vvv "
	CustomClusterTerminateAction customClusterManageAction = "terminate"
	KubesprayTerminateCMD        customClusterManageCMD    = KubesprayCMDPrefix + "reset.yml -vvv -e reset_confirmation=yes"

	CustomClusterScaleUpAction customClusterManageAction = "scale-up"
	KubesprayScaleUpCMD        customClusterManageCMD    = KubesprayCMDPrefix + "scale.yml -vvv "

	CustomClusterScaleDownAction customClusterManageAction = "scale-down"
	KubesprayScaleDownCMDPrefix  customClusterManageCMD    = KubesprayCMDPrefix + "remove-node.yml -vvv -e skip_confirmation=yes"

	CustomClusterUpgradeAction customClusterManageAction = "upgrade"
	KubesprayUpgradeCMDPrefix  customClusterManageCMD    = KubesprayCMDPrefix + "upgrade-cluster.yml -vvv "

	// CustomClusterFinalizer is the finalizer applied to crd.
	CustomClusterFinalizer = "customcluster.cluster.kurator.dev"
	// custom configmap finalizer requires at least one slash.
	CustomClusterConfigMapFinalizer = CustomClusterFinalizer + "/configmap"

	// KubeVersionPrefix is the prefix string of version of kubernetes
	KubeVersionPrefix = "kube_version: "

	// DefaultKubesprayVersion is the version of Kubespray used by the customCluster manager worker pod.
	DefaultKubesprayVersion = "v2.20.0"
)

// SetupWithManager sets up the controller with the Manager.
func (r *CustomClusterController) SetupWithManager(ctx context.Context, mgr ctrl.Manager, options controller.Options) error {
	c, err1 := ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.CustomCluster{}).
		WithOptions(options).
		Build(r)
	if err1 != nil {
		return fmt.Errorf("failed setting up with a controller manager: %v", err1)
	}

	if err := c.Watch(
		&source.Kind{Type: &corev1.Pod{}},
		handler.EnqueueRequestsFromMapFunc(r.WorkerToCustomClusterMapFunc),
	); err != nil {
		return fmt.Errorf("failed adding Watch for worker to controller manager: %v", err)
	}

	if err := c.Watch(
		&source.Kind{Type: &v1alpha1.CustomMachine{}},
		handler.EnqueueRequestsFromMapFunc(r.CustomMachineToCustomClusterMapFunc),
	); err != nil {
		return fmt.Errorf("failed adding Watch for CustomMachine to controller manager: %v", err)
	}

	if err := c.Watch(
		&source.Kind{Type: &clusterv1.Cluster{}},
		handler.EnqueueRequestsFromMapFunc(r.ClusterToCustomClusterMapFunc),
	); err != nil {
		return fmt.Errorf("failed adding Watch for Clusters to controller manager: %v", err)
	}

	if err := c.Watch(
		&source.Kind{Type: &controlplanev1.KubeadmControlPlane{}},
		handler.EnqueueRequestsFromMapFunc(r.KcpToCustomClusterMapFunc),
	); err != nil {
		return fmt.Errorf("failed adding Watch for KubeadmControlPlan to controller manager: %v", err)
	}

	return nil
}

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *CustomClusterController) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)

	// Fetch the customCluster instance.
	customCluster := &v1alpha1.CustomCluster{}
	if err := r.Client.Get(ctx, req.NamespacedName, customCluster); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("customCluster does not exist", "customCluster", req)
			return ctrl.Result{}, nil
		}
		log.Error(err, "failed to find customCluster", "customCluster", req)
		// Error reading the object - requeue the request.
		return ctrl.Result{RequeueAfter: RequeueAfter}, err
	}
	log = log.WithValues("customCluster", klog.KObj(customCluster))
	ctx = ctrl.LoggerInto(ctx, log)
	// ensure customCluster status no nil
	if len(customCluster.Status.Phase) == 0 {
		customCluster.Status.Phase = v1alpha1.PendingPhase
		if err := r.Status().Update(ctx, customCluster); err != nil {
			log.Error(err, "failed to update customCluster status", "customCluster", req)
			return ctrl.Result{RequeueAfter: RequeueAfter}, err
		}
	}

	// Fetch the Cluster instance.
	var clusterName string
	for _, owner := range customCluster.GetOwnerReferences() {
		if owner.Kind == ClusterKind {
			clusterName = owner.Name
			break
		}
	}
	if len(clusterName) == 0 {
		log.Info("failed to get cluster from customCluster.GetOwnerReferences", "customCluster", req)
		return ctrl.Result{RequeueAfter: RequeueAfter}, nil
	}
	clusterKey := client.ObjectKey{
		Namespace: customCluster.GetNamespace(),
		Name:      clusterName,
	}
	cluster := &clusterv1.Cluster{}
	if err := r.Client.Get(ctx, clusterKey, cluster); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("cluster does not exist", "cluster", clusterKey)
			return ctrl.Result{}, nil
		}
		log.Error(err, "failed to get cluster", "cluster", clusterKey)
		return ctrl.Result{RequeueAfter: RequeueAfter}, err
	}

	// Fetch the CustomMachine instance.
	customMachinekey := client.ObjectKey{
		Namespace: customCluster.Spec.MachineRef.Namespace,
		Name:      customCluster.Spec.MachineRef.Name,
	}
	customMachine := &v1alpha1.CustomMachine{}
	if err := r.Client.Get(ctx, customMachinekey, customMachine); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("customMachine does not exist", "customMachine", customMachinekey)
			return ctrl.Result{}, nil
		}
		log.Error(err, "failed to find customMachine", "customMachine", customMachinekey)
		return ctrl.Result{RequeueAfter: RequeueAfter}, err
	}

	// Fetch the KubeadmControlPlane instance.
	kcpKey := client.ObjectKey{
		Namespace: cluster.Spec.ControlPlaneRef.Namespace,
		Name:      cluster.Spec.ControlPlaneRef.Name,
	}
	kcp := &controlplanev1.KubeadmControlPlane{}
	if err := r.Client.Get(ctx, kcpKey, kcp); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("kcp does not exist", "kcp", kcpKey)
			return ctrl.Result{}, nil
		}
		log.Error(err, "failed to get kcp", "kcp", kcpKey)
		return ctrl.Result{RequeueAfter: RequeueAfter}, err
	}

	return r.reconcile(ctx, customCluster, customMachine, cluster, kcp)
}

// reconcile handles CustomCluster reconciliation.
func (r *CustomClusterController) reconcile(ctx context.Context, customCluster *v1alpha1.CustomCluster, customMachine *v1alpha1.CustomMachine, cluster *clusterv1.Cluster, kcp *controlplanev1.KubeadmControlPlane) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)

	phase := customCluster.Status.Phase

	// If upstream cluster at pre-delete, the customCluster need to be deleted.
	if !cluster.DeletionTimestamp.IsZero() {
		// If customCluster is already in phase Deleting, the controller will check the terminating worker to handle Deleting process.
		if phase == v1alpha1.DeletingPhase {
			return r.reconcileHandleDeleting(ctx, customCluster, customMachine, kcp)
		}
		// If customCluster is not in Deleting, the controller should terminate the Vms cluster by create a terminating worker.
		return r.reconcileVMsTerminate(ctx, customCluster)
	}

	// CustomCluster in phase nil or ProvisionFailed will try to enter Provisioning phase by creating an init worker successfully.
	if phase == v1alpha1.PendingPhase || phase == v1alpha1.ProvisionFailedPhase {
		return r.reconcileCustomClusterInit(ctx, customCluster, customMachine, cluster, kcp)
	}

	// If customCluster is in phase Provisioning, the controller will handle Provisioning process by checking the init worker status.
	if phase == v1alpha1.ProvisioningPhase {
		return r.reconcileHandleProvisioning(ctx, customCluster, customMachine, cluster, kcp)
	}

	// Obtain desiredClusterInfo and provisionedClusterInfo to determine whether to proceed with subsequent scaling or upgrading.
	desiredClusterInfo := getDesiredClusterInfo(customMachine, customCluster)
	provisionedClusterInfo, err1 := r.getProvisionedClusterInfo(ctx, customCluster)
	if err1 != nil {
		log.Error(err1, "failed to get provisioned cluster Info from configmap")
		return ctrl.Result{RequeueAfter: RequeueAfter}, err1
	}

	// Handle worker nodes scaling.
	// By comparing desiredClusterInfo and provisionedClusterInfo to decide whether to proceed reconcileScaleUp or reconcileScaleDown.
	scaleUpWorkerNodes := findScaleUpWorkerNodes(provisionedClusterInfo.WorkerNodes, desiredClusterInfo.WorkerNodes)
	scaleDownWorkerNodes := findScaleDownWorkerNodes(provisionedClusterInfo.WorkerNodes, desiredClusterInfo.WorkerNodes)
	if (phase == v1alpha1.ProvisionedPhase && len(scaleUpWorkerNodes) != 0) || phase == v1alpha1.ScalingUpPhase {
		return r.reconcileScaleUp(ctx, customCluster, scaleUpWorkerNodes)
	}
	if (phase == v1alpha1.ProvisionedPhase && len(scaleDownWorkerNodes) != 0) || phase == v1alpha1.ScalingDownPhase {
		return r.reconcileScaleDown(ctx, customCluster, customMachine, scaleDownWorkerNodes)
	}

	// Handle cluster version upgrading.
	// desiredVersion is the one recorded in customCluster.spec.upgradeK8sVersion.
	desiredVersion := desiredClusterInfo.KubeVersion
	// provisionedVersion is the one recorded in cm cluster-config.data.kube_version.
	provisionedVersion := provisionedClusterInfo.KubeVersion
	if len(desiredVersion) != 0 {
		minVersion := kubesprayImageSupportMap[DefaultKubesprayVersion].MinK8sVersion
		maxVersion := kubesprayImageSupportMap[DefaultKubesprayVersion].MaxK8sVersion
		midVersion := kubesprayImageSupportMap[DefaultKubesprayVersion].MidK8sVersion
		// If the desired version is not within a specified range of versions, return directly.
		if !isSupportedVersion(desiredVersion, minVersion, maxVersion) {
			log.Error(fmt.Errorf("the kubespray image version is %s. So the minimum supported version of Kubernetes is %s, and the maximum is %s. However, the desired version of Kubernetes is %s ", DefaultKubesprayVersion, minVersion, maxVersion, desiredVersion), "")
			return ctrl.Result{}, nil
		}
		// By comparing desiredClusterInfo.kubeVersion and provisionedClusterInfo.kubeVersion to decide whether to proceed reconcileUpgrade.
		if (phase == v1alpha1.ProvisionedPhase && desiredVersion != provisionedVersion) || phase == v1alpha1.UpgradingPhase {
			// targetVersion may be equal to the desiredVersion, or it may be an intermediate version, depending on whether the minor version difference is at most 1
			targetVersion := calculateTargetVersion(provisionedVersion, desiredVersion, midVersion)
			return r.reconcileUpgrade(ctx, customCluster, kcp, targetVersion)
		}
	}

	return ctrl.Result{}, nil
}

// reconcileHandleProvisioning determine whether customCluster enter Provisioned phase or ProvisionFailed phase when current phase is Provisioning.
func (r *CustomClusterController) reconcileHandleProvisioning(ctx context.Context, customCluster *v1alpha1.CustomCluster, customMachine *v1alpha1.CustomMachine, cluster *clusterv1.Cluster, kcp *controlplanev1.KubeadmControlPlane) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)

	initWorker := &corev1.Pod{}
	initWorkerKey := generateWorkerKey(customCluster, CustomClusterInitAction)
	if err := r.Client.Get(ctx, initWorkerKey, initWorker); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("init worker does not exist, turn to reconcileCustomClusterInit to create a new one", "worker", initWorkerKey)
			return r.reconcileCustomClusterInit(ctx, customCluster, customMachine, cluster, kcp)
		}
		log.Error(err, "failed to get init worker", "worker", initWorkerKey)
		return ctrl.Result{RequeueAfter: RequeueAfter}, err
	}

	if initWorker.Status.Phase == corev1.PodSucceeded {
		customCluster.Status.Phase = v1alpha1.ProvisionedPhase
		if err := r.Status().Update(ctx, customCluster); err != nil {
			log.Error(err, "failed to update customCluster status", "customCluster", customCluster.Name)
			return ctrl.Result{RequeueAfter: RequeueAfter}, err
		}
		log.Info("customCluster's phase changes from Provisioning to Provisioned")
		return ctrl.Result{}, nil
	}

	if initWorker.Status.Phase == corev1.PodFailed {
		customCluster.Status.Phase = v1alpha1.ProvisionFailedPhase
		if err := r.Status().Update(ctx, customCluster); err != nil {
			log.Error(err, "failed to update customCluster status", "customCluster", customCluster.Name)
			return ctrl.Result{RequeueAfter: RequeueAfter}, err
		}
		log.Info("customCluster's phase changes from Provisioning to ProvisionFailed")
		return ctrl.Result{}, nil
	}
	return ctrl.Result{}, nil
}

// reconcileScaleUp is responsible for handling the customCluster reconciliation process when worker nodes need to be scaled up.
func (r *CustomClusterController) reconcileScaleUp(ctx context.Context, customCluster *v1alpha1.CustomCluster, scaleUpWorkerNodes []NodeInfo) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)

	// Create a temporary configmap that represents the desired state to create the scaleUp pod.
	if _, err := r.ensureScaleUpHostsCreated(ctx, customCluster, scaleUpWorkerNodes); err != nil {
		log.Error(err, "failed to ensure that scaleUp configmap is created ", "customCluster", customCluster.Name)
		return ctrl.Result{RequeueAfter: RequeueAfter}, err
	}

	// Create the scaleUp pod
	workerPod, err1 := r.ensureWorkerPodCreated(ctx, customCluster, CustomClusterScaleUpAction, KubesprayScaleUpCMD, generateScaleUpHostsName(customCluster), generateClusterConfigName(customCluster))
	if err1 != nil {
		log.Error(err1, "failed to ensure that scaleUp WorkerPod is created ", "customCluster", customCluster.Name)
		return ctrl.Result{RequeueAfter: RequeueAfter}, err1
	}

	// Check the current customCluster status
	if customCluster.Status.Phase != v1alpha1.ScalingUpPhase {
		customCluster.Status.Phase = v1alpha1.ScalingUpPhase
		if err := r.Status().Update(ctx, customCluster); err != nil {
			log.Error(err, "failed to update customCluster status", "customCluster", customCluster.Name)
			return ctrl.Result{RequeueAfter: RequeueAfter}, err
		}
		log.Info(fmt.Sprintf("customCluster's phase changes from %s to %s", string(v1alpha1.ProvisionedPhase), string(v1alpha1.ScalingUpPhase)))
	}

	// Determine the progress of scaling based on the status of the workerPod
	if workerPod.Status.Phase == corev1.PodSucceeded {
		// Update cluster nodes to ensure that the current cluster-host represents the cluster after the scaling.
		if err := r.updateClusterNodes(ctx, customCluster, scaleUpWorkerNodes); err != nil {
			log.Error(err, "failed to update cluster nodes")
			return ctrl.Result{RequeueAfter: RequeueAfter}, err
		}

		// The scale up process is completed by restoring the workerPod's status to "provisioned".
		customCluster.Status.Phase = v1alpha1.ProvisionedPhase
		if err := r.Status().Update(ctx, customCluster); err != nil {
			log.Error(err, "failed to update customCluster status", "customCluster", customCluster.Name)
			return ctrl.Result{RequeueAfter: RequeueAfter}, err
		}
		log.Info(fmt.Sprintf("customCluster's phase changes from %s to %s", string(v1alpha1.UnknownPhase), string(v1alpha1.ProvisionedPhase)))

		// Delete the temporary scaleUp cm
		if err := r.ensureConfigMapDeleted(ctx, generateScaleUpHostsKey(customCluster)); err != nil {
			log.Error(err, "failed to delete scaleUp configmap", "configmap", generateScaleUpHostsKey(customCluster))
			return ctrl.Result{RequeueAfter: RequeueAfter}, err
		}

		// Delete the scaleUp worker
		if err := r.ensureWorkerPodDeleted(ctx, generateWorkerKey(customCluster, CustomClusterScaleUpAction)); err != nil {
			log.Error(err, "failed to delete scaleUp worker pod", "worker", generateWorkerKey(customCluster, CustomClusterScaleUpAction))
			return ctrl.Result{RequeueAfter: RequeueAfter}, err
		}

		return ctrl.Result{}, nil
	}

	if workerPod.Status.Phase == corev1.PodFailed {
		customCluster.Status.Phase = v1alpha1.UnknownPhase
		if err := r.Status().Update(ctx, customCluster); err != nil {
			log.Error(err, "failed to update customCluster status", "customCluster", customCluster.Name)
			return ctrl.Result{RequeueAfter: RequeueAfter}, err
		}
		log.Info(fmt.Sprintf("customCluster's phase changes from %s to %s", string(v1alpha1.ScalingUpPhase), string(v1alpha1.UnknownPhase)))
		return ctrl.Result{}, nil
	}
	return ctrl.Result{}, nil
}

// reconcileScaleDown is responsible for handling the customCluster reconciliation process when worker nodes need to be scaled down.
func (r *CustomClusterController) reconcileScaleDown(ctx context.Context, customCluster *v1alpha1.CustomCluster, customMachine *v1alpha1.CustomMachine, scaleDownWorkerNodes []NodeInfo) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)

	// Checks whether the worker node for scaling down already exists. If it does not exist, the function creates it.
	workerPod, err1 := r.ensureWorkerPodCreated(ctx, customCluster, CustomClusterScaleDownAction, generateScaleDownManageCMD(scaleDownWorkerNodes), generateClusterHostsName(customCluster), generateClusterConfigName(customCluster))
	if err1 != nil {
		log.Error(err1, "failed to ensure that scaleDown WorkerPod is created", "customCluster", customCluster.Name)
		return ctrl.Result{RequeueAfter: RequeueAfter}, err1
	}

	// Check the current customCluster status.
	if customCluster.Status.Phase != v1alpha1.ScalingDownPhase {
		customCluster.Status.Phase = v1alpha1.ScalingDownPhase
		if err := r.Status().Update(ctx, customCluster); err != nil {
			log.Error(err, "failed to update customCluster status", "customCluster", customCluster.Name)
			return ctrl.Result{RequeueAfter: RequeueAfter}, err
		}
		log.Info(fmt.Sprintf("customCluster's phase changes from %s to %s", string(v1alpha1.ProvisionedPhase), string(v1alpha1.ScalingDownPhase)))
	}

	// Determine the progress of scaling based on the status of the workerPod.
	if workerPod.Status.Phase == corev1.PodSucceeded {
		// Recreate the cluster-host to ensure that the current cluster-host represents the cluster after the deletion.
		if _, err := r.recreateClusterHosts(ctx, customCluster, customMachine); err != nil {
			log.Error(err, "failed to recreate configmap cluster-hosts when scale down")
			return ctrl.Result{RequeueAfter: RequeueAfter}, err
		}

		// The scale down process is completed by restoring the workerPod's status to "provisioned".
		customCluster.Status.Phase = v1alpha1.ProvisionedPhase
		if err := r.Status().Update(ctx, customCluster); err != nil {
			log.Error(err, "failed to update customCluster status", "customCluster", customCluster.Name)
			return ctrl.Result{RequeueAfter: RequeueAfter}, err
		}
		log.Info(fmt.Sprintf("customCluster's phase changes from %s to %s", string(v1alpha1.UnknownPhase), string(v1alpha1.ProvisionedPhase)))

		// Delete the scaleDown worker
		if err := r.ensureWorkerPodDeleted(ctx, generateWorkerKey(customCluster, CustomClusterScaleDownAction)); err != nil {
			log.Error(err, "failed to delete scaleDown worker pod", "worker", generateWorkerKey(customCluster, CustomClusterScaleDownAction))
			return ctrl.Result{RequeueAfter: RequeueAfter}, err
		}

		return ctrl.Result{}, nil
	}

	if workerPod.Status.Phase == corev1.PodFailed {
		customCluster.Status.Phase = v1alpha1.UnknownPhase
		if err := r.Status().Update(ctx, customCluster); err != nil {
			log.Error(err, "failed to update customCluster status", "customCluster", customCluster.Name)
			return ctrl.Result{RequeueAfter: RequeueAfter}, err
		}
		log.Info(fmt.Sprintf("customCluster's phase changes from %s to %s", string(v1alpha1.ScalingDownPhase), string(v1alpha1.UnknownPhase)))
		return ctrl.Result{}, nil
	}

	return ctrl.Result{}, nil
}

// reconcileUpgrade is responsible for handling the customCluster reconciliation process of cluster upgrading to targetVersion
func (r *CustomClusterController) reconcileUpgrade(ctx context.Context, customCluster *v1alpha1.CustomCluster, kcp *controlplanev1.KubeadmControlPlane, targetVersion string) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)

	cmd := generateUpgradeManageCMD(targetVersion)
	// Checks whether the worker node for upgrading already exists. If it does not exist, then create it.
	workerPod, err1 := r.ensureWorkerPodCreated(ctx, customCluster, CustomClusterUpgradeAction, cmd, generateClusterHostsName(customCluster), generateClusterConfigName(customCluster))
	if err1 != nil {
		log.Error(err1, "failed to ensure that upgrade WorkerPod is created", "customCluster", customCluster.Name)
		return ctrl.Result{RequeueAfter: RequeueAfter}, err1
	}

	// Check the current customCluster status.
	if customCluster.Status.Phase != v1alpha1.UpgradingPhase {
		customCluster.Status.Phase = v1alpha1.UpgradingPhase
		if err := r.Status().Update(ctx, customCluster); err != nil {
			log.Error(err, "failed to update customCluster status", "customCluster", customCluster.Name)
			return ctrl.Result{RequeueAfter: RequeueAfter}, err
		}
		log.Info(fmt.Sprintf("customCluster's phase changes from %s to %s", string(v1alpha1.ProvisionedPhase), string(v1alpha1.UpgradingPhase)))
	}

	// Determine the progress of upgrading based on the status of the workerPod.
	if workerPod.Status.Phase == corev1.PodSucceeded {
		// Update the cluster-config and kcp to ensure that the current cluster-config and kcp represents the cluster after upgrading.
		if err := r.updateKubeVersion(ctx, customCluster, kcp, targetVersion); err != nil {
			log.Error(err, "failed to update the kubeVersion of configmap cluster-config after upgrading")
			return ctrl.Result{RequeueAfter: RequeueAfter}, err
		}

		// Restore the workerPod's status to "provisioned" after upgrading.
		customCluster.Status.Phase = v1alpha1.ProvisionedPhase
		if err := r.Status().Update(ctx, customCluster); err != nil {
			log.Error(err, "failed to update customCluster status", "customCluster", customCluster.Name)
			return ctrl.Result{RequeueAfter: RequeueAfter}, err
		}

		// Delete the upgrading worker.
		if err := r.ensureWorkerPodDeleted(ctx, generateWorkerKey(customCluster, CustomClusterUpgradeAction)); err != nil {
			log.Error(err, "failed to delete upgrade worker pod", "worker", generateWorkerKey(customCluster, CustomClusterUpgradeAction))
			return ctrl.Result{RequeueAfter: RequeueAfter}, err
		}

		return ctrl.Result{}, nil
	}

	if workerPod.Status.Phase == corev1.PodFailed {
		// Restore the workerPod's status to "provisioned" to ensure it can be upgraded again once the current upgrade error worker is deleted.
		customCluster.Status.Phase = v1alpha1.ProvisionedPhase
		if err := r.Status().Update(ctx, customCluster); err != nil {
			log.Error(err, "failed to update customCluster status", "customCluster", customCluster.Name)
			return ctrl.Result{RequeueAfter: RequeueAfter}, err
		}
		return ctrl.Result{}, nil
	}

	return ctrl.Result{}, nil
}

// reconcileHandleDeleting determine whether customCluster go to reconcileDeleteResource or enter DeletingFailed phase when current phase is Deleting.
func (r *CustomClusterController) reconcileHandleDeleting(ctx context.Context, customCluster *v1alpha1.CustomCluster, customMachine *v1alpha1.CustomMachine, kcp *controlplanev1.KubeadmControlPlane) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)

	terminateWorker := &corev1.Pod{}
	terminateWorkerKey := generateWorkerKey(customCluster, CustomClusterTerminateAction)

	if err := r.Client.Get(ctx, terminateWorkerKey, terminateWorker); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("terminate worker does not exist, turn to reconcileVMsTerminate to create a new one", "worker", terminateWorkerKey)
			return r.reconcileVMsTerminate(ctx, customCluster)
		}
		log.Error(err, "failed to get terminate worker. maybe it has been deleted", "worker", terminateWorkerKey)
		return ctrl.Result{RequeueAfter: RequeueAfter}, err
	}

	if terminateWorker.Status.Phase == corev1.PodSucceeded {
		log.Info("terminating worker was completed successfully, then we need delete the related CRD")
		// After k8s cluster on VMs has been reset successful, we need delete the related CRD.
		return r.reconcileDeleteResource(ctx, customCluster, customMachine, kcp)
	}

	if terminateWorker.Status.Phase == corev1.PodFailed {
		customCluster.Status.Phase = v1alpha1.UnknownPhase
		if err := r.Status().Update(ctx, customCluster); err != nil {
			log.Error(err, "failed to update customCluster status", "customCluster", customCluster.Name)
			return ctrl.Result{RequeueAfter: RequeueAfter}, err
		}
		log.Info("customCluster's phase changes from Deleting to DeletingFailed")
		return ctrl.Result{}, nil
	}
	return ctrl.Result{}, nil
}

// reconcileVMsTerminate uninstall the k8s cluster on VMs.
func (r *CustomClusterController) reconcileVMsTerminate(ctx context.Context, customCluster *v1alpha1.CustomCluster) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)

	// Delete the init worker.
	if err := r.ensureWorkerPodDeleted(ctx, generateWorkerKey(customCluster, CustomClusterInitAction)); err != nil {
		log.Error(err, "failed to delete init worker", "customCluster", customCluster.Name)
		return ctrl.Result{RequeueAfter: RequeueAfter}, err
	}

	// Delete the scaleUp worker.
	if err := r.ensureWorkerPodDeleted(ctx, generateWorkerKey(customCluster, CustomClusterScaleUpAction)); err != nil {
		log.Error(err, "failed to delete scaleUp worker", "customCluster", customCluster.Name)
		return ctrl.Result{RequeueAfter: RequeueAfter}, err
	}

	// Delete the scaleDown worker.
	if err := r.ensureWorkerPodDeleted(ctx, generateWorkerKey(customCluster, CustomClusterScaleDownAction)); err != nil {
		log.Error(err, "failed to delete scaleDown worker", "customCluster", customCluster.Name)
		return ctrl.Result{RequeueAfter: RequeueAfter}, err
	}

	// Delete the upgrade worker.
	if err := r.ensureWorkerPodDeleted(ctx, generateWorkerKey(customCluster, CustomClusterUpgradeAction)); err != nil {
		log.Error(err, "failed to delete upgrade worker", "customCluster", customCluster.Name)
		return ctrl.Result{RequeueAfter: RequeueAfter}, err
	}

	// Create the termination worker.
	if _, err1 := r.ensureWorkerPodCreated(ctx, customCluster, CustomClusterTerminateAction, KubesprayTerminateCMD, generateClusterHostsName(customCluster), generateClusterConfigName(customCluster)); err1 != nil {
		log.Error(err1, "failed to create terminate worker", "customCluster", customCluster.Name)
		return ctrl.Result{RequeueAfter: RequeueAfter}, err1
	}

	customCluster.Status.Phase = v1alpha1.DeletingPhase
	if err := r.Status().Update(ctx, customCluster); err != nil {
		log.Error(err, "failed to update customCluster status", "customCluster", customCluster.Name)
		return ctrl.Result{RequeueAfter: RequeueAfter}, err
	}
	log.Info("customCluster's phase changes to Deleting")

	return ctrl.Result{}, nil
}

// reconcileDeleteResource delete resource related to customCluster: configmap, pod, customMachine, customCluster.
func (r *CustomClusterController) reconcileDeleteResource(ctx context.Context, customCluster *v1alpha1.CustomCluster, customMachine *v1alpha1.CustomMachine, kcp *controlplanev1.KubeadmControlPlane) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)

	// Delete the configmap cluster-hosts.
	if err := r.ensureConfigMapDeleted(ctx, generateClusterHostsKey(customCluster)); err != nil {
		log.Error(err, "failed to ensure that configmap is deleted", "configmap", generateClusterHostsKey(customCluster))
		return ctrl.Result{RequeueAfter: RequeueAfter}, err
	}

	// Delete the configmap cluster-config.
	if err := r.ensureConfigMapDeleted(ctx, generateClusterConfigKey(customCluster)); err != nil {
		log.Error(err, "failed to ensure that configmap is deleted", "configmap", generateClusterConfigKey(customCluster))
		return ctrl.Result{RequeueAfter: RequeueAfter}, err
	}

	// Remove finalizer of customMachine.
	controllerutil.RemoveFinalizer(customMachine, CustomClusterFinalizer)
	if err := r.Client.Update(ctx, customMachine); err != nil && !apierrors.IsNotFound(err) {
		log.Error(err, "failed to remove finalizer of customMachine", "customMachine", customMachine.Name)
		return ctrl.Result{RequeueAfter: RequeueAfter}, err
	}

	// Remove finalizer of kcp.
	controllerutil.RemoveFinalizer(kcp, CustomClusterFinalizer)
	if err := r.Client.Update(ctx, kcp); err != nil && !apierrors.IsNotFound(err) {
		log.Error(err, "failed to remove finalizer of kcp", "kcp", kcp.Name)
		return ctrl.Result{RequeueAfter: RequeueAfter}, err
	}

	// Remove finalizer of customCluster. After this, cluster will be deleted completely.
	controllerutil.RemoveFinalizer(customCluster, CustomClusterFinalizer)
	if err := r.Client.Update(ctx, customCluster); err != nil && !apierrors.IsNotFound(err) {
		log.Error(err, "failed to remove finalizer of customCluster", "customCluster", customCluster.Name)
		return ctrl.Result{RequeueAfter: RequeueAfter}, err
	}

	return ctrl.Result{}, nil
}

type RelatedResource struct {
	clusterHosts  *corev1.ConfigMap
	clusterConfig *corev1.ConfigMap
	customCluster *v1alpha1.CustomCluster
	customMachine *v1alpha1.CustomMachine
	kcp           *controlplanev1.KubeadmControlPlane
}

// reconcileCustomClusterInit create an init worker for installing cluster on VMs.
func (r *CustomClusterController) reconcileCustomClusterInit(ctx context.Context, customCluster *v1alpha1.CustomCluster, customMachine *v1alpha1.CustomMachine, cluster *clusterv1.Cluster, kcp *controlplanev1.KubeadmControlPlane) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)

	var clusterHost *corev1.ConfigMap
	var errorHost error
	if clusterHost, errorHost = r.ensureClusterHostsCreated(ctx, customCluster, customMachine); errorHost != nil {
		log.Error(errorHost, "failed to update cluster-hosts configmap")
		return ctrl.Result{RequeueAfter: RequeueAfter}, errorHost
	}

	var clusterConfig *corev1.ConfigMap
	var errorConfig error
	if clusterConfig, errorConfig = r.ensureClusterConfigCreated(ctx, customCluster, customCluster, cluster, kcp); errorConfig != nil {
		log.Error(errorConfig, "failed to update cluster-config configmap")
		return ctrl.Result{RequeueAfter: RequeueAfter}, errorConfig
	}

	if _, err := r.ensureWorkerPodCreated(ctx, customCluster, CustomClusterInitAction, KubesprayInitCMD, generateClusterHostsName(customCluster), generateClusterConfigName(customCluster)); err != nil {
		log.Error(err, "failed to ensure that init WorkerPod is created ", "customCluster", customCluster.Name)
		return ctrl.Result{RequeueAfter: RequeueAfter}, err
	}

	initRelatedResource := &RelatedResource{
		clusterHosts:  clusterHost,
		clusterConfig: clusterConfig,
		customCluster: customCluster,
		customMachine: customMachine,
		kcp:           kcp,
	}
	// When all related object is ready, we need ensure object's finalizer and ownerRef is set appropriately.
	if err := r.ensureFinalizerAndOwnerRef(ctx, initRelatedResource); err != nil {
		log.Error(err, "failed to set finalizer or ownerRefs")
		return ctrl.Result{RequeueAfter: RequeueAfter}, err
	}

	customCluster.Status.Phase = v1alpha1.ProvisioningPhase
	if err := r.Status().Update(ctx, customCluster); err != nil {
		log.Error(err, "failed to update customCluster status", "customCluster", customCluster.Name)
		return ctrl.Result{RequeueAfter: RequeueAfter}, err
	}
	log.Info("customCluster's phase changes to Provisioning")

	return ctrl.Result{}, nil
}

// ensureFinalizerAndOwnerRef ensure every related resource's finalizer and ownerRef is ready.
func (r *CustomClusterController) ensureFinalizerAndOwnerRef(ctx context.Context, res *RelatedResource) error {
	controllerutil.AddFinalizer(res.customCluster, CustomClusterFinalizer)
	controllerutil.AddFinalizer(res.customMachine, CustomClusterFinalizer)
	controllerutil.AddFinalizer(res.kcp, CustomClusterFinalizer)
	controllerutil.AddFinalizer(res.clusterHosts, CustomClusterConfigMapFinalizer)
	controllerutil.AddFinalizer(res.clusterConfig, CustomClusterConfigMapFinalizer)

	ownerRefs := generateOwnerRefFromCustomCluster(res.customCluster)
	res.customMachine.OwnerReferences = []metav1.OwnerReference{ownerRefs}
	res.clusterHosts.OwnerReferences = []metav1.OwnerReference{ownerRefs}
	res.clusterConfig.OwnerReferences = []metav1.OwnerReference{ownerRefs}

	if err := r.Client.Update(ctx, res.customMachine); err != nil {
		return fmt.Errorf("failed to set finalizer or ownerRef of customMachine: %v", err)
	}

	if err := r.Client.Update(ctx, res.clusterHosts); err != nil {
		return fmt.Errorf("failed to set finalizer or ownerRef of clusterHosts: %v", err)
	}

	if err := r.Client.Update(ctx, res.clusterConfig); err != nil {
		return fmt.Errorf("failed to set finalizer or ownerRef of clusterConfig: %v", err)
	}

	if err := r.Client.Update(ctx, res.customCluster); err != nil {
		return fmt.Errorf("failed to set finalizer or ownerRef of customCluster: %v", err)
	}

	if err := r.Client.Update(ctx, res.kcp); err != nil {
		return fmt.Errorf("failed to set finalizer of kcp: %v", err)
	}

	return nil
}

// generateClusterManageWorker generate a kubespray manage worker pod from configmap.
func (r *CustomClusterController) generateClusterManageWorker(customCluster *v1alpha1.CustomCluster, manageAction customClusterManageAction, manageCMD customClusterManageCMD, hostsName, configName string) *corev1.Pod {
	podName := customCluster.Name + "-" + string(manageAction)
	namespace := customCluster.Namespace
	kubesprayImage := getKubesprayImage(DefaultKubesprayVersion)
	defaultMode := int32(0o600)

	managerWorker := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      podName,
		},
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},

		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:    podName,
					Image:   kubesprayImage,
					Command: []string{"/bin/sh", "-c"},
					Args:    []string{string(manageCMD)},

					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      ClusterHostsName,
							MountPath: "/kubespray/inventory",
						},
						{
							Name:      ClusterConfigName,
							MountPath: "/kubespray/inventory/group_vars/all",
						},
						{
							Name:      SecreteName,
							MountPath: "/root/.ssh",
							ReadOnly:  true,
						},
					},
				},
			},

			Volumes: []corev1.Volume{
				{
					Name: ClusterHostsName,
					VolumeSource: corev1.VolumeSource{
						ConfigMap: &corev1.ConfigMapVolumeSource{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: hostsName,
							},
						},
					},
				},
				{
					Name: ClusterConfigName,
					VolumeSource: corev1.VolumeSource{
						ConfigMap: &corev1.ConfigMapVolumeSource{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: configName,
							},
						},
					},
				},
				{
					Name: SecreteName,
					VolumeSource: corev1.VolumeSource{
						Secret: &corev1.SecretVolumeSource{
							SecretName:  SecreteName,
							DefaultMode: &defaultMode,
						},
					},
				},
			},
			RestartPolicy: corev1.RestartPolicyNever,
		},
	}
	return managerWorker
}

type HostTemplateContent struct {
	NodeAndIP    []string
	MasterName   []string
	NodeName     []string
	EtcdNodeName []string // default: NodeName + MasterName
}

type ConfigTemplateContent struct {
	KubeVersion string
	PodCIDR     string
	// CNIType is the CNI plugin of the cluster on VMs. The default plugin is calico and can be ["calico", "cilium", "canal", "flannel"]
	CNIType string
	// TODO: support other kubernetes configs
}

func GetConfigContent(c *clusterv1.Cluster, kcp *controlplanev1.KubeadmControlPlane, cc *v1alpha1.CustomCluster) *ConfigTemplateContent {
	// Add kubespray init config here
	configContent := &ConfigTemplateContent{
		PodCIDR:     c.Spec.ClusterNetwork.Pods.CIDRBlocks[0],
		KubeVersion: kcp.Spec.Version,
		CNIType:     cc.Spec.CNI.Type,
	}
	return configContent
}

func GetHostsContent(customMachine *v1alpha1.CustomMachine) *HostTemplateContent {
	masterMachine := customMachine.Spec.Master
	nodeMachine := customMachine.Spec.Nodes
	hostVar := &HostTemplateContent{
		NodeAndIP:    make([]string, len(masterMachine)+len(nodeMachine)),
		MasterName:   make([]string, len(masterMachine)),
		NodeName:     make([]string, len(nodeMachine)),
		EtcdNodeName: make([]string, len(masterMachine)),
	}

	count := 0
	for i, machine := range masterMachine {
		masterName := machine.HostName
		nodeAndIp := fmt.Sprintf("%s ansible_host=%s ip=%s", machine.HostName, machine.PublicIP, machine.PrivateIP)
		hostVar.MasterName[i] = masterName
		hostVar.EtcdNodeName[count] = masterName
		hostVar.NodeAndIP[count] = nodeAndIp
		count++
	}
	for i, machine := range nodeMachine {
		nodeName := machine.HostName
		nodeAndIp := fmt.Sprintf("%s ansible_host=%s ip=%s", machine.HostName, machine.PublicIP, machine.PrivateIP)
		hostVar.NodeName[i] = nodeName
		hostVar.NodeAndIP[count] = nodeAndIp
		count++
	}

	return hostVar
}

func (r *CustomClusterController) CreateConfigMapWithTemplate(ctx context.Context, name, namespace, fileName, configMapData string) (*corev1.ConfigMap, error) {
	cm := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Data: map[string]string{fileName: strings.TrimSpace(configMapData)},
	}

	if err := r.Client.Create(ctx, cm); err != nil {
		return nil, err
	}
	return cm, nil
}

// recreateClusterHosts delete current clusterHosts configmap and create a new one with latest customMachine configuration.
func (r *CustomClusterController) recreateClusterHosts(ctx context.Context, customCluster *v1alpha1.CustomCluster, customMachine *v1alpha1.CustomMachine) (*corev1.ConfigMap, error) {
	// Delete the configmap cluster-hosts.
	if err := r.ensureConfigMapDeleted(ctx, generateClusterHostsKey(customCluster)); err != nil {
		return nil, err
	}
	return r.CreateClusterHosts(ctx, customMachine, customCluster)
}

func (r *CustomClusterController) CreateClusterHosts(ctx context.Context, customMachine *v1alpha1.CustomMachine, customCluster *v1alpha1.CustomCluster) (*corev1.ConfigMap, error) {
	hostsContent := GetHostsContent(customMachine)
	hostData := &strings.Builder{}

	// todo: split this to a separated file
	tmpl := template.Must(template.New("").Parse(`
[all]
{{ range $v := .NodeAndIP }}
{{ $v }}
{{ end }}
[kube_control_plane]
{{ range $v := .MasterName }}
{{ $v }}
{{ end }}
[etcd]
{{- range $v := .EtcdNodeName }}
{{ $v }}
{{ end }}
[kube_node]
{{- range $v := .NodeName }}
{{ $v }}
{{ end }}
[k8s-cluster:children]
kube_node
kube_control_plane
`))

	if err := tmpl.Execute(hostData, hostsContent); err != nil {
		return nil, err
	}
	name := generateClusterHostsName(customCluster)
	namespace := customCluster.Namespace

	return r.CreateConfigMapWithTemplate(ctx, name, namespace, ClusterHostsName, hostData.String())
}

func (r *CustomClusterController) CreateClusterConfig(ctx context.Context, c *clusterv1.Cluster, kcp *controlplanev1.KubeadmControlPlane, cc *v1alpha1.CustomCluster) (*corev1.ConfigMap, error) {
	configContent := GetConfigContent(c, kcp, cc)
	configData := &strings.Builder{}

	// todo: split this to a separated file
	tmpl := template.Must(template.New("").Parse(`
kube_version: {{ .KubeVersion}}
download_run_once: true
download_container: false
download_localhost: true
# network
kube_pods_subnet: {{ .PodCIDR }}
kube_network_plugin: {{ .CNIType }}

`))

	if err := tmpl.Execute(configData, configContent); err != nil {
		return nil, err
	}
	name := generateClusterConfigName(cc)
	namespace := cc.Namespace

	return r.CreateConfigMapWithTemplate(ctx, name, namespace, ClusterConfigName, configData.String())
}

// createScaleUpConfigMap create temporary cluster-hosts configmap for scaling.
func (r *CustomClusterController) createScaleUpConfigMap(ctx context.Context, customCluster *v1alpha1.CustomCluster, scaleUpWorkerNodes []NodeInfo) (*corev1.ConfigMap, error) {
	// get current cm
	curCM := &corev1.ConfigMap{}
	if err := r.Client.Get(ctx, generateClusterHostsKey(customCluster), curCM); err != nil {
		return nil, err
	}

	newCM := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      generateScaleUpHostsName(customCluster),
			Namespace: customCluster.Namespace,
		},
		Data: map[string]string{ClusterHostsName: strings.TrimSpace(getScaleUpConfigMapData(curCM.Data[ClusterHostsName], scaleUpWorkerNodes))},
	}

	if err := r.Client.Create(ctx, newCM); err != nil {
		return nil, err
	}
	return newCM, nil
}

// ensureScaleUpHostsCreated ensure that the temporary cluster-hosts configmap for scaling up is created.
func (r *CustomClusterController) ensureScaleUpHostsCreated(ctx context.Context, customCluster *v1alpha1.CustomCluster, scaleUpWorkerNodes []NodeInfo) (*corev1.ConfigMap, error) {
	cmKey := generateScaleUpHostsKey(customCluster)
	cm := &corev1.ConfigMap{}
	if err := r.Client.Get(ctx, cmKey, cm); err != nil {
		if apierrors.IsNotFound(err) {
			return r.createScaleUpConfigMap(ctx, customCluster, scaleUpWorkerNodes)
		}
		return nil, err
	}
	return cm, nil
}

// ensureClusterHostsCreated ensure that the cluster-hosts configmap is created.
func (r *CustomClusterController) ensureClusterHostsCreated(ctx context.Context, customCluster *v1alpha1.CustomCluster, customMachine *v1alpha1.CustomMachine) (*corev1.ConfigMap, error) {
	cmKey := generateClusterHostsKey(customCluster)
	cm := &corev1.ConfigMap{}
	if err := r.Client.Get(ctx, cmKey, cm); err != nil {
		if apierrors.IsNotFound(err) {
			return r.CreateClusterHosts(ctx, customMachine, customCluster)
		}
		return nil, err
	}
	return cm, nil
}

// ensureClusterConfigCreated ensure that the cluster-config configmap is created.
func (r *CustomClusterController) ensureClusterConfigCreated(ctx context.Context, customCluster *v1alpha1.CustomCluster, cc *v1alpha1.CustomCluster, cluster *clusterv1.Cluster, kcp *controlplanev1.KubeadmControlPlane) (*corev1.ConfigMap, error) {
	cmKey := generateClusterConfigKey(customCluster)
	cm := &corev1.ConfigMap{}
	if err := r.Client.Get(ctx, cmKey, cm); err != nil {
		if apierrors.IsNotFound(err) {
			return r.CreateClusterConfig(ctx, cluster, kcp, cc)
		}
		return nil, err
	}
	return cm, nil
}

// findScaleUpWorkerNodes find the workerNodes which need to be scale up
func findScaleUpWorkerNodes(provisionedWorkerNodes, curWorkerNodes []NodeInfo) []NodeInfo {
	return findAdditionalWorkerNodes(provisionedWorkerNodes, curWorkerNodes)
}

// findScaleDownWorkerNodes find the workerNodes which need to be scale down
func findScaleDownWorkerNodes(provisionedWorkerNodes, curWorkerNodes []NodeInfo) []NodeInfo {
	return findAdditionalWorkerNodes(curWorkerNodes, provisionedWorkerNodes)
}

// findAdditionalWorkerNodes find additional workers in secondWorkersNodes than firstWorkerNodes
func findAdditionalWorkerNodes(firstWorkerNodes, secondWorkersNodes []NodeInfo) []NodeInfo {
	var additionalWorkers []NodeInfo
	if len(secondWorkersNodes) == 0 {
		return nil
	}
	if len(firstWorkerNodes) == 0 {
		additionalWorkers = append(additionalWorkers, secondWorkersNodes...)
		return additionalWorkers
	}
	var set = make(map[string]struct{})

	for _, firstNode := range firstWorkerNodes {
		set[firstNode.NodeName] = struct{}{}
	}

	for _, secondNode := range secondWorkersNodes {
		if _, ok := set[secondNode.NodeName]; !ok {
			additionalWorkers = append(additionalWorkers, secondNode)
		}
	}

	return additionalWorkers
}

func (r *CustomClusterController) WorkerToCustomClusterMapFunc(o client.Object) []ctrl.Request {
	c, ok := o.(*corev1.Pod)
	if !ok {
		panic(fmt.Sprintf("Expected a Cluster but got a %T", o))
	}
	for _, owner := range c.GetOwnerReferences() {
		if owner.Kind == CustomClusterKind {
			return []ctrl.Request{{NamespacedName: client.ObjectKey{Namespace: c.Namespace, Name: owner.Name}}}
		}
	}
	return nil
}

func (r *CustomClusterController) CustomMachineToCustomClusterMapFunc(o client.Object) []ctrl.Request {
	c, ok := o.(*v1alpha1.CustomMachine)
	if !ok {
		panic(fmt.Sprintf("Expected a CustomMachine but got a %T", o))
	}

	var result []ctrl.Request
	for _, owner := range c.GetOwnerReferences() {
		if owner.Kind == CustomClusterKind {
			name := client.ObjectKey{Namespace: c.GetNamespace(), Name: owner.Name}
			result = append(result, ctrl.Request{NamespacedName: name})
			break
		}
	}
	return result
}

func (r *CustomClusterController) ClusterToCustomClusterMapFunc(o client.Object) []ctrl.Request {
	c, ok := o.(*clusterv1.Cluster)
	if !ok {
		panic(fmt.Sprintf("Expected a Cluster but got a %T", o))
	}
	infrastructureRef := c.Spec.InfrastructureRef
	if infrastructureRef != nil && infrastructureRef.Kind == CustomClusterKind {
		return []ctrl.Request{{NamespacedName: client.ObjectKey{Namespace: infrastructureRef.Namespace, Name: infrastructureRef.Name}}}
	}
	return nil
}

func (r *CustomClusterController) KcpToCustomClusterMapFunc(o client.Object) []ctrl.Request {
	c, ok := o.(*controlplanev1.KubeadmControlPlane)
	if !ok {
		panic(fmt.Sprintf("Expected a KubeadmControlPlane but got a %T", o))
	}
	var result []ctrl.Request

	// Find the cluster from kcp.
	clusterKey := client.ObjectKey{}
	for _, owner := range c.GetOwnerReferences() {
		if owner.Kind == ClusterKind {
			clusterKey = client.ObjectKey{Namespace: c.GetNamespace(), Name: owner.Name}
			break
		}
	}
	ownerCluster := &clusterv1.Cluster{}
	if err := r.Client.Get(context.TODO(), clusterKey, ownerCluster); err != nil {
		return nil
	}

	// Find the customCluster from cluster.
	infrastructureRef := ownerCluster.Spec.InfrastructureRef
	if infrastructureRef != nil && infrastructureRef.Kind == CustomClusterKind {
		return []ctrl.Request{{NamespacedName: client.ObjectKey{Namespace: infrastructureRef.Namespace, Name: infrastructureRef.Name}}}
	}

	return result
}

func generateWorkerKey(customCluster *v1alpha1.CustomCluster, action customClusterManageAction) client.ObjectKey {
	return client.ObjectKey{
		Namespace: customCluster.Namespace,
		Name:      customCluster.Name + "-" + string(action),
	}
}

func generateClusterHostsKey(customCluster *v1alpha1.CustomCluster) client.ObjectKey {
	return client.ObjectKey{
		Namespace: customCluster.Namespace,
		Name:      generateClusterHostsName(customCluster),
	}
}

func generateClusterConfigKey(customCluster *v1alpha1.CustomCluster) client.ObjectKey {
	return client.ObjectKey{
		Namespace: customCluster.Namespace,
		Name:      generateClusterConfigName(customCluster),
	}
}

func generateClusterHostsName(customCluster *v1alpha1.CustomCluster) string {
	return customCluster.Name + "-" + ClusterHostsName
}

func generateClusterConfigName(customCluster *v1alpha1.CustomCluster) string {
	return customCluster.Name + "-" + ClusterConfigName
}

func generateScaleUpHostsKey(customCluster *v1alpha1.CustomCluster) client.ObjectKey {
	return client.ObjectKey{
		Namespace: customCluster.Namespace,
		Name:      generateScaleUpHostsName(customCluster),
	}
}

func generateScaleUpHostsName(customCluster *v1alpha1.CustomCluster) string {
	return customCluster.Name + "-" + ClusterHostsName + "-scale-up"
}

func generateOwnerRefFromCustomCluster(customCluster *v1alpha1.CustomCluster) metav1.OwnerReference {
	return metav1.OwnerReference{
		APIVersion: customCluster.APIVersion,
		Kind:       customCluster.Kind,
		Name:       customCluster.Name,
		UID:        customCluster.UID,
	}
}

// generateScaleDownManageCMD generate a kubespray cmd to delete the node from the list of nodesNeedDelete
func generateScaleDownManageCMD(nodesNeedDelete []NodeInfo) customClusterManageCMD {
	if len(nodesNeedDelete) == 0 {
		return ""
	}
	cmd := string(KubesprayScaleDownCMDPrefix) + " --extra-vars \"node=" + nodesNeedDelete[0].NodeName
	if len(nodesNeedDelete) == 1 {
		return customClusterManageCMD(cmd + "\" ")
	}

	for i := 1; i < len(nodesNeedDelete); i++ {
		cmd = cmd + "," + nodesNeedDelete[i].NodeName
	}

	return customClusterManageCMD(cmd + "\" ")
}

// generateScaleDownManageCMD generate a kubespray cmd to upgrade cluster to desired kubeVersion.
func generateUpgradeManageCMD(kubeVersion string) customClusterManageCMD {
	if len(kubeVersion) == 0 {
		return ""
	}

	cmd := string(KubesprayUpgradeCMDPrefix) + " -e kube_version=v" + strings.TrimPrefix(kubeVersion, "v")

	return customClusterManageCMD(cmd)
}

// getDesiredClusterInfo get desired cluster info from crd configuration。
func getDesiredClusterInfo(customMachine *v1alpha1.CustomMachine, customCluster *v1alpha1.CustomCluster) *ClusterInfo {
	workerNodes := getWorkerNodesFromCustomMachine(customMachine)

	clusterInfo := &ClusterInfo{
		WorkerNodes: workerNodes,
		KubeVersion: customCluster.Spec.UpgradeK8sVersion,
	}

	return clusterInfo
}

// getWorkerNodesFromCustomMachine takes in CustomMachine object and returns an array of workerNodes NodeInfo objects.
func getWorkerNodesFromCustomMachine(customMachine *v1alpha1.CustomMachine) []NodeInfo {
	var workerNodes []NodeInfo
	for i := 0; i < len(customMachine.Spec.Nodes); i++ {
		curNode := NodeInfo{
			NodeName:  customMachine.Spec.Nodes[i].HostName,
			PublicIP:  customMachine.Spec.Nodes[i].PublicIP,
			PrivateIP: customMachine.Spec.Nodes[i].PrivateIP,
		}
		workerNodes = append(workerNodes, curNode)
	}
	return workerNodes
}

// getProvisionedClusterInfo get the provisioned cluster info on VMs from current configmap.
func (r *CustomClusterController) getProvisionedClusterInfo(ctx context.Context, customCluster *v1alpha1.CustomCluster) (*ClusterInfo, error) {
	// get current cluster-host configMap
	clusterHost := &corev1.ConfigMap{}
	if err := r.Client.Get(ctx, generateClusterHostsKey(customCluster), clusterHost); err != nil {
		return nil, err
	}

	// get current cluster-config configMap
	clusterConfig := &corev1.ConfigMap{}
	if err := r.Client.Get(ctx, generateClusterConfigKey(customCluster), clusterConfig); err != nil {
		return nil, err
	}

	// get workerNode from cluster-host
	workerNodes := getWorkerNodeInfoFromClusterHost(clusterHost)

	// create workerNodes from cluster-host Configmap
	clusterInfo := &ClusterInfo{
		WorkerNodes: workerNodes,
		KubeVersion: getKubeVersionFromCM(clusterConfig),
	}

	return clusterInfo, nil
}

// getWorkerNodeInfoFromClusterHost get the provisioned workerNode info on VMs from the cluster-host configmap.
func getWorkerNodeInfoFromClusterHost(clusterHost *corev1.ConfigMap) []NodeInfo {
	var workerNodes []NodeInfo
	var allNodes = make(map[string]NodeInfo)

	clusterHostDate := clusterHost.Data[ClusterHostsName]
	clusterHostDate = strings.TrimSpace(clusterHostDate)

	// the regexp string depend on the template text which the function "CreateClusterHosts" use
	sep := regexp.MustCompile(`\[all]|\[kube_control_plane]|\[kube_node]|\[k8s-cluster:children]`)
	clusterHostDateArr := sep.Split(clusterHostDate, -1)

	allNodesStr := clusterHostDateArr[1]
	workerNodesStr := clusterHostDateArr[3]

	zp := regexp.MustCompile(`[\t\n\f\r]`)
	allNodeArr := zp.Split(allNodesStr, -1)
	workerNodesArr := zp.Split(workerNodesStr, -1)

	// get all nodes info
	for _, nodeStr := range allNodeArr {
		if len(nodeStr) == 0 {
			continue
		}
		nodeStr = strings.TrimSpace(nodeStr)
		curName, cruNodeInfo := getNodeInfoFromNodeStr(nodeStr)
		// deduplication
		allNodes[curName] = cruNodeInfo
	}

	// choose workerNode from all node
	for _, workerNodeName := range workerNodesArr {
		if len(workerNodeName) == 0 {
			continue
		}
		workerNodeName = strings.TrimSpace(workerNodeName)
		workerNodes = append(workerNodes, allNodes[workerNodeName])
	}

	return workerNodes
}

// getNodeInfoFromNodeStr takes a string representing a node and returns its hostname and NodeInfo
func getNodeInfoFromNodeStr(nodeStr string) (hostName string, nodeInfo NodeInfo) {
	nodeStr = strings.TrimSpace(nodeStr)
	// the sepStr depend on the template text which the function "CreateClusterHosts" use
	sepStr := regexp.MustCompile(` ansible_host=| ip=`)
	strArr := sepStr.Split(nodeStr, -1)

	hostName = strArr[0]
	publicIP := strArr[1]
	privateIP := strArr[2]

	return hostName, NodeInfo{
		NodeName:  hostName,
		PublicIP:  publicIP,
		PrivateIP: privateIP,
	}
}

// ensureConfigMapDeleted ensure that the configmap is deleted.
func (r *CustomClusterController) ensureConfigMapDeleted(ctx context.Context, cmKey client.ObjectKey) error {
	cm := &corev1.ConfigMap{}
	errGetConfigmap := r.Client.Get(ctx, cmKey, cm)
	// errGetConfigmap can be divided into three situation: isNotFound(no action); not isNotFound(return err); nil(start to delete cm).
	if apierrors.IsNotFound(errGetConfigmap) {
		return nil
	} else if errGetConfigmap != nil && !apierrors.IsNotFound(errGetConfigmap) {
		return fmt.Errorf("failed to get cm when it should be deleted: %v", errGetConfigmap)
	} else if errGetConfigmap == nil {
		controllerutil.RemoveFinalizer(cm, CustomClusterConfigMapFinalizer)
		if err := r.Client.Update(ctx, cm); err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("failed to remove finalizer of cm when it should be deleted: %v", err)
		}
		if err := r.Client.Delete(ctx, cm); err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("failed to delete cm when it should be deleted: %v", err)
		}
	}
	return nil
}

// ensureWorkerPodDeleted ensures that the worker pod is deleted.
func (r *CustomClusterController) ensureWorkerPodDeleted(ctx context.Context, workerPodKey client.ObjectKey) error {
	// Delete worker.
	worker := &corev1.Pod{}
	errGetWorker := r.Client.Get(ctx, workerPodKey, worker)
	// errGetWorker can be divided into three situation: isNotFound; not isNotFound; nil.
	if apierrors.IsNotFound(errGetWorker) {
		return nil
	} else if errGetWorker != nil && !apierrors.IsNotFound(errGetWorker) {
		return fmt.Errorf("failed to get worker pod when it should be deleted: %v", errGetWorker)
	} else if errGetWorker == nil {
		if err := r.Client.Delete(ctx, worker); err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("failed to delete cm when it should be deleted: %v", err)
		}
	}
	return nil
}

// ensureWorkerPodCreated ensure that the worker pod is created.
func (r *CustomClusterController) ensureWorkerPodCreated(ctx context.Context, customCluster *v1alpha1.CustomCluster, manageAction customClusterManageAction, manageCMD customClusterManageCMD, hostName, configName string) (*corev1.Pod, error) {
	workerPodKey := generateWorkerKey(customCluster, manageAction)
	workerPod := &corev1.Pod{}

	if err := r.Client.Get(ctx, workerPodKey, workerPod); err != nil {
		if apierrors.IsNotFound(err) {
			workerPod = r.generateClusterManageWorker(customCluster, manageAction, manageCMD, hostName, configName)
			workerPod.OwnerReferences = []metav1.OwnerReference{generateOwnerRefFromCustomCluster(customCluster)}
			if err1 := r.Client.Create(ctx, workerPod); err1 != nil {
				return nil, fmt.Errorf("failed to create customCluster manager worker pod: %v", err1)
			}
		}
		return nil, fmt.Errorf("failed to get worker pod: %v", err)
	}
	return workerPod, nil
}

// updateClusterNodes update the cluster nodes info in configmap.
func (r *CustomClusterController) updateClusterNodes(ctx context.Context, customCluster *v1alpha1.CustomCluster, scaleUpWorkerNodes []NodeInfo) error {
	// get cm
	cm := &corev1.ConfigMap{}
	if err := r.Client.Get(ctx, generateClusterHostsKey(customCluster), cm); err != nil {
		return err
	}

	// Add new nodes on the original data, instead of directly modifying it to the desired state (read from customMachine).
	// This is the basis for the automatic execution of scaleDown after scaleUp is completed when both "scaleUpWorkerNodes" and "scaleDownWorkerNodes" are not nil.
	cm.Data[ClusterHostsName] = getScaleUpConfigMapData(cm.Data[ClusterHostsName], scaleUpWorkerNodes)

	// update cm
	if err := r.Client.Update(ctx, cm); err != nil {
		return err
	}
	return nil
}

// getScaleUpConfigMapData return a string of the configmap data that adds the scaleUp nodes to the original data.
func getScaleUpConfigMapData(data string, scaleUpWorkerNodes []NodeInfo) string {
	sep := regexp.MustCompile(`\[kube_control_plane]|\[k8s-cluster:children]`)
	dateParts := sep.Split(data, -1)

	nodeAndIP := "\n"
	nodeName := "\n"

	for _, node := range scaleUpWorkerNodes {
		nodeAndIP = nodeAndIP + fmt.Sprintf("%s ansible_host=%s ip=%s\n", node.NodeName, node.PublicIP, node.PrivateIP)
		nodeName = nodeName + fmt.Sprintf("%s\n", node.NodeName)
	}

	ans := fmt.Sprintf("%s%s[kube_control_plane]%s%s[k8s-cluster:children]%s", dateParts[0], nodeAndIP, dateParts[1], nodeName, dateParts[2])

	return ans
}

// isSupportedVersion checks if a desired version is within a specified range of versions.
func isSupportedVersion(desiredVersion, minVersion, maxVersion string) bool {
	desiredVersion = strings.TrimPrefix(desiredVersion, "v")
	minVersion = strings.TrimPrefix(minVersion, "v")
	maxVersion = strings.TrimPrefix(maxVersion, "v")

	// Parse the version strings using semver package
	desired, err := semver.NewVersion(desiredVersion)
	if err != nil {
		return false
	}

	min, err1 := semver.NewVersion(minVersion)
	if err1 != nil {
		return false
	}

	max, err2 := semver.NewVersion(maxVersion)
	if err2 != nil {
		return false
	}

	// check the desiredVersion is in the correct scope .
	if desired.Compare(*min) < 0 || desired.Compare(*max) > 0 {
		return false
	}

	return true
}

// isKubeadmUpgradeSupported check if this upgrading is supported to Kubeadm. kubespray using kubeadm to upgrade, but it is not supported to skip MINOR versions during the upgrade process using Kubeadm.
func isKubeadmUpgradeSupported(originVersion, targetVersion string) bool {
	originVersion = strings.TrimPrefix(originVersion, "v")
	targetVersion = strings.TrimPrefix(targetVersion, "v")
	// Parse the version strings using semver package
	origin, err := semver.NewVersion(originVersion)
	if err != nil {
		return false
	}
	target, err1 := semver.NewVersion(targetVersion)
	if err1 != nil {
		return false
	}

	// Compare the major and minor versions
	if origin.Major != target.Major {
		return false
	}

	// Check if the minor version difference is at most 1
	if origin.Minor-target.Minor > 1 || target.Minor-origin.Minor > 1 {
		return false
	}

	return true
}

// getKubeVersionFromCM get the provisioned k8s version from the cluster-config configmap.
func getKubeVersionFromCM(clusterConfig *corev1.ConfigMap) string {
	clusterConfigDate := clusterConfig.Data[ClusterConfigName]
	clusterConfigDate = strings.TrimSpace(clusterConfigDate)

	zp := regexp.MustCompile(`[\t\n\f\r]`)
	clusterHostDateArr := zp.Split(clusterConfigDate, -1)

	for _, configStr := range clusterHostDateArr {
		if strings.HasPrefix(configStr, KubeVersionPrefix) {
			return strings.TrimSpace(strings.Replace(configStr, KubeVersionPrefix, "", -1))
		}
	}
	return ""
}

// getUpdatedKubeVersionConfigData get the configuration data that represents the upgraded version of Kubernetes.
func getUpdatedKubeVersionConfigData(clusterConfig *corev1.ConfigMap, newKubeVersion string) string {
	clusterConfigDate := strings.TrimSpace(clusterConfig.Data[ClusterConfigName])

	// add KubeVersionPrefix to avoid confusion with other configurations.
	oldStr := KubeVersionPrefix + getKubeVersionFromCM(clusterConfig)
	newStr := KubeVersionPrefix + newKubeVersion

	return strings.TrimSpace(strings.Replace(clusterConfigDate, oldStr, newStr, -1))
}

// updateKubeVersion update the kubeVersion in configmap and kcp.
func (r *CustomClusterController) updateKubeVersion(ctx context.Context, customCluster *v1alpha1.CustomCluster, kcp *controlplanev1.KubeadmControlPlane, newKubeVersion string) error {
	// get cm
	cm := &corev1.ConfigMap{}
	if err := r.Client.Get(ctx, generateClusterConfigKey(customCluster), cm); err != nil {
		return err
	}

	cm.Data[ClusterConfigName] = getUpdatedKubeVersionConfigData(cm, newKubeVersion)

	// update cm
	if err := r.Client.Update(ctx, cm); err != nil {
		return err
	}

	kcp.Spec.Version = newKubeVersion
	// update kcp
	if err := r.Client.Update(ctx, kcp); err != nil {
		return err
	}

	return nil
}

func calculateTargetVersion(provisionedVersion, desiredVersion, midVersion string) string {
	if isKubeadmUpgradeSupported(provisionedVersion, desiredVersion) {
		return desiredVersion
	}
	return midVersion
}

// getKubesprayImage take in kubesprayVersion return the kubespray image path of this version.
func getKubesprayImage(kubesprayVersion string) string {
	imagePath := "quay.io/kubespray/kubespray:" + kubesprayVersion
	return imagePath
}
