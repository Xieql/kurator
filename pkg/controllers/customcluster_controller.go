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
	RequeueAfter        = time.Second * 5
	HostsConfigMapName  = "cluster-hosts"
	ParamsConfigMapName = "cluster-config"
	SecreteName         = "cluster-secret"

	CustomClusterInitAction customClusterManageAction = "init"
	KubesprayInitCMD        customClusterManageCMD    = "ansible-playbook -i inventory/" + HostsConfigMapName + " --private-key /root/.ssh/ssh-privatekey cluster.yml -vvv"

	CustomClusterTerminateAction customClusterManageAction = "terminate"
	KubesprayTerminateCMD        customClusterManageCMD    = "ansible-playbook -e reset_confirmation=yes -i inventory/" + HostsConfigMapName + " --private-key /root/.ssh/ssh-privatekey reset.yml -vvv"
	DefaultKubesprayImage                                  = "quay.io/kubespray/kubespray:v2.20.0"

	workerLabelKeyName = "customClusterName"
)

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *CustomClusterController) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)

	// Fetch the customCluster instance.
	customCluster := &v1alpha1.CustomCluster{}
	if err := r.Client.Get(ctx, req.NamespacedName, customCluster); err != nil {
		return ctrl.Result{RequeueAfter: RequeueAfter}, err
	}

	log = log.WithValues("customCluster", klog.KObj(customCluster))
	ctx = ctrl.LoggerInto(ctx, log)

	phase := customCluster.Status.Phase

	if len(phase) == 0 {
		customCluster.Status.Phase = v1alpha1.PendingPhase
		log.Info("customCluster's phase changes from nil to Pending")
		if err := r.Status().Update(ctx, customCluster); err != nil {
			log.Error(err, "phase can not update")
			return ctrl.Result{RequeueAfter: RequeueAfter}, err
		}
	}

	if phase == v1alpha1.FailedPhase {
		return ctrl.Result{Requeue: false}, nil
	}

	if phase == v1alpha1.RunningPhase {
		return r.reconcileRunningStatusUpdate(ctx, customCluster)
	}
	if phase == v1alpha1.TerminatingPhase {
		return r.reconcileTerminatingStatusUpdate(ctx, customCluster)
	}

	// Fetch the Cluster instance
	key := client.ObjectKey{
		Namespace: customCluster.Spec.ClusterRef.Namespace,
		Name:      customCluster.Spec.ClusterRef.Name,
	}
	cluster := &clusterv1.Cluster{}
	if err := r.Client.Get(ctx, key, cluster); err != nil {
		log.Error(err, "can not get cluster")
		return ctrl.Result{RequeueAfter: RequeueAfter}, err
	}

	// handle customCluster termination when the cluster is deleting
	if cluster.Status.Phase == "Deleting" && phase == v1alpha1.SucceededPhase {
		return r.reconcileCustomClusterTerminate(ctx, customCluster)
	}

	// If the installation is successful and the cluster has not been deleted,
	// the modification of the variable will not affect the previously installed cluster.
	if phase == v1alpha1.SucceededPhase {
		return ctrl.Result{RequeueAfter: RequeueAfter}, nil
	}

	// Handle the rest of the phase "pending"
	return r.reconcileCustomClusterInit(ctx, customCluster, cluster)
}

// reconcileRunningStatusUpdate decide whether it needs to be converted to succeed or failed
func (r *CustomClusterController) reconcileRunningStatusUpdate(ctx context.Context, customCluster *v1alpha1.CustomCluster) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)
	worker := &corev1.Pod{}

	key := client.ObjectKey{
		Namespace: customCluster.Namespace,
		Name:      customCluster.Name + "-" + string(CustomClusterInitAction),
	}
	if err := r.Client.Get(ctx, key, worker); err != nil {
		log.Error(err, "can not get init worker. maybe it has been deleted.")
		return ctrl.Result{RequeueAfter: RequeueAfter}, err
	}

	if worker.Status.Phase == "Succeeded" {
		customCluster.Status.Phase = v1alpha1.SucceededPhase
		customCluster.Status.StartTime = &metav1.Time{Time: time.Now()}
		log.Info("customCluster's phase changes from Running to Succeeded")
		if err := r.Status().Update(ctx, customCluster); err != nil {
			return ctrl.Result{RequeueAfter: RequeueAfter}, err
		}
		return ctrl.Result{RequeueAfter: RequeueAfter}, nil
	}

	if worker.Status.Phase == "Failed" {
		customCluster.Status.Phase = v1alpha1.FailedPhase
		customCluster.Status.StartTime = &metav1.Time{Time: time.Now()}
		log.Info("customCluster's phase changes from Running to Failed")
		if err := r.Status().Update(ctx, customCluster); err != nil {
			return ctrl.Result{RequeueAfter: RequeueAfter}, err
		}
		return ctrl.Result{RequeueAfter: RequeueAfter}, nil
	}
	return ctrl.Result{RequeueAfter: RequeueAfter}, nil
}

// reconcileTerminatingStatusUpdate decide whether it needs to be converted to pending or failed
func (r *CustomClusterController) reconcileTerminatingStatusUpdate(ctx context.Context, customCluster *v1alpha1.CustomCluster) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)
	worker := &corev1.Pod{}

	key := client.ObjectKey{
		Namespace: customCluster.Namespace,
		Name:      customCluster.Name + "-" + string(CustomClusterTerminateAction),
	}
	if err := r.Client.Get(ctx, key, worker); err != nil {
		log.Error(err, "can not get init worker. maybe it has been deleted.")
		return ctrl.Result{RequeueAfter: RequeueAfter}, err
	}

	if worker.Status.Phase == "Succeeded" {
		customCluster.Status.Phase = v1alpha1.PendingPhase
		customCluster.Status.StartTime = &metav1.Time{Time: time.Now()}
		log.Info("customCluster's phase changes from Terminating to Succeeded")

		if err := r.Status().Update(ctx, customCluster); err != nil {
			return ctrl.Result{RequeueAfter: RequeueAfter}, err
		}
		return ctrl.Result{RequeueAfter: RequeueAfter}, nil
	}

	if worker.Status.Phase == "Failed" {
		customCluster.Status.Phase = v1alpha1.FailedPhase
		customCluster.Status.StartTime = &metav1.Time{Time: time.Now()}
		log.Info("customCluster's phase changes from Terminating to Failed")

		if err := r.Status().Update(ctx, customCluster); err != nil {
			return ctrl.Result{RequeueAfter: RequeueAfter}, err
		}
		return ctrl.Result{RequeueAfter: RequeueAfter}, nil
	}
	return ctrl.Result{RequeueAfter: RequeueAfter}, nil
}

func (r *CustomClusterController) reconcileCustomClusterTerminate(ctx context.Context, customCluster *v1alpha1.CustomCluster) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)

	terminateClusterPod := r.CreateClusterManageWorker(customCluster, CustomClusterTerminateAction, KubesprayTerminateCMD)
	if err := r.Client.Create(ctx, terminateClusterPod); err != nil {
		log.Error(err, "failed to create customCluster terminate worker")
		return ctrl.Result{RequeueAfter: RequeueAfter}, err
	}
	customCluster.Status.Phase = v1alpha1.TerminatingPhase
	customCluster.Status.StartTime = &metav1.Time{Time: time.Now()}

	podRef := &corev1.ObjectReference{
		Namespace: terminateClusterPod.Namespace,
		Name:      terminateClusterPod.Name,
	}
	customCluster.Status.WorkerRef = podRef
	log.Info("customCluster's phase changes from Succeed to Failed")
	if err := r.Status().Update(ctx, customCluster); err != nil {
		return ctrl.Result{RequeueAfter: RequeueAfter}, err
	}

	return ctrl.Result{RequeueAfter: RequeueAfter}, nil
}

func (r *CustomClusterController) reconcileCustomClusterInit(ctx context.Context, customCluster *v1alpha1.CustomCluster, cluster *clusterv1.Cluster) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)

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
	if err := r.UpdateHostsConfigMap(ctx, customCluster, customMachine); err != nil {
		log.Error(err, "failed to create hosts configMap")
		return ctrl.Result{RequeueAfter: RequeueAfter}, err
	}

	if err := r.UpdateClusterConfig(ctx, customCluster, customCluster, cluster, kcp); err != nil {
		log.Error(err, "failed to create cluster-config configMap")
		return ctrl.Result{RequeueAfter: RequeueAfter}, err
	}

	initClusterPod := r.CreateClusterManageWorker(customCluster, CustomClusterInitAction, KubesprayInitCMD)

	if err := r.Client.Create(ctx, initClusterPod); err != nil {
		log.Error(err, "failed to init Worker")
		return ctrl.Result{RequeueAfter: RequeueAfter}, err
	}
	customCluster.Status.Phase = v1alpha1.RunningPhase
	customCluster.Status.StartTime = &metav1.Time{Time: time.Now()}
	podRef := &corev1.ObjectReference{
		Namespace: initClusterPod.Namespace,
		Name:      initClusterPod.Name,
	}
	customCluster.Status.WorkerRef = podRef
	log.Info("customCluster's phase changes from Pending to Running")

	if err := r.Status().Update(ctx, customCluster); err != nil {
		return ctrl.Result{RequeueAfter: RequeueAfter}, err
	}

	return ctrl.Result{RequeueAfter: RequeueAfter}, nil
}

// CreateClusterManageWorker create a kubespray init cluster pod from configMap
func (r *CustomClusterController) CreateClusterManageWorker(customCluster *v1alpha1.CustomCluster, manageAction customClusterManageAction, manageCMD customClusterManageCMD) *corev1.Pod {
	podName := customCluster.Name + "-" + string(manageAction)
	namespace := customCluster.Namespace
	defaultMode := int32(0o600)
	labels := map[string]string{workerLabelKeyName: customCluster.Name}

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
							Name:      HostsConfigMapName,
							MountPath: "/kubespray/inventory",
						},
						{
							Name:      ParamsConfigMapName,
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
					Name: HostsConfigMapName,
					VolumeSource: corev1.VolumeSource{
						ConfigMap: &corev1.ConfigMapVolumeSource{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: customCluster.Name + "-" + HostsConfigMapName,
							},
						},
					},
				},
				{
					Name: ParamsConfigMapName,
					VolumeSource: corev1.VolumeSource{
						ConfigMap: &corev1.ConfigMapVolumeSource{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: customCluster.Name + "-" + ParamsConfigMapName,
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

func GetVarContent(c *clusterv1.Cluster, kcp *controlplanev1.KubeadmControlPlane) *ConfigTemplateContent {
	// Add kubespray init config here
	varContent := &ConfigTemplateContent{
		PodCIDR:     c.Spec.ClusterNetwork.Pods.CIDRBlocks[0],
		KubeVersion: kcp.Spec.Version,
	}
	return varContent
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
	ConfigMap := &corev1.ConfigMap{
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

	if err := r.Client.Create(ctx, ConfigMap); err != nil {
		return err
	}
	return nil
}

func (r *CustomClusterController) CreateHostsConfigMap(ctx context.Context, customMachine *v1alpha1.CustomMachine, customCluster *v1alpha1.CustomCluster) error {
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
	name := fmt.Sprintf("%s-%s", customCluster.Name, HostsConfigMapName)
	namespace := customCluster.Namespace

	return r.CreatConfigMapWithTemplate(ctx, name, namespace, HostsConfigMapName, hostData.String())
}

func (r *CustomClusterController) CreateClusterConfig(ctx context.Context, c *clusterv1.Cluster, kcp *controlplanev1.KubeadmControlPlane, cc *v1alpha1.CustomCluster) error {
	configContent := GetVarContent(c, kcp)
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
	name := fmt.Sprintf("%s-%s", cc.Name, ParamsConfigMapName)
	namespace := cc.Namespace

	return r.CreatConfigMapWithTemplate(ctx, name, namespace, ParamsConfigMapName, configData.String())
}

func (r *CustomClusterController) UpdateHostsConfigMap(ctx context.Context, customCluster *v1alpha1.CustomCluster, customMachine *v1alpha1.CustomMachine) error {
	log := ctrl.LoggerFrom(ctx)
	ConfigMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      customCluster.Name + "-" + HostsConfigMapName,
			Namespace: customCluster.Namespace,
		},
	}

	if err := r.Client.Delete(ctx, ConfigMap); err != nil {
		if !apierrors.IsNotFound(err) {
			log.Error(err, "failed to delete cluster-hosts configMap")
			return err
		}
	}
	return r.CreateHostsConfigMap(ctx, customMachine, customCluster)
}

func (r *CustomClusterController) UpdateClusterConfig(ctx context.Context, customCluster *v1alpha1.CustomCluster, cc *v1alpha1.CustomCluster, cluster *clusterv1.Cluster, kcp *controlplanev1.KubeadmControlPlane) error {
	log := ctrl.LoggerFrom(ctx)
	ConfigMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      customCluster.Name + "-" + ParamsConfigMapName,
			Namespace: customCluster.Namespace,
		},
	}

	if err := r.Client.Delete(ctx, ConfigMap); err != nil {
		if !apierrors.IsNotFound(err) {
			log.Error(err, "failed to delete cluster-config configMap")
			return err
		}
	}
	return r.CreateClusterConfig(ctx, cluster, kcp, cc)
}

// SetupWithManager sets up the controller with the Manager.
func (r *CustomClusterController) SetupWithManager(mgr ctrl.Manager, options controller.Options) error {
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

func (r *CustomClusterController) ClusterToCustomCluster(o client.Object) []ctrl.Request {
	c, ok := o.(*clusterv1.Cluster)
	if !ok {
		panic(fmt.Sprintf("Expected a Cluster but got a %T", o))
	}

	infrastructureRef := c.Spec.InfrastructureRef
	if infrastructureRef != nil && infrastructureRef.Kind == "CustomCluster" {
		return []ctrl.Request{{NamespacedName: client.ObjectKey{Namespace: infrastructureRef.Namespace, Name: infrastructureRef.Name}}}
	}

	return nil
}

func (r *CustomClusterController) WorkerToCustomCluster(o client.Object) []ctrl.Request {
	c, ok := o.(*corev1.Pod)
	if !ok {
		panic(fmt.Sprintf("Expected a Cluster but got a %T", o))
	}

	if len(c.Labels[workerLabelKeyName]) != 0 {
		return []ctrl.Request{{NamespacedName: client.ObjectKey{Namespace: c.Namespace, Name: c.Labels[workerLabelKeyName]}}}
	}

	return nil
}
