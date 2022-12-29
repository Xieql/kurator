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

	HostYamlFileName = "hosts.yaml"
	VarsYamlFileName = "vars.yaml"

	// default kubespray init config
	DefaultFileRepo         = "https://files.m.daocloud.io"
	DefaultHostArchitecture = "amd64"
	DefaultCniVersion       = "v1.1.1"
	DefaultKubesprayImage   = "quay.io/kubespray/kubespray:v2.20.0"

	KubesprayInitCMD       = "ansible-playbook -i inventory/hosts.yaml --private-key /root/.ssh/id_rsa cluster.yml"
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
		if apierrors.IsNotFound(err) {
			log.Info("--------------------customMachine not found")
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
		if apierrors.IsNotFound(err) {
			return ctrl.Result{Requeue: false}, nil
		}
		return ctrl.Result{RequeueAfter: RequeueAfter}, err
	}

	// TODO: check cluster status

	log = log.WithValues("Cluster", klog.KObj(cluster))
	ctx = ctrl.LoggerInto(ctx, log)

	log.Info("--------------------everything is ok let begin real reconcile")
	return r.reconcile(ctx, customCluster, customMachine, cluster, kcp)
}

func (r *CustomClusterController) reconcile(ctx context.Context, customCluster *v1alpha1.CustomCluster, customMachine *v1alpha1.CustomMachine, cluster *clusterv1.Cluster, kcp *controlplanev1.KubeadmControlPlane) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)
	log.Info("~~~~~~~~~~reconcile begin~~~~~~~~~")

	// TODO: 根据 Cluster KCP CustomMachine 及 SSH key secret生成kubespray参数 Configmap

	// TODO: 先删除再创建？

	if _, err := r.CreateHostsConfigMap(customMachine, customCluster); err != nil {
		log.Error(err, "failed to CreateHostsConfigMap")
		//return ctrl.Result{RequeueAfter: RequeueAfter}, err
	}

	if _, err := r.CreateVarsConfigMap(cluster, kcp, customCluster); err != nil {
		log.Error(err, "failed to CreateVarsConfigMap")
		//return ctrl.Result{RequeueAfter: RequeueAfter}, err
	}

	log.Info("***********~~~~~~~let's test create a job ~~~~~")

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
							Args:    []string{KubesprayShowConfigCMD},

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
									Name:      "id-rsa-conf",
									MountPath: "/root/.ssh",
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
										Name: customCluster.Name + HostYamlFileName,
									},
								},
							},
						},
						{
							Name: "vars-conf",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: customCluster.Name + VarsYamlFileName,
									},
								},
							},
						},
						{
							Name: "id-rsa-conf",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName:  SecretName,
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

//go:embed hosts.yaml.template
var hostsTemplate string

//go:embed vars.yaml.template
var varsTemplate string

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
	tmpl := template.Must(template.New(HostYamlFileName).Parse(hostsTemplate))
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
	tmpl := template.Must(template.New(VarsYamlFileName).Parse(varsTemplate))
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
