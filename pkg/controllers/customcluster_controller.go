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
	_ "embed"
	"fmt"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"kurator.dev/kurator/pkg/apis/cluster/v1alpha1"
	"strings"
	"text/template"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/klog/v2"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
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
	ClientSet kubernetes.Interface

	APIReader client.Reader
	Scheme    *runtime.Scheme

	externalTracker external.ObjectTracker
}

const (
	RequeueAfter = time.Second * 5
	// HostYamlFileName hostConfigmap's default name is customCluster.Name + "-" + HostYamlFileName
	HostYamlFileName = "hosts.yaml"
	// VarsYamlFileName varsConfigmap's default name is customCluster.Name + "-" + VarsYamlFileName
	VarsYamlFileName = "vars.yaml"
	secreteName      = "ssh-key-secret"

	DefaultFileRepo       = "https://files.m.daocloud.io"
	DefaultKubesprayImage = "quay.io/kubespray/kubespray:v2.20.0"

	KubesprayInitCMD  = "ansible-playbook -i inventory/hosts.yaml --private-key /root/.ssh/ssh-privatekey cluster.yml -vvv"
	KubesprayResetCMD = "ansible-playbook -e reset_confirmation=yes -i inventory/hosts.yaml reset.yml -vvv"
)

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

	// Fetch the customCluster instance.
	customCluster := &v1alpha1.CustomCluster{}
	if err := r.Client.Get(ctx, req.NamespacedName, customCluster); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{Requeue: false}, nil
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
		log.Error(err, "can not get customMachine")
		if apierrors.IsNotFound(err) {
			return ctrl.Result{Requeue: false}, nil
		}
		return ctrl.Result{RequeueAfter: RequeueAfter}, err
	}

	// Fetch the Cluster instance
	clusterkey := client.ObjectKey{
		Namespace: customCluster.Spec.ClusterRef.Namespace,
		Name:      customCluster.Spec.ClusterRef.Name,
	}
	cluster := &clusterv1.Cluster{}
	if err := r.Client.Get(ctx, clusterkey, cluster); err != nil {
		log.Error(err, "can not get cluster")
		if apierrors.IsNotFound(err) {
			return ctrl.Result{Requeue: false}, nil
		}
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
		if apierrors.IsNotFound(err) {
			return ctrl.Result{Requeue: false}, nil
		}
		return ctrl.Result{RequeueAfter: RequeueAfter}, err
	}

	// TODO: check cluster status

	log = log.WithValues("customCluster", klog.KObj(customCluster))
	ctx = ctrl.LoggerInto(ctx, log)

	return r.reconcile(ctx, customCluster, customMachine, cluster, kcp)
}

func (r *CustomClusterController) reconcile(ctx context.Context, customCluster *v1alpha1.CustomCluster, customMachine *v1alpha1.CustomMachine, cluster *clusterv1.Cluster, kcp *controlplanev1.KubeadmControlPlane) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)

	// 根据 Cluster KCP CustomMachine 及 SSH key secret生成kubespray参数 Configmap

	// 删除cm重新根据当前对象创建新的cm
	if err := r.UpdateHostsConfigMap(ctx, customCluster, customMachine); err != nil {
		log.Error(err, "failed to create hosts configMap")
		return ctrl.Result{RequeueAfter: RequeueAfter}, err
	}

	if err := r.UpdateVarsConfigMap(ctx, customCluster, customCluster, cluster, kcp); err != nil {
		log.Error(err, "failed to create hosts configMap")
		return ctrl.Result{RequeueAfter: RequeueAfter}, err
	}

	initClusterJob := r.CreateKubesprayInitClusterJob(customCluster)

	if curJob, err := r.ClientSet.BatchV1().Jobs(initClusterJob.Namespace).Create(context.Background(), initClusterJob, metav1.CreateOptions{}); err != nil {
		log.Error(err, "failed to create job", "jobName", curJob.Name)
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// CreateKubesprayInitClusterJob create a kubespray init cluster job from configMap, or check exist first ?
func (r *CustomClusterController) CreateKubesprayInitClusterJob(customCluster *v1alpha1.CustomCluster) *batchv1.Job {
	jobName := customCluster.Name + "-kubespray-init-cluster"
	namespace := customCluster.Namespace
	containerName := customCluster.Name + "-container"
	DefaultMode := int32(0o600)

	initJob := &batchv1.Job{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "batch/v1",
			Kind:       "Job",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      jobName,
		},

		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:    containerName,
							Image:   DefaultKubesprayImage,
							Command: []string{"/bin/sh", "-c"},
							Args:    []string{KubesprayInitCMD},

							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "hosts-conf",
									MountPath: "/kubespray/inventory",
								},
								{
									Name:      "vars-conf",
									MountPath: "/kubespray/inventory/group_vars/all",
								},
								{
									Name:      "secret-volume",
									MountPath: "/root/.ssh",
									ReadOnly:  true,
								},
							},
						},
					},

					Volumes: []corev1.Volume{
						{
							Name: "hosts-conf",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: customCluster.Name + "-" + HostYamlFileName,
										//Name: "hosts-conf",
									},
								},
							},
						},
						{
							Name: "vars-conf",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: customCluster.Name + "-" + VarsYamlFileName,
										//Name: "vars-conf",
									},
								},
							},
						},
						{
							Name: "secret-volume",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName:  secreteName,
									DefaultMode: &DefaultMode,
								},
							},
						},
					},
					RestartPolicy: corev1.RestartPolicyNever,
				},
			},
		},
	}
	return initJob
}

type HostTemplateContent struct {
	NodeAndIP    []string
	MasterName   []string
	NodeName     []string
	EtcdNodeName []string // default: NodeName + MasterName
}

type VarsTemplateContent struct {
	KubeVersion string
	PodCIDR     string
	FileRepo    string
}

func GetVarContent(c *clusterv1.Cluster, kcp *controlplanev1.KubeadmControlPlane) *VarsTemplateContent {
	// add kubespray init config here
	varContent := &VarsTemplateContent{
		PodCIDR:     c.Spec.ClusterNetwork.Pods.CIDRBlocks[0],
		KubeVersion: kcp.Spec.Version,
		FileRepo:    DefaultFileRepo,
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
		EtcdNodeName: make([]string, len(masterMachine)+len(nodeMachine)),
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

func (r *CustomClusterController) CreatConfigMapWithTemplate(name, namespace, fileName, configMapData string) error {
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
	var err error
	if ConfigMap, err = r.ClientSet.CoreV1().ConfigMaps(ConfigMap.Namespace).Create(context.Background(), ConfigMap, metav1.CreateOptions{}); err != nil {
		return err
	}
	return nil
}

func (r *CustomClusterController) CreateHostsConfigMap(customMachine *v1alpha1.CustomMachine, customCluster *v1alpha1.CustomCluster) error {
	hostsContent := GetHostsContent(customMachine)
	hostData := &strings.Builder{}

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
	name := fmt.Sprintf("%s-%s", customCluster.Name, HostYamlFileName)
	namespace := customCluster.Namespace

	return r.CreatConfigMapWithTemplate(name, namespace, HostYamlFileName, hostData.String())
}

func (r *CustomClusterController) CreateVarsConfigMap(c *clusterv1.Cluster, kcp *controlplanev1.KubeadmControlPlane, cc *v1alpha1.CustomCluster) error {
	VarsContent := GetVarContent(c, kcp)
	VarsData := &strings.Builder{}

	tmpl := template.Must(template.New("").Parse(`
kube_version: {{ .KubeVersion}}
download_run_once: true
download_container: false
download_localhost: true
# network
kube_pods_subnet: {{ .PodCIDR }}
# gcr and kubernetes image repo define
gcr_image_repo: "gcr.m.daocloud.io"
kube_image_repo: "k8s.m.daocloud.io"
# docker image repo define
docker_image_repo: "docker.m.daocloud.io"
# quay image repo define
quay_image_repo: "quay.m.daocloud.io"
github_image_repo: "ghcr.m.daocloud.io"
files_repo: "{{ .FileRepo }}"
kubeadm_download_url: "{{ .FileRepo }}/storage.googleapis.com/kubernetes-release/release/{{ .KubeVersion }}/bin/linux/{{ "{{ image_arch }}" }}/kubeadm"
kubectl_download_url: "{{ .FileRepo }}/storage.googleapis.com/kubernetes-release/release/{{ .KubeVersion }}/bin/linux/{{ "{{ image_arch }}" }}/kubectl"
kubelet_download_url: "{{ .FileRepo }}/storage.googleapis.com/kubernetes-release/release/{{ .KubeVersion }}/bin/linux/{{ "{{ image_arch }}" }}/kubelet"
`))

	if err := tmpl.Execute(VarsData, VarsContent); err != nil {
		return err
	}
	name := fmt.Sprintf("%s-%s", cc.Name, VarsYamlFileName)
	namespace := cc.Namespace

	return r.CreatConfigMapWithTemplate(name, namespace, VarsYamlFileName, VarsData.String())
}

func (r *CustomClusterController) UpdateHostsConfigMap(ctx context.Context, customCluster *v1alpha1.CustomCluster, customMachine *v1alpha1.CustomMachine) error {
	log := ctrl.LoggerFrom(ctx)
	// delete origin cm
	if err := r.ClientSet.CoreV1().ConfigMaps(customCluster.Namespace).Delete(context.Background(), customCluster.Name+"-"+HostYamlFileName, metav1.DeleteOptions{}); err != nil {
		if !apierrors.IsNotFound(err) {
			log.Error(err, "failed to  delete hosts configMap")
			return err
		}
	}
	return r.CreateHostsConfigMap(customMachine, customCluster)
}

func (r *CustomClusterController) UpdateVarsConfigMap(ctx context.Context, customCluster *v1alpha1.CustomCluster, cc *v1alpha1.CustomCluster, cluster *clusterv1.Cluster, kcp *controlplanev1.KubeadmControlPlane) error {
	log := ctrl.LoggerFrom(ctx)
	// delete origin cm
	if err := r.ClientSet.CoreV1().ConfigMaps(customCluster.Namespace).Delete(context.Background(), customCluster.Name+"-"+VarsYamlFileName, metav1.DeleteOptions{}); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		log.Error(err, "failed to  delete vars configMap")
		return err
	}

	return r.CreateVarsConfigMap(cluster, kcp, cc)
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

	if c.Watch(
		&source.Kind{Type: &clusterv1.Cluster{}},
		handler.EnqueueRequestsFromMapFunc(r.ClusterToCustomCluster),
	); err != nil {
		return fmt.Errorf("failed adding Watch for Clusters to controller manager: %v", err)
	}

	if c.Watch(
		&source.Kind{Type: &v1alpha1.CustomMachine{}},
		handler.EnqueueRequestsFromMapFunc(r.CustomMachineToCustomCluster),
	); err != nil {
		return fmt.Errorf("failed adding Watch for CustomMachine to controller manager: %v", err)
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

func (r *CustomClusterController) CustomMachineToCustomCluster(o client.Object) []ctrl.Request {
	c, ok := o.(*v1alpha1.CustomMachine)
	if !ok {
		panic(fmt.Sprintf("Expected a CustomMachine but got a %T", o))
	}

	machineOwner := c.Spec.MachineOwner
	if machineOwner != nil && machineOwner.Kind == "CustomCluster" {
		return []ctrl.Request{{NamespacedName: client.ObjectKey{Namespace: machineOwner.Namespace, Name: machineOwner.Name}}}
	}

	return nil
}
