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
	addonsv1 "sigs.k8s.io/cluster-api/exp/addons/api/v1beta1"
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

	// CustomClusterFinalizer is the finalizer applied to CustomClusterFinalizer resources
	// by its managing controller.
	CustomClusterFinalizer = "customcluster.cluster.kurator.dev"
)

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *CustomClusterController) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)

	// Fetch the customCluster instance.
	customCluster := &v1alpha1.CustomCluster{}
	if err := r.Client.Get(ctx, req.NamespacedName, customCluster); err != nil {
		log.Error(err, "can not get customCluster", "customCluster name", req.Name)
		return ctrl.Result{RequeueAfter: RequeueAfter}, err
	}
	log = log.WithValues("customCluster", klog.KObj(customCluster))
	ctx = ctrl.LoggerInto(ctx, log)

	// Fetch the Cluster instance
	key := client.ObjectKey{
		Namespace: customCluster.Spec.ClusterRef.Namespace,
		Name:      customCluster.Spec.ClusterRef.Name,
	}
	cluster := &clusterv1.Cluster{}
	if err := r.Client.Get(ctx, key, cluster); err != nil {
		log.Error(err, "can not get cluster", "cluster name", key.Name)
		return ctrl.Result{RequeueAfter: RequeueAfter}, err
	}

	// Fetch the CustomMachine instance.
	customMachinekey := client.ObjectKey{
		Namespace: customCluster.Spec.MachineRef.Namespace,
		Name:      customCluster.Spec.MachineRef.Name,
	}
	customMachine := &v1alpha1.CustomMachine{}
	if err := r.Client.Get(ctx, customMachinekey, customMachine); err != nil {
		log.Error(err, "can not get customMachine")
		return ctrl.Result{RequeueAfter: RequeueAfter}, err
	}

	return r.reconcile(ctx, customCluster, customMachine, cluster)
}

// reconcile handles CustomCluster reconciliation.
func (r *CustomClusterController) reconcile(ctx context.Context, customCluster *v1alpha1.CustomCluster, customMachine *v1alpha1.CustomMachine, cluster *clusterv1.Cluster) (ctrl.Result, error) {
	phase := customCluster.Status.Phase

	// if upstream cluster at pre-delete status and customCluster not in terminate phase, the customCluster will try to terminate/uninstall/reset installed k8s cluster on VMs. In addition, the related crd needs to be deleted
	if cluster.DeletionTimestamp.IsZero() && phase != v1alpha1.TerminatingPhase {
		return r.reconcileDelete(ctx, customCluster, customMachine)
	}

	// customCluster in phase nil or initFailed will try to enter running phase by creating an init worker successful
	if len(phase) == 0 || phase == v1alpha1.InitFailedPhase {
		return r.reconcileCustomClusterInit(ctx, customCluster, customMachine, cluster)
	}

	// customCluster in phase running will wait worker event to enter succeed phase or initFailed phase
	if phase == v1alpha1.RunningPhase {
		return r.reconcileHandleRunning(ctx, customCluster)
	}

	// customCluster in phase Terminating will wait worker event to finish terminal or enter terminalFailed phase
	if phase == v1alpha1.TerminatingPhase {
		return r.reconcileHandleTerminating(ctx, customCluster, customMachine)
	}

	return ctrl.Result{Requeue: false}, nil
}

// reconcileHandleRunning determine whether customCluster enter succeed phase or initFailed phase by checking the status of the worker
func (r *CustomClusterController) reconcileHandleRunning(ctx context.Context, customCluster *v1alpha1.CustomCluster) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)
	worker := &corev1.Pod{}

	key := client.ObjectKey{
		Namespace: customCluster.Namespace,
		Name:      customCluster.Name + "-" + string(CustomClusterInitAction),
	}
	if err := r.Client.Get(ctx, key, worker); err != nil {
		log.Error(err, "can not get init worker. maybe it has been deleted.", "worker", key.Name)
		return ctrl.Result{RequeueAfter: RequeueAfter}, err
	}

	if worker.Status.Phase == "Succeeded" {
		customCluster.Status.Phase = v1alpha1.SucceededPhase
		log.Info("customCluster's phase changes from Running to Succeeded")
		if err := r.Status().Update(ctx, customCluster); err != nil {
			return ctrl.Result{RequeueAfter: RequeueAfter}, err
		}
		return ctrl.Result{RequeueAfter: RequeueAfter}, nil
	}

	if worker.Status.Phase == "Failed" {
		customCluster.Status.Phase = v1alpha1.FailedPhase
		log.Info("customCluster's phase changes from Running to Failed")
		if err := r.Status().Update(ctx, customCluster); err != nil {
			return ctrl.Result{RequeueAfter: RequeueAfter}, err
		}
		return ctrl.Result{RequeueAfter: RequeueAfter}, nil
	}
	return ctrl.Result{RequeueAfter: RequeueAfter}, nil
}

// reconcileHandleTerminating determine whether customCluster enter terminateFailed phase or not
func (r *CustomClusterController) reconcileHandleTerminating(ctx context.Context, customCluster *v1alpha1.CustomCluster, customMachine *v1alpha1.CustomMachine) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)
	worker := &corev1.Pod{}

	key := client.ObjectKey{
		Namespace: customCluster.Namespace,
		Name:      customCluster.Name + "-" + string(CustomClusterTerminateAction),
	}
	if err := r.Client.Get(ctx, key, worker); err != nil {
		log.Error(err, "can not get init worker. maybe it has been deleted.", "worker", key.Name)
		return ctrl.Result{RequeueAfter: RequeueAfter}, err
	}

	if worker.Status.Phase == "Succeeded" {
		// the k8s on VMs has been reset successful, what should we do is just to delete the related CRD
		return r.reconcileDeleteCRD(ctx, customCluster, customMachine)
	}

	if worker.Status.Phase == "Failed" {
		customCluster.Status.Phase = v1alpha1.TerminateFailedPhase
		log.Info("customCluster's phase changes from Terminating to TerminateFailed")

		if err := r.Status().Update(ctx, customCluster); err != nil {
			return ctrl.Result{RequeueAfter: RequeueAfter}, err
		}
	}
	return ctrl.Result{Requeue: false}, nil
}

// reconcileCustomClusterTerminate, if k8s cluster has been installed on VMs, we should uninstall it first. Or we just should delete related CRD
func (r *CustomClusterController) reconcileDelete(ctx context.Context, customCluster *v1alpha1.CustomCluster, customMachine *v1alpha1.CustomMachine) (ctrl.Result, error) {

	if customCluster.Status.Phase == v1alpha1.RunningPhase || customCluster.Status.Phase == v1alpha1.SucceededPhase {
		return r.reconcileCustomClusterTerminate(ctx, customCluster, customMachine)
	}
	return r.reconcileDeleteCRD(ctx, customCluster, customMachine)
}

// reconcileCustomClusterTerminate uninstall the k8s cluster on VMs
func (r *CustomClusterController) reconcileCustomClusterTerminate(ctx context.Context, customCluster *v1alpha1.CustomCluster, customMachine *v1alpha1.CustomMachine) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)

	terminateClusterPod := r.generateClusterManageWorker(customCluster, CustomClusterTerminateAction, KubesprayTerminateCMD)
	if err := r.Client.Create(ctx, terminateClusterPod); err != nil {
		log.Error(err, "failed to create customCluster terminate worker")
		return ctrl.Result{RequeueAfter: RequeueAfter}, err
	}
	customCluster.Status.Phase = v1alpha1.TerminatingPhase

	log.Info("customCluster's phase changes from Succeed to Failed")
	if err := r.Status().Update(ctx, customCluster); err != nil {
		return ctrl.Result{RequeueAfter: RequeueAfter}, err
	}

	return ctrl.Result{RequeueAfter: RequeueAfter}, nil
}

// reconcileDeleteCRD delete resource related to customCluster: customMachine, worker etc.
func (r *CustomClusterController) reconcileDeleteCRD(ctx context.Context, customCluster *v1alpha1.CustomCluster, customMachine *v1alpha1.CustomMachine) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)

	controllerutil.RemoveFinalizer(customCluster, addonsv1.ClusterResourceSetFinalizer)
	controllerutil.RemoveFinalizer(customMachine, addonsv1.ClusterResourceSetFinalizer)
	if err := r.Client.Update(ctx, customCluster); err != nil {
		log.Error(err, "can not remove finalizer of customCluster", "customCluster", customCluster.Name)
		return ctrl.Result{RequeueAfter: RequeueAfter}, err
	}
	if err := r.Client.Update(ctx, customMachine); err != nil {
		log.Error(err, "can not remove finalizer of customMachine", "customMachine", customMachine.Name)
		return ctrl.Result{RequeueAfter: RequeueAfter}, err
	}

	return ctrl.Result{RequeueAfter: RequeueAfter}, nil
}

func (r *CustomClusterController) reconcileCustomClusterInit(ctx context.Context, customCluster *v1alpha1.CustomCluster, customMachine *v1alpha1.CustomMachine, cluster *clusterv1.Cluster) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)

	// Set the ownerRefs of customCluster and customMachine
	if err := r.setFinalizerAndOwnerRef(ctx, cluster, customCluster, customMachine); err != nil {
		log.Error(err, "can not set ownerRefs")
		return ctrl.Result{RequeueAfter: RequeueAfter}, err
	}

	log.Info("~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~set owner ref succeed")

	// Fetch the KubeadmControlPlane instance.
	kcpKey := client.ObjectKey{
		Namespace: cluster.Spec.ControlPlaneRef.Namespace,
		Name:      cluster.Spec.ControlPlaneRef.Name,
	}
	kcp := &controlplanev1.KubeadmControlPlane{}
	if err := r.Client.Get(ctx, kcpKey, kcp); err != nil {
		log.Error(err, "can not get kcp")
		return ctrl.Result{RequeueAfter: RequeueAfter}, err
	}

	// Delete current cm and create new cm using current object
	if err := r.UpdateClusterHosts(ctx, customCluster, customMachine); err != nil {
		log.Error(err, "failed to create hosts configMap")
		return ctrl.Result{RequeueAfter: RequeueAfter}, err
	}

	if err := r.UpdateClusterConfig(ctx, customCluster, customCluster, cluster, kcp); err != nil {
		log.Error(err, "failed to create cluster-config configMap")
		return ctrl.Result{RequeueAfter: RequeueAfter}, err
	}

	initClusterPod := r.generateClusterManageWorker(customCluster, CustomClusterInitAction, KubesprayInitCMD)

	if err := r.Client.Create(ctx, initClusterPod); err != nil {
		log.Error(err, "failed to init Worker")
		return ctrl.Result{RequeueAfter: RequeueAfter}, err
	}
	customCluster.Status.Phase = v1alpha1.RunningPhase

	log.Info("customCluster's phase changes from Pending to Running")

	if err := r.Status().Update(ctx, customCluster); err != nil {
		return ctrl.Result{RequeueAfter: RequeueAfter}, err
	}

	return ctrl.Result{RequeueAfter: RequeueAfter}, nil
}

func (r *CustomClusterController) setFinalizerAndOwnerRef(ctx context.Context, cluster *clusterv1.Cluster, customCluster *v1alpha1.CustomCluster, customMachine *v1alpha1.CustomMachine) error {
	log := ctrl.LoggerFrom(ctx)

	r.setFinalizer(customCluster, customMachine)
	r.setOwnerRef(cluster, customCluster, customMachine)
	if err := r.Client.Update(ctx, customCluster); err != nil {
		log.Error(err, "can not set finalizer or ownerRef of customCluster")
		return err
	}

	if err := r.Client.Update(ctx, customMachine); err != nil {
		log.Error(err, "can not set finalizer or ownerRef of customMachine")
		return err
	}
	return nil
}

// setFinalizer set customCluster's and customMachine's finalizer with CustomClusterFinalizer to avoid the race condition between init and delete
func (r *CustomClusterController) setFinalizer(customCluster *v1alpha1.CustomCluster, customMachine *v1alpha1.CustomMachine) {
	if !controllerutil.ContainsFinalizer(customCluster, CustomClusterFinalizer) {
		controllerutil.AddFinalizer(customCluster, CustomClusterFinalizer)
	}

	if !controllerutil.ContainsFinalizer(customMachine, CustomClusterFinalizer) {
		controllerutil.AddFinalizer(customMachine, CustomClusterFinalizer)
	}
}

// setOwnerRef set customCluster's ownerRefs with cluster, set customMachine ownerRefs with customCluster
func (r *CustomClusterController) setOwnerRef(cluster *clusterv1.Cluster, customCluster *v1alpha1.CustomCluster, customMachine *v1alpha1.CustomMachine) {
	customClusterRefs := metav1.OwnerReference{
		APIVersion: cluster.APIVersion,
		Kind:       "Cluster",
		Name:       cluster.Name,
		UID:        cluster.UID,
	}
	customCluster.OwnerReferences = []metav1.OwnerReference{customClusterRefs}
	customMachineRefs := metav1.OwnerReference{
		APIVersion: cluster.APIVersion,
		Kind:       "CustomCluster",
		Name:       customCluster.Name,
		UID:        customCluster.UID,
	}
	customMachine.OwnerReferences = []metav1.OwnerReference{customMachineRefs}
}

// generateClusterManageWorker create a kubespray init cluster pod from configMap
func (r *CustomClusterController) generateClusterManageWorker(customCluster *v1alpha1.CustomCluster, manageAction customClusterManageAction, manageCMD customClusterManageCMD) *corev1.Pod {
	podName := customCluster.Name + "-" + string(manageAction)
	namespace := customCluster.Namespace
	defaultMode := int32(0o600)
	labels := map[string]string{WorkerLabelKey: customCluster.Name}

	initPod := &corev1.Pod{
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
	return initPod
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

func (r *CustomClusterController) CreatConfigMapWithTemplate(ctx context.Context, name, namespace, fileName, configMapData string) error {
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
		return err
	}
	return nil
}

func (r *CustomClusterController) CreateClusterHosts(ctx context.Context, customMachine *v1alpha1.CustomMachine, customCluster *v1alpha1.CustomCluster) error {
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
		return err
	}
	name := fmt.Sprintf("%s-%s", customCluster.Name, ClusterHostsName)
	namespace := customCluster.Namespace

	return r.CreatConfigMapWithTemplate(ctx, name, namespace, ClusterHostsName, hostData.String())
}

func (r *CustomClusterController) CreateClusterConfig(ctx context.Context, c *clusterv1.Cluster, kcp *controlplanev1.KubeadmControlPlane, cc *v1alpha1.CustomCluster) error {
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
		return err
	}
	name := fmt.Sprintf("%s-%s", cc.Name, ClusterConfigName)
	namespace := cc.Namespace

	return r.CreatConfigMapWithTemplate(ctx, name, namespace, ClusterConfigName, configData.String())
}

func (r *CustomClusterController) UpdateClusterHosts(ctx context.Context, customCluster *v1alpha1.CustomCluster, customMachine *v1alpha1.CustomMachine) error {
	log := ctrl.LoggerFrom(ctx)
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      customCluster.Name + "-" + ClusterHostsName,
			Namespace: customCluster.Namespace,
		},
	}

	if err := r.Client.Delete(ctx, cm); err != nil {
		if !apierrors.IsNotFound(err) {
			log.Error(err, "failed to delete cluster-hosts configMap")
			return err
		}
	}
	return r.CreateClusterHosts(ctx, customMachine, customCluster)
}

func (r *CustomClusterController) UpdateClusterConfig(ctx context.Context, customCluster *v1alpha1.CustomCluster, cc *v1alpha1.CustomCluster, cluster *clusterv1.Cluster, kcp *controlplanev1.KubeadmControlPlane) error {
	log := ctrl.LoggerFrom(ctx)
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      customCluster.Name + "-" + ClusterConfigName,
			Namespace: customCluster.Namespace,
		},
	}

	if err := r.Client.Delete(ctx, cm); err != nil {
		if !apierrors.IsNotFound(err) {
			log.Error(err, "failed to delete cluster-config configMap")
			return err
		}
	}
	return r.CreateClusterConfig(ctx, cluster, kcp, cc)
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
		&source.Kind{Type: &clusterv1.Cluster{}},
		handler.EnqueueRequestsFromMapFunc(r.ClusterToCustomCluster),
	); err != nil {
		return fmt.Errorf("failed adding Watch for Clusters to controller manager: %v", err)
	}

	if err := c.Watch(
		&source.Kind{Type: &corev1.Pod{}},
		handler.EnqueueRequestsFromMapFunc(r.WorkerToCustomCluster),
	); err != nil {
		return fmt.Errorf("failed adding Watch for worker to controller manager: %v", err)
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
