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
	ClientSet kubernetes.Interface

	APIReader client.Reader
	Scheme    *runtime.Scheme

	externalTracker external.ObjectTracker
}

const (
	RequeueAfter = time.Second * 5
	// HostYamlFileName configmap's default name is customCluster.Name + "-" + YamlFileName
	HostYamlFileName = "hosts.yaml"
	// VarsYamlFileName varsConfigmap's default name is customCluster.Name + "-" + VarsYamlFileName
	VarsYamlFileName = "vars.yaml"
	secreteName      = "ssh-quickstart"

	// default kubespray init config
	DefaultFileRepo         = "https://files.m.daocloud.io"
	DefaultHostArchitecture = "amd64"
	DefaultCniVersion       = "v1.1.1"
	DefaultKubesprayImage   = "quay.io/kubespray/kubespray:v2.20.0"

	KubesprayInitCMD = "ansible-playbook -i inventory/hosts.yaml --private-key /root/.ssh/id_rsa cluster.yml"

	// maybe need reset
	KubesprayResetCMD      = "ansible-playbook -e reset_confirmation=yes -i inventory/hosts.yaml reset.yml -vvv"
	KubesprayShowConfigCMD = "cat /root/.ssh/id_rsa && cat /kubespray/inventory/hosts.yaml && cat /kubespray/inventory/group_var/all/vars.yaml"

	SecretName = "ssh-key-quickstart "
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
	log.Info("***********~~~~~~~CustomClusterController reconcile begin~~~~~~~~~")

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
	// 如果cluster是初始状态，则开始后续； 如果正在创建中running,则检查一次job状态，然后重新排队/更新状态为完成、失败 ；如果cluster已经完成，；如果cluster失败，则执行reset； reseting状态，检查resetjob，如果完成，则重新cluster、

	log = log.WithValues("Cluster", klog.KObj(cluster))
	ctx = ctrl.LoggerInto(ctx, log)

	return r.reconcile(ctx, customCluster, customMachine, cluster, kcp)
}

func (r *CustomClusterController) RemoveCustomClusterConfigMap(ctx context.Context, customCluster *v1alpha1.CustomCluster) error {
	log := ctrl.LoggerFrom(ctx)
	if err := r.ClientSet.CoreV1().ConfigMaps(customCluster.Namespace).Delete(context.Background(), customCluster.Name+"-"+HostYamlFileName, metav1.DeleteOptions{}); err != nil {
		log.Error(err, "failed to  delete hosts configMap")
		return err
	}
	if err := r.ClientSet.CoreV1().ConfigMaps(customCluster.Namespace).Delete(context.Background(), customCluster.Name+"-"+VarsYamlFileName, metav1.DeleteOptions{}); err != nil {
		log.Error(err, "failed to  delete vars configMap")
		return err
	}
	return nil
}

func (r *CustomClusterController) reconcile(ctx context.Context, customCluster *v1alpha1.CustomCluster, customMachine *v1alpha1.CustomMachine, cluster *clusterv1.Cluster, kcp *controlplanev1.KubeadmControlPlane) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)

	// TODO: 根据 Cluster KCP CustomMachine 及 SSH key secret生成kubespray参数 Configmap

	// 删除cm重新根据当前对象创建新的cm
	if err := r.RemoveCustomClusterConfigMap(ctx, customCluster); err != nil {
		log.Error(err, "failed to remove customCluster configMap")
		return ctrl.Result{RequeueAfter: RequeueAfter}, err
	}

	if _, err := r.CreateHostsConfigMap(customMachine, customCluster); err != nil {
		log.Error(err, "failed to create hosts configMap")
		return ctrl.Result{RequeueAfter: RequeueAfter}, err
	}

	if _, err := r.CreateVarsConfigMap(cluster, kcp, customCluster); err != nil {
		log.Error(err, "failed to create vars configMap")
		return ctrl.Result{RequeueAfter: RequeueAfter}, err
	}

	initClusterJob := r.CreateKubesprayInitClusterJob(ctx, customCluster)

	if curJob, err := r.ClientSet.BatchV1().Jobs(initClusterJob.Namespace).Create(context.Background(), initClusterJob, metav1.CreateOptions{}); err != nil {
		log.Error(err, "failed to create job", "jobName", curJob.Name)
		return ctrl.Result{}, err
	}

	log.Info("***********~~~~~~~create a job  success  ~~~~~")

	log.Info("~~~~~~~~~~exec the job~~~~~~~~")

	log.Info("~~~~~~~~~~reconcile end~~~~~~~~~~")
	return ctrl.Result{}, nil
}

// CreateKubesprayInitClusterJob create a kubespray init cluster job from configMap, or check exist first ?
func (r *CustomClusterController) CreateKubesprayInitClusterJob(ctx context.Context, customCluster *v1alpha1.CustomCluster) *batchv1.Job {
	log := ctrl.LoggerFrom(ctx)
	log.Info("~#############~~~~~~~~~CreateKubesprayInitClusterJob ~~~~~~~~~")

	jobName := customCluster.Name + "-kubespray-init-cluster"
	namespace := customCluster.Namespace
	containerName := customCluster.Name + "-container"
	//DefaultMode := int32(0o600)
	PrivatekeyMode := int32(0o400)

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
									Name:      "ssh-auth",
									MountPath: "/auth/ssh-privatekey",
									SubPath:   "ssh-privatekey",
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
									},
								},
							},
						},
						{
							Name: "ssh-auth",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									// todo: read from customMachine
									SecretName: secreteName,
									// todo: fix it
									DefaultMode: &PrivatekeyMode, // fix Permissions 0644 are too open
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
	KubeVersion      string
	PodCIDR          string
	FileRepo         string
	HostArchitecture string
	CniVersion       string
}

func GetVarContent(c *clusterv1.Cluster, kcp *controlplanev1.KubeadmControlPlane) *VarsTemplateContent {
	// add kubespray init config here
	varContent := &VarsTemplateContent{
		PodCIDR:          c.Spec.ClusterNetwork.Pods.CIDRBlocks[0],
		KubeVersion:      kcp.Spec.Version,
		FileRepo:         DefaultFileRepo,
		HostArchitecture: DefaultHostArchitecture,
		CniVersion:       DefaultCniVersion,
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
		hostVar.EtcdNodeName[count] = nodeName
		hostVar.NodeAndIP[count] = nodeAndIp
		count++
	}

	return hostVar
}

func (r *CustomClusterController) CreatConfigMapWithTemplate(name, namespace, fileName, configMapData string) (bool, error) {
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
		return false, err
	}
	return true, nil
}

func (r *CustomClusterController) CreateHostsConfigMap(customMachine *v1alpha1.CustomMachine, customCluster *v1alpha1.CustomCluster) (bool, error) {
	hostsContent := GetHostsContent(customMachine)
	hostData := &strings.Builder{}

	// If read the template file, an extra /r will appear, it will make kubespray fail to read the configuration
	tmpl := template.Must(template.New("").Parse(`
[all]
{{ range $v := .NodeAndIP }}
{{- $v }}
{{ end -}}
[kube_control_plane]
{{- range $v := .MasterName }}
{{ $v }}
{{ end -}}
[etcd]
{{- range $v := .EtcdNodeName }}
{{ $v -}}
{{ end -}}
[kube_node]
{{- range $v := .NodeName }}
{{ $v }}
{{ end -}}
[k8s-cluster:children]
kube-master
kube-node
`))

	if err := tmpl.Execute(hostData, hostsContent); err != nil {
		return false, err
	}
	name := fmt.Sprintf("%s-%s", customCluster.Name, HostYamlFileName)
	namespace := customCluster.Namespace

	return r.CreatConfigMapWithTemplate(name, namespace, HostYamlFileName, hostData.String())
}

func (r *CustomClusterController) CreateVarsConfigMap(c *clusterv1.Cluster, kcp *controlplanev1.KubeadmControlPlane, cc *v1alpha1.CustomCluster) (bool, error) {
	VarsContent := GetVarContent(c, kcp)
	VarsData := &strings.Builder{}

	tmpl := template.Must(template.New("").Parse(`
# Download Config
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
# github image repo define (ex multus only use that)
github_image_repo: "ghcr.m.daocloud.io"
# Kubernetes components
kubeadm_download_url: "{{ .FileRepo }}/storage.googleapis.com/kubernetes-release/release/{{ .KubeVersion }}/bin/linux/{{ .HostArchitecture }}/kubeadm"
kubectl_download_url: "{{ .FileRepo }}/storage.googleapis.com/kubernetes-release/release/{{ .KubeVersion }}/bin/linux/{{ .HostArchitecture }}/kubectl"
kubelet_download_url: "{{ .FileRepo }}/storage.googleapis.com/kubernetes-release/release/{{ .KubeVersion }}/bin/linux/{{ .HostArchitecture }}/kubelet"
# CNI Plugins
cni_download_url: "{{ .FileRepo }}/github.com/containernetworking/plugins/releases/download/{{ .CniVersion }}/cni-plugins-linux-{{ .HostArchitecture }}-{{ .CniVersion }}.tgz"
`))

	if err := tmpl.Execute(VarsData, VarsContent); err != nil {
		return false, err
	}
	name := fmt.Sprintf("%s-%s", cc.Name, VarsYamlFileName)
	namespace := cc.Namespace

	return r.CreatConfigMapWithTemplate(name, namespace, VarsYamlFileName, VarsData.String())
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
