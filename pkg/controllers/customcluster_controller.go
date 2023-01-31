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
	"istio.io/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"strings"
	"text/template"
	"time"

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
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"kurator.dev/kurator/pkg/apis/cluster/v1alpha1"
)

// CustomClusterController reconciles a CustomCluster object
type CustomClusterController struct {
	client.Client
	APIReader client.Reader
	Scheme    *runtime.Scheme
}

type customClusterManageCMD string
type customClusterManageAction string

const (
	RequeueAfter      = time.Second * 5
	ClusterHostsName  = "cluster-hosts"
	ClusterConfigName = "cluster-config"
	SecreteName       = "cluster-secret"

	CustomClusterInitAction customClusterManageAction = "init"
	KubesprayInitCMD        customClusterManageCMD    = "ansible-playbook -i inventory/" + ClusterHostsName + " --private-key /root/.ssh/ssh-privatekey cluster.yml -vvv"

	CustomClusterTerminateAction customClusterManageAction = "terminate"
	KubesprayTerminateCMD        customClusterManageCMD    = "ansible-playbook -e reset_confirmation=yes -i inventory/" + ClusterHostsName + " --private-key /root/.ssh/ssh-privatekey reset.yml -vvv"

	// TODO: support custom this in CustomCluster/CustomMachine
	DefaultKubesprayImage = "quay.io/kubespray/kubespray:v2.20.0"

	WorkerLabelKey = "customClusterName"

	// CustomClusterFinalizer is the finalizer applied to crd
	CustomClusterFinalizer = "customcluster.cluster.kurator.dev"
	// custom configmap finalizer requires at least one slash
	CustomClusterConfigMapFinalizer = CustomClusterFinalizer + "/configmap"
)

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *CustomClusterController) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)

	// Fetch the customCluster instance.
	customCluster := &v1alpha1.CustomCluster{}
	if err := r.Client.Get(ctx, req.NamespacedName, customCluster); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("Could not find customCluster", "customCluster", req)
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return ctrl.Result{RequeueAfter: RequeueAfter}, err
	}
	log = log.WithValues("customCluster", klog.KObj(customCluster))
	ctx = ctrl.LoggerInto(ctx, log)

	// Fetch the Cluster instance
	clusterKey := client.ObjectKey{
		Namespace: customCluster.Spec.ClusterRef.Namespace,
		Name:      customCluster.Spec.ClusterRef.Name,
	}
	cluster := &clusterv1.Cluster{}
	if err := r.Client.Get(ctx, clusterKey, cluster); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("Could not find cluster", "cluster", clusterKey)
			return ctrl.Result{}, nil
		}
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
			log.Info("Could not find customMachine", "customMachine", customMachinekey)
			return ctrl.Result{}, nil
		}
		return ctrl.Result{RequeueAfter: RequeueAfter}, err
	}

	return r.reconcile(ctx, customCluster, customMachine, cluster)
}

// reconcile handles CustomCluster reconciliation.
func (r *CustomClusterController) reconcile(ctx context.Context, customCluster *v1alpha1.CustomCluster, customMachine *v1alpha1.CustomMachine, cluster *clusterv1.Cluster) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)
	log.Info("~~~~~~~~reconcile start")

	phase := customCluster.Status.Phase

	// if upstream cluster at pre-delete the customCluster need to be deleted
	if !cluster.DeletionTimestamp.IsZero() {
		// if customCluster is already in phase Terminating, the controller will check the terminating worker status to handle terminating status
		if phase == v1alpha1.TerminatingPhase {
			return r.reconcileHandleTerminating(ctx, customCluster, customMachine)
		}
		return r.reconcileDelete(ctx, customCluster, customMachine)
	}

	// customCluster in phase nil or initFailed will try to enter running phase by creating an init worker successfully
	if len(phase) == 0 || phase == v1alpha1.InitFailedPhase {
		return r.reconcileCustomClusterInit(ctx, customCluster, customMachine, cluster)
	}

	// if customCluster is in phase running, the controller will check the init worker status to handle Running status
	if phase == v1alpha1.RunningPhase {
		return r.reconcileHandleRunning(ctx, customCluster)
	}

	return ctrl.Result{}, nil
}

// reconcileHandleRunning determine whether customCluster enter succeed phase or initFailed phase by checking the status of the worker
func (r *CustomClusterController) reconcileHandleRunning(ctx context.Context, customCluster *v1alpha1.CustomCluster) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)
	log.Info("~~~~~~~~current is reconcileHandleRunning")

	initWorker := &corev1.Pod{}
	initWorkerKey := getWorkerKey(customCluster, CustomClusterInitAction)
	if err := r.Client.Get(ctx, initWorkerKey, initWorker); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("Could not find worker", "worker", initWorkerKey)
			return ctrl.Result{}, nil
		}
		log.Error(err, "failed to get init worker. maybe it has been deleted.", "worker", initWorkerKey)
		return ctrl.Result{RequeueAfter: RequeueAfter}, err
	}

	if initWorker.Status.Phase == "Succeeded" {
		customCluster.Status.Phase = v1alpha1.SucceededPhase
		log.Info("customCluster's phase changes from Running to Succeeded")
		if err := r.Status().Update(ctx, customCluster); err != nil {
			return ctrl.Result{RequeueAfter: RequeueAfter}, err
		}
		return ctrl.Result{}, nil
	}

	if initWorker.Status.Phase == "Failed" {
		customCluster.Status.Phase = v1alpha1.InitFailedPhase
		log.Info("customCluster's phase changes from Running to InitFailedPhase")
		if err := r.Status().Update(ctx, customCluster); err != nil {
			return ctrl.Result{RequeueAfter: RequeueAfter}, err
		}
		return ctrl.Result{}, nil
	}
	return ctrl.Result{}, nil
}

// reconcileHandleTerminating determine whether customCluster enter terminateFailed phase or not
func (r *CustomClusterController) reconcileHandleTerminating(ctx context.Context, customCluster *v1alpha1.CustomCluster, customMachine *v1alpha1.CustomMachine) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)
	log.Info("~~~~~~~~current reconcileHandleTerminating")

	terminateWorker := &corev1.Pod{}
	terminateWorkerKey := getWorkerKey(customCluster, CustomClusterTerminateAction)

	if err := r.Client.Get(ctx, terminateWorkerKey, terminateWorker); err != nil {
		if apierrors.IsNotFound(err) {
			log.Error(err, "failed to find terminate worker", "worker", terminateWorkerKey)
			return ctrl.Result{}, nil
		}
		log.Error(err, "failed to get terminate worker. maybe it has been deleted.", "worker", terminateWorkerKey)
		return ctrl.Result{RequeueAfter: RequeueAfter}, err
	}

	log.Info("~~~~~~~~current terminate worker phase", "woker.phase", terminateWorker.Status.Phase)

	if terminateWorker.Status.Phase == "Succeeded" {
		// after k8s on VMs has been reset successful, we need delete the related CRD
		return r.reconcileDeleteResource(ctx, customCluster, customMachine)
	}

	if terminateWorker.Status.Phase == "Failed" {
		customCluster.Status.Phase = v1alpha1.TerminateFailedPhase
		log.Info("customCluster's phase changes from Terminating to TerminateFailed")
		if err := r.Status().Update(ctx, customCluster); err != nil {
			log.Error(err, "failed to Update", "worker", terminateWorkerKey)
			return ctrl.Result{RequeueAfter: RequeueAfter}, err
		}
		return ctrl.Result{}, nil
	}
	return ctrl.Result{}, nil
}

// reconcileDelete. If k8s cluster has been installed on VMs then we should uninstall it first, otherwise we just should delete related CRD
func (r *CustomClusterController) reconcileDelete(ctx context.Context, customCluster *v1alpha1.CustomCluster, customMachine *v1alpha1.CustomMachine) (ctrl.Result, error) {
	log.Info("~~~~~~~~current reconcileDelete")
	if vmsClusterIsAlreadyInstalled(customCluster) {
		return r.reconcileVMsTerminate(ctx, customCluster)
	}
	return r.reconcileDeleteResource(ctx, customCluster, customMachine)
}

// vmsClusterIsAlreadyInstalled determine whether a complete or partial k8s cluster has been installed on the VMs
func vmsClusterIsAlreadyInstalled(customCluster *v1alpha1.CustomCluster) bool {
	if len(customCluster.Status.Phase) != 0 {
		return true
	}
	return false
}

// reconcileVMsTerminate uninstall the k8s cluster on VMs
func (r *CustomClusterController) reconcileVMsTerminate(ctx context.Context, customCluster *v1alpha1.CustomCluster) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)
	log.Info("~~~~~~~~current reconcileVMsTerminate")

	// check if worker already exist. if not, create it
	terminateWorkerKey := getWorkerKey(customCluster, CustomClusterTerminateAction)
	workerPod := &corev1.Pod{}
	if err := r.Client.Get(ctx, terminateWorkerKey, workerPod); err != nil {
		if apierrors.IsNotFound(err) {
			terminateClusterPod := r.generateClusterManageWorker(customCluster, CustomClusterTerminateAction, KubesprayTerminateCMD)
			if err1 := r.Client.Create(ctx, terminateClusterPod); err1 != nil {
				log.Error(err1, "failed to create customCluster terminate worker")
				return ctrl.Result{RequeueAfter: RequeueAfter}, err1
			}
		} else {
			return ctrl.Result{RequeueAfter: RequeueAfter}, err
		}
	}
	log.Info("~~~~~~~~current reconcileHandleTerminating current worker phase~~~~~start worker reconcileVMsTerminate ", "worker.phase", workerPod.Status.Phase)

	customCluster.Status.Phase = v1alpha1.TerminatingPhase
	if err1 := r.Status().Update(ctx, customCluster); err1 != nil {
		return ctrl.Result{RequeueAfter: RequeueAfter}, err1
	}
	log.Info("customCluster's phase changes to Terminating")

	return ctrl.Result{}, nil
}

// reconcileDeleteResource delete resource related to customCluster: configmap, pod, customMachine customCluster etc.
func (r *CustomClusterController) reconcileDeleteResource(ctx context.Context, customCluster *v1alpha1.CustomCluster, customMachine *v1alpha1.CustomMachine) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)
	log.Info("~~~~~~~~current reconcileDeleteResource")

	log.Info("~~~~~~~~current delete cluster-hosts")
	// delete cluster-hosts. If not found, just ignore err and go to the next step
	clusterHostsKey := getClusterHostsKey(customCluster)
	clusterHosts := &corev1.ConfigMap{}
	if err := r.Client.Get(ctx, clusterHostsKey, clusterHosts); err != nil && !apierrors.IsNotFound(err) {
		log.Error(err, "failed to get clusterHosts when it should be deleted", "clusterHosts", clusterHostsKey)
		return ctrl.Result{RequeueAfter: RequeueAfter}, err
	}
	controllerutil.RemoveFinalizer(clusterHosts, CustomClusterConfigMapFinalizer)
	if err := r.Client.Update(ctx, clusterHosts); err != nil && !apierrors.IsNotFound(err) {
		log.Error(err, "failed to remove finalizer of clusterHosts when it should be deleted", "clusterHosts", clusterHostsKey)
		return ctrl.Result{RequeueAfter: RequeueAfter}, err
	}
	if err := r.Client.Delete(ctx, clusterHosts); err != nil && !apierrors.IsNotFound(err) {
		log.Error(err, "failed to delete clusterHosts", "clusterHosts", clusterHostsKey)
		return ctrl.Result{RequeueAfter: RequeueAfter}, err
	}

	log.Info("~~~~~~~~current delete cluster-config")
	// delete cluster-config. If not found, just ignore err and go to the next step
	clusterConfigKey := getClusterConfigKey(customCluster)
	clusterConfig := &corev1.ConfigMap{}
	if err := r.Client.Get(ctx, clusterConfigKey, clusterConfig); err != nil && !apierrors.IsNotFound(err) {
		log.Error(err, "failed to get clusterConfig when it should be deleted", "clusterConfig", clusterConfigKey)
		return ctrl.Result{RequeueAfter: RequeueAfter}, err
	}
	controllerutil.RemoveFinalizer(clusterConfig, CustomClusterConfigMapFinalizer)
	if err := r.Client.Update(ctx, clusterConfig); err != nil && !apierrors.IsNotFound(err) {
		log.Error(err, "failed to remove finalizer of clusterConfig  when it should be deleted", "clusterConfig", clusterConfigKey)
		return ctrl.Result{RequeueAfter: RequeueAfter}, err
	}
	if err := r.Client.Delete(ctx, clusterConfig); err != nil && !apierrors.IsNotFound(err) {
		log.Error(err, "failed to delete clusterConfig", "clusterConfig", clusterConfigKey)
		return ctrl.Result{RequeueAfter: RequeueAfter}, err
	}

	// delete init worker
	initWorker := &corev1.Pod{}
	initWorkerKey := getWorkerKey(customCluster, CustomClusterInitAction)
	if err := r.Client.Get(ctx, initWorkerKey, initWorker); err != nil && !apierrors.IsNotFound(err) {
		log.Error(err, "failed to get worker when it should be deleted", "worker", initWorkerKey)
		return ctrl.Result{RequeueAfter: RequeueAfter}, err
	}
	if err := r.Client.Delete(ctx, initWorker); err != nil && !apierrors.IsNotFound(err) {
		log.Error(err, "failed to delete worker ", "worker", initWorkerKey)
		return ctrl.Result{RequeueAfter: RequeueAfter}, err
	}

	// delete terminal worker
	terminateWorker := &corev1.Pod{}
	terminateWorkerKey := getWorkerKey(customCluster, CustomClusterTerminateAction)
	if err := r.Client.Get(ctx, terminateWorkerKey, terminateWorker); err != nil && !apierrors.IsNotFound(err) {
		log.Error(err, "failed to delete worker when it should be deleted", "worker", terminateWorkerKey)
		return ctrl.Result{RequeueAfter: RequeueAfter}, err
	}
	if err := r.Client.Delete(ctx, terminateWorker); err != nil && !apierrors.IsNotFound(err) {
		log.Error(err, "failed to delete worker", "worker", terminateWorkerKey)
		return ctrl.Result{RequeueAfter: RequeueAfter}, err
	}

	log.Info("~~~~~~~~current  delete customMachine")
	// delete customMachine
	controllerutil.RemoveFinalizer(customMachine, CustomClusterFinalizer)
	if err := r.Client.Update(ctx, customMachine); err != nil && !apierrors.IsNotFound(err) {
		log.Error(err, "failed to remove finalizer of customMachine when it should be deleted", "customMachine", customMachine.Name)
		return ctrl.Result{RequeueAfter: RequeueAfter}, err
	}
	if err := r.Client.Delete(ctx, customMachine); err != nil && !apierrors.IsNotFound(err) {
		log.Error(err, "failed to delete customMachine", "customMachine", customMachine.Name)
		return ctrl.Result{RequeueAfter: RequeueAfter}, err
	}

	log.Info("~~~~~~~~current  delete customCluster")
	// delete customCluster
	controllerutil.RemoveFinalizer(customCluster, CustomClusterFinalizer)
	if err := r.Client.Update(ctx, customCluster); err != nil && !apierrors.IsNotFound(err) {
		log.Error(err, "failed to remove finalizer of customCluster", "customCluster", customCluster.Name)
		return ctrl.Result{RequeueAfter: RequeueAfter}, err
	}
	if err := r.Client.Delete(ctx, customCluster); err != nil && !apierrors.IsNotFound(err) {
		log.Error(err, "failed to delete customCluster", "customCluster", customCluster.Name)
		return ctrl.Result{RequeueAfter: RequeueAfter}, err
	}

	return ctrl.Result{}, nil
}

type RelatedResource struct {
	clusterHosts  *corev1.ConfigMap
	clusterConfig *corev1.ConfigMap
	customCluster *v1alpha1.CustomCluster
	customMachine *v1alpha1.CustomMachine
	worker        *corev1.Pod
}

// reconcileCustomClusterInit create an init worker for installing cluster on VMs
func (r *CustomClusterController) reconcileCustomClusterInit(ctx context.Context, customCluster *v1alpha1.CustomCluster, customMachine *v1alpha1.CustomMachine, cluster *clusterv1.Cluster) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)
	log.Info("~~~~~~~~current is reconcileCustomClusterInit")

	// Fetch the KubeadmControlPlane instance.
	kcpKey := client.ObjectKey{
		Namespace: cluster.Spec.ControlPlaneRef.Namespace,
		Name:      cluster.Spec.ControlPlaneRef.Name,
	}
	kcp := &controlplanev1.KubeadmControlPlane{}
	if err := r.Client.Get(ctx, kcpKey, kcp); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("Could not find kcp", "kcp", kcpKey)
			return ctrl.Result{}, nil
		}
		return ctrl.Result{RequeueAfter: RequeueAfter}, err
	}

	clusterHost := &corev1.ConfigMap{}
	var errorHost error
	if clusterHost, errorHost = r.updateClusterHosts(ctx, customCluster, customMachine); errorHost != nil {
		log.Error(errorHost, "failed to update cluster-hosts configMap")
		return ctrl.Result{RequeueAfter: RequeueAfter}, errorHost
	}
	clusterConfig := &corev1.ConfigMap{}
	var errorConfig error
	if clusterConfig, errorConfig = r.updateClusterConfig(ctx, customCluster, customCluster, cluster, kcp); errorConfig != nil {
		log.Error(errorConfig, "failed to update cluster-config configMap")
		return ctrl.Result{RequeueAfter: RequeueAfter}, errorConfig
	}

	// check if init worker already exist. If not, create it
	initWorkerKey := getWorkerKey(customCluster, CustomClusterInitAction)
	initWorker := &corev1.Pod{}
	if err := r.Client.Get(ctx, initWorkerKey, initWorker); err != nil {
		if apierrors.IsNotFound(err) {
			initClusterPod := r.generateClusterManageWorker(customCluster, CustomClusterInitAction, KubesprayInitCMD)
			if err1 := r.Client.Create(ctx, initClusterPod); err1 != nil {
				log.Error(err1, "failed to create customCluster init worker", "initWorker", initWorkerKey)
				return ctrl.Result{RequeueAfter: RequeueAfter}, err1
			}
		} else {
			return ctrl.Result{RequeueAfter: RequeueAfter}, err
		}
	}
	log.Info("~~~~~~~~reconcileCustomClusterInit finish create worker")

	initRelatedResource := &RelatedResource{
		clusterHosts:  clusterHost,
		clusterConfig: clusterConfig,
		customCluster: customCluster,
		customMachine: customMachine,
		worker:        initWorker,
	}
	// when all related object is ready, we need ensure object's finalizer and ownerRef is set appropriately
	if err := r.ensureFinalizerAndOwnerRef(ctx, initRelatedResource); err != nil {
		log.Error(err, "failed to set finalizer or ownerRefs")
		return ctrl.Result{RequeueAfter: RequeueAfter}, err
	}
	log.Info("~~~~~~~~ensureFinalizerAndOwnerRef done")

	customCluster.Status.Phase = v1alpha1.RunningPhase
	if err1 := r.Status().Update(ctx, customCluster); err1 != nil {
		log.Error(err1, "failed to update customCluster", "customCluster", customCluster.Name)
		return ctrl.Result{RequeueAfter: RequeueAfter}, err1
	}
	log.Info("customCluster's phase changes to Running")

	return ctrl.Result{}, nil
}

// ensureFinalizerAndOwnerRef ensure every related resource's finalizer and ownerRef is ready
func (r *CustomClusterController) ensureFinalizerAndOwnerRef(ctx context.Context, res *RelatedResource) error {
	log := ctrl.LoggerFrom(ctx)

	controllerutil.AddFinalizer(res.customCluster, CustomClusterFinalizer)
	controllerutil.AddFinalizer(res.customMachine, CustomClusterFinalizer)
	controllerutil.AddFinalizer(res.clusterHosts, CustomClusterConfigMapFinalizer)
	controllerutil.AddFinalizer(res.clusterConfig, CustomClusterConfigMapFinalizer)

	ownerRefs := metav1.OwnerReference{
		APIVersion: res.customCluster.APIVersion,
		Kind:       "CustomCluster",
		Name:       res.customCluster.Name,
		UID:        res.customCluster.UID,
	}
	res.customMachine.OwnerReferences = []metav1.OwnerReference{ownerRefs}
	res.customCluster.OwnerReferences = []metav1.OwnerReference{ownerRefs}
	res.clusterHosts.OwnerReferences = []metav1.OwnerReference{ownerRefs}
	res.clusterConfig.OwnerReferences = []metav1.OwnerReference{ownerRefs}
	res.worker.OwnerReferences = []metav1.OwnerReference{ownerRefs}

	if err := r.Client.Update(ctx, res.customCluster); err != nil {
		log.Error(err, "failed to set finalizer or ownerRef of customCluster")
		return err
	}

	if err := r.Client.Update(ctx, res.customMachine); err != nil {
		log.Error(err, "failed to set finalizer or ownerRef of customMachine")
		return err
	}

	if err := r.Client.Update(ctx, res.clusterHosts); err != nil {
		log.Error(err, "failed to set finalizer or ownerRef of clusterHosts")
		return err
	}

	if err := r.Client.Update(ctx, res.clusterConfig); err != nil {
		log.Error(err, "failed to set finalizer or ownerRef of clusterConfig")
		return err
	}

	if err := r.Client.Update(ctx, res.worker); err != nil {
		log.Error(err, "failed to set finalizer or ownerRef of worker")
		return err
	}

	return nil
}

// generateClusterManageWorker generate a kubespray init cluster pod from configMap
func (r *CustomClusterController) generateClusterManageWorker(customCluster *v1alpha1.CustomCluster, manageAction customClusterManageAction, manageCMD customClusterManageCMD) *corev1.Pod {
	podName := customCluster.Name + "-" + string(manageAction)
	namespace := customCluster.Namespace
	defaultMode := int32(0o600)
	labels := map[string]string{WorkerLabelKey: customCluster.Name}

	managerWorker := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      podName,
			Labels:    labels,
		},
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},

		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:    podName,
					Image:   DefaultKubesprayImage,
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
								Name: customCluster.Name + "-" + ClusterHostsName,
							},
						},
					},
				},
				{
					Name: ClusterConfigName,
					VolumeSource: corev1.VolumeSource{
						ConfigMap: &corev1.ConfigMapVolumeSource{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: customCluster.Name + "-" + ClusterConfigName,
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
	// TODO: support other kubernetes configs
}

func GetConfigContent(c *clusterv1.Cluster, kcp *controlplanev1.KubeadmControlPlane) *ConfigTemplateContent {
	// Add kubespray init config here
	configContent := &ConfigTemplateContent{
		PodCIDR:     c.Spec.ClusterNetwork.Pods.CIDRBlocks[0],
		KubeVersion: kcp.Spec.Version,
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

func (r *CustomClusterController) CreatConfigMapWithTemplate(ctx context.Context, name, namespace, fileName, configMapData string) (*corev1.ConfigMap, error) {
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

func (r *CustomClusterController) generateClusterHosts(ctx context.Context, customMachine *v1alpha1.CustomMachine, customCluster *v1alpha1.CustomCluster) (*corev1.ConfigMap, error) {
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
	name := fmt.Sprintf("%s-%s", customCluster.Name, ClusterHostsName)
	namespace := customCluster.Namespace

	return r.CreatConfigMapWithTemplate(ctx, name, namespace, ClusterHostsName, hostData.String())
}

func (r *CustomClusterController) generateClusterConfig(ctx context.Context, c *clusterv1.Cluster, kcp *controlplanev1.KubeadmControlPlane, cc *v1alpha1.CustomCluster) (*corev1.ConfigMap, error) {
	configContent := GetConfigContent(c, kcp)
	configData := &strings.Builder{}

	// todo: split this to a separated file
	tmpl := template.Must(template.New("").Parse(`
kube_version: {{ .KubeVersion}}
download_run_once: true
download_container: false
download_localhost: true
# network
kube_pods_subnet: {{ .PodCIDR }}
`))

	if err := tmpl.Execute(configData, configContent); err != nil {
		return nil, err
	}
	name := fmt.Sprintf("%s-%s", cc.Name, ClusterConfigName)
	namespace := cc.Namespace

	return r.CreatConfigMapWithTemplate(ctx, name, namespace, ClusterConfigName, configData.String())
}

// updateClusterHosts. If cluster-hosts configmap is not exist, create it.
func (r *CustomClusterController) updateClusterHosts(ctx context.Context, customCluster *v1alpha1.CustomCluster, customMachine *v1alpha1.CustomMachine) (*corev1.ConfigMap, error) {
	cmKey := getClusterHostsKey(customCluster)
	cm := &corev1.ConfigMap{}
	if err := r.Client.Get(ctx, cmKey, cm); err != nil {
		if apierrors.IsNotFound(err) {
			return r.generateClusterHosts(ctx, customMachine, customCluster)
		}
		return nil, err
	}
	return cm, nil
}

// updateClusterConfig. If cluster-config configmap is not exist, create it.
func (r *CustomClusterController) updateClusterConfig(ctx context.Context, customCluster *v1alpha1.CustomCluster, cc *v1alpha1.CustomCluster, cluster *clusterv1.Cluster, kcp *controlplanev1.KubeadmControlPlane) (*corev1.ConfigMap, error) {
	cmKey := getClusterConfigKey(customCluster)
	cm := &corev1.ConfigMap{}
	if err := r.Client.Get(ctx, cmKey, cm); err != nil {
		if apierrors.IsNotFound(err) {
			return r.generateClusterConfig(ctx, cluster, kcp, cc)
		}
		return nil, err
	}
	return cm, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *CustomClusterController) SetupWithManager(ctx context.Context, mgr ctrl.Manager, options controller.Options) error {
	c, err := ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.CustomCluster{}).
		WithOptions(options).
		Build(r)
	if err != nil {
		return fmt.Errorf("failed setting up with a controller manager: %v", err)
	}

	if err := c.Watch(
		&source.Kind{Type: &corev1.Pod{}},
		handler.EnqueueRequestsFromMapFunc(r.WorkerToCustomCluster),
	); err != nil {
		return fmt.Errorf("failed adding Watch for worker to controller manager: %v", err)
	}

	if err := c.Watch(
		&source.Kind{Type: &v1alpha1.CustomMachine{}},
		handler.EnqueueRequestsFromMapFunc(r.CustomMachineToCustomCluster),
	); err != nil {
		return fmt.Errorf("failed adding Watch for CustomMachine to controller manager: %v", err)
	}

	if err := c.Watch(
		&source.Kind{Type: &clusterv1.Cluster{}},
		handler.EnqueueRequestsFromMapFunc(r.ClusterToCustomCluster),
	); err != nil {
		return fmt.Errorf("failed adding Watch for Clusters to controller manager: %v", err)
	}

	if err := c.Watch(
		&source.Kind{Type: &controlplanev1.KubeadmControlPlane{}},
		handler.EnqueueRequestsFromMapFunc(r.KcpToCustomCluster),
	); err != nil {
		return fmt.Errorf("failed adding Watch for KubeadmControlPlan to controller manager: %v", err)
	}

	return nil
}

func (r *CustomClusterController) WorkerToCustomCluster(o client.Object) []ctrl.Request {
	c, ok := o.(*corev1.Pod)
	if !ok {
		panic(fmt.Sprintf("Expected a Cluster but got a %T", o))
	}

	var log = ctrl.Log.WithName("cluster-operator")
	log.Info("catch a event: pod %s", "pod-name", c.Name)
	log.Info("catch a event: pod %s", "current-pod-phase", c.Status.Phase)

	if len(c.Labels[WorkerLabelKey]) != 0 {
		log.Info("catch a event: target is  %s", "infrastructureRef.Name", c.Labels[WorkerLabelKey])

		return []ctrl.Request{{NamespacedName: client.ObjectKey{Namespace: c.Namespace, Name: c.Labels[WorkerLabelKey]}}}
	}

	return nil
}

func (r *CustomClusterController) CustomMachineToCustomCluster(o client.Object) []ctrl.Request {
	c, ok := o.(*v1alpha1.CustomMachine)
	if !ok {
		panic(fmt.Sprintf("Expected a CustomMachine but got a %T", o))
	}
	var result []ctrl.Request

	var log = ctrl.Log.WithName("CustomCluster-operator")
	log.Info("catch a event: CustomMachine %s", "CustomMachine-name", c.Name)

	// Add all CustomMachine owners.
	for _, owner := range c.GetOwnerReferences() {
		if owner.Kind == "CustomCluster" {
			name := client.ObjectKey{Namespace: c.GetNamespace(), Name: owner.Name}
			result = append(result, ctrl.Request{NamespacedName: name})
			log.Info("catch a event: target is  %s", "infrastructureRef.Name", owner.Name)
			break
		}
	}

	return result
}

func (r *CustomClusterController) ClusterToCustomCluster(o client.Object) []ctrl.Request {
	c, ok := o.(*clusterv1.Cluster)
	if !ok {
		panic(fmt.Sprintf("Expected a Cluster but got a %T", o))
	}

	var log = ctrl.Log.WithName("cluster-operator")
	log.Info("catch a event: Cluster %s", "Cluster-name", c.Name)

	infrastructureRef := c.Spec.InfrastructureRef
	if infrastructureRef != nil && infrastructureRef.Kind == "CustomCluster" {
		log.Info("catch a event: target is  %s", "infrastructureRef.Name", infrastructureRef.Name)

		return []ctrl.Request{{NamespacedName: client.ObjectKey{Namespace: infrastructureRef.Namespace, Name: infrastructureRef.Name}}}
	}

	return nil
}

func (r *CustomClusterController) KcpToCustomCluster(o client.Object) []ctrl.Request {
	c, ok := o.(*controlplanev1.KubeadmControlPlane)
	if !ok {
		panic(fmt.Sprintf("Expected a KubeadmControlPlane but got a %T", o))
	}
	var result []ctrl.Request

	var log = ctrl.Log.WithName("CustomCluster-operator")
	log.Info("catch a event: KubeadmControlPlane %s", "KubeadmControlPlane-name", c.Name)

	// Find the cluster, that is the owner of kcp, from kcp
	clusterKey := client.ObjectKey{}
	for _, owner := range c.GetOwnerReferences() {
		if owner.Kind == "CustomCluster" {
			clusterKey = client.ObjectKey{Namespace: c.GetNamespace(), Name: owner.Name}
			break
		}
	}
	ownerCluster := &clusterv1.Cluster{}
	if err := r.Client.Get(context.TODO(), clusterKey, ownerCluster); err != nil {
		return nil
	}

	// Find the customCluster from cluster
	infrastructureRef := ownerCluster.Spec.InfrastructureRef
	if infrastructureRef != nil && infrastructureRef.Kind == "CustomCluster" {
		log.Info("catch a event: target is  %s", "infrastructureRef.Name", infrastructureRef.Name)
		return []ctrl.Request{{NamespacedName: client.ObjectKey{Namespace: infrastructureRef.Namespace, Name: infrastructureRef.Name}}}
	}

	return result
}

func getWorkerKey(customCluster *v1alpha1.CustomCluster, action customClusterManageAction) client.ObjectKey {
	return client.ObjectKey{
		Namespace: customCluster.Namespace,
		Name:      customCluster.Name + "-" + string(action),
	}
}

func getClusterHostsKey(customCluster *v1alpha1.CustomCluster) client.ObjectKey {
	return client.ObjectKey{
		Namespace: customCluster.Namespace,
		Name:      customCluster.Name + "-" + ClusterHostsName,
	}
}

func getClusterConfigKey(customCluster *v1alpha1.CustomCluster) client.ObjectKey {
	return client.ObjectKey{
		Namespace: customCluster.Namespace,
		Name:      customCluster.Name + "-" + ClusterConfigName,
	}
}
