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

	//log.Info("***********~~~~~~~let's test create a job ~~~~~")
	//
	//curJob := r.CreateKubesprayInitClusterJob(ctx)
	//
	//var err error
	//
	//curJob, err = r.ClientSet.BatchV1().Jobs(curJob.Namespace).Create(context.Background(), curJob, metav1.CreateOptions{})
	//
	//if err != nil {
	//	log.Error(err, "~~~~~Failed to create job~~~~~~~~~~")
	//
	//	return ctrl.Result{}, err
	//}
	//log.Info("***********~~~~~~~create a job  success  ~~~~~")

	// Fetch the customCluster instance.
	customCluster := &v1alpha1.CustomCluster{}
	if err := r.Client.Get(ctx, req.NamespacedName, customCluster); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{Requeue: false}, nil
		}
		klog.Error(err)
		return ctrl.Result{RequeueAfter: RequeueAfter}, err
	}

	log.Info("--------------------customCluster is ok")

	// Fetch the CustomMachine instance.
	customMachinekey := client.ObjectKey{
		Namespace: customCluster.Spec.MachineRef.Namespace,
		Name:      customCluster.Spec.MachineRef.Name,
	}
	customMachine := &v1alpha1.CustomMachine{}
	log.Info("-------------------customMachinekey")

	klog.Infof("customMachine name is %s ns is %s", customMachinekey.Name, customMachinekey.Namespace)

	log.Info("--------------------customMachinekey not found")

	if err := r.Client.Get(ctx, customMachinekey, customMachine); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("--------------------customMachine not found")

			return ctrl.Result{Requeue: false}, nil
		}
		klog.Error(err)

		return ctrl.Result{RequeueAfter: RequeueAfter}, err
	}

	log.Info("--------------------customMachinekey is ok")

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
		klog.Error(err)
		return ctrl.Result{RequeueAfter: RequeueAfter}, err
	}
	log.Info("--------------------cluster is ok")

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
		klog.Error(err)
		return ctrl.Result{RequeueAfter: RequeueAfter}, err
	}
	log.Info("--------------------kcp is ok")

	// TODO: check cluster status

	log = log.WithValues("Cluster", klog.KObj(cluster))
	ctx = ctrl.LoggerInto(ctx, log)

	log.Info("--------------------everything is ok let begin real reconcile")
	return r.reconcile(ctx, customCluster, customMachine, cluster, kcp)
}

func (r *CustomClusterController) reconcile(ctx context.Context, customCluster *v1alpha1.CustomCluster, customMachine *v1alpha1.CustomMachine, cluster *clusterv1.Cluster, kcp *controlplanev1.KubeadmControlPlane) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)
	log.Info("~~~~~~~~~~reconcile begin~~~~~~~~~")
	// 这里认为相关资源状态正常

	// TODO: 根据 Cluster KCP CustomMachine 及 SSH key secret生成kubespray参数 Configmap

	log.Info("~~~~~~~~~~create hosts configmap from customMachine~~~~~~~~~~~~~")
	log.Info("$$$$$$$$$$$$$$$$$$$$$$~~~~~~~~~~createHostVars begin~~~~~~~~~")

	// 先删除再创建？
	_, err := r.CreateHostsConfigMap(ctx, customMachine, customCluster)
	if err != nil {
		klog.Infof("!!!!!!!!!!!!!!!!!!!!!!!!!!CreateHostsConfigMap error %v", err)
		//return ctrl.Result{RequeueAfter: RequeueAfter}, err
	}

	log.Info("~~~~~~~~~~create configmap from ssh key secret / kcp /customMachine~~~~~~~~~~~~~")

	if _, err := r.CreateVarsConfigMap(ctx, cluster, kcp, customCluster); err != nil {
		klog.Error(err)
		klog.Infof("!!!!!!!!!!!!!!!!!!!!!!!!!!CreateHostsConfigMap error %v", err)

		return ctrl.Result{RequeueAfter: RequeueAfter}, err
	}

	// 创建Pod （挂载SSH证书以及配置参数） 执行ansible-playbook创建集群
	log.Info("~~~~~~~~~~create job/pod to exec ansible-playbook~~~~~~~~")

	log.Info("~~~~~~~~~~exec the job~~~~~~~~")

	log.Info("~~~~~~~~~~reconcile end~~~~~~~~~~")
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *CustomClusterController) SetupWithManager(ctx context.Context, mgr ctrl.Manager, options controller.Options) error {
	log := ctrl.LoggerFrom(ctx)

	log.Info("~~~~~~~~~ cc SetupWithManager begin~~~~~~~~~~")

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

	log.Info("~~~~~~~~~ cc SetupWithManager finish~~~~~~~~~~")

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

// CreateKubesprayInitClusterJob create a kubespray init cluster job from configMap, or check exist first ?
func (r *CustomClusterController) CreateKubesprayInitClusterJob(ctx context.Context) *batchv1.Job {
	clusterName := "test-cluster"
	defaultNamespace := "default"
	defaultImage := "quay.io/kubespray/kubespray:v2.20.0"

	jobName := clusterName + "-kubespray-init-cluster-job"
	namespace := defaultNamespace
	image := defaultImage
	containerName := clusterName + "-container"
	DefaultMode := int32(0o600)
	log := ctrl.LoggerFrom(ctx)
	log.Info("~#############~~~~~~~~~CreateKubesprayInitClusterJob 1~~~~~~~~~")

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
							Image:   image,
							Command: []string{"/bin/sh", "-c"},
							Args:    []string{"ansible-playbook -i inventory/hosts.yaml --private-key /root/.ssh/id_rsa cluster.yml"},

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
										Name: "hosts-conf",
									},
								},
							},
						},
						{
							Name: "vars-conf",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: "vars-conf",
									},
								},
							},
						},
						{
							Name: "id-rsa-conf",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName:  "id-rsa-conf",
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

	log.Info("~#############~~~~~~~~~CreateKubesprayInitClusterJob 2~~~~~~~~~")

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
	KubeVersion string
	PodCIDR     string
}

func GetVarContent(c *clusterv1.Cluster, kcp *controlplanev1.KubeadmControlPlane) *VarsTemplateContent {
	// add kubespray init config here
	varContent := &VarsTemplateContent{
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
		EtcdNodeName: make([]string, len(masterMachine)+len(nodeMachine)),
	}

	for _, machine := range masterMachine {
		masterName := machine.HostName
		nodeAndIp := fmt.Sprintf("%s ansible_host=%s ip=%s", machine.HostName, machine.PublicIP, machine.PrivateIP)
		hostVar.MasterName = append(hostVar.MasterName, masterName)
		hostVar.EtcdNodeName = append(hostVar.EtcdNodeName, masterName)
		hostVar.NodeAndIP = append(hostVar.NodeAndIP, nodeAndIp)
	}

	for _, machine := range nodeMachine {
		nodeName := machine.HostName
		nodeAndIp := fmt.Sprintf("%s ansible_host=%s ip=%s", machine.HostName, machine.PublicIP, machine.PrivateIP)
		hostVar.NodeName = append(hostVar.MasterName, nodeName)
		hostVar.EtcdNodeName = append(hostVar.EtcdNodeName, nodeName)
		hostVar.NodeAndIP = append(hostVar.NodeAndIP, nodeAndIp)
	}

	return hostVar
}

func (r *CustomClusterController) CreatConfigMapWithTemplate(name, namespace, fileName, configMapData string) (bool, error) {
	hostVarConfigMap := &corev1.ConfigMap{
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
	if hostVarConfigMap, err = r.ClientSet.CoreV1().ConfigMaps(hostVarConfigMap.Namespace).Create(context.Background(), hostVarConfigMap, metav1.CreateOptions{}); err != nil {
		return false, err
	}
	return true, nil
}

func (r *CustomClusterController) CreateHostsConfigMap(ctx context.Context, customMachine *v1alpha1.CustomMachine, customCluster *v1alpha1.CustomCluster) (bool, error) {
	log := ctrl.LoggerFrom(ctx)
	log.Info("$$$$$$$$$$$$$$$$$$$$$$~~~~~~~~~~createHostVars begin~~~~~~~~~")
	hostsContent := GetHostsContent(customMachine)

	hostData := &strings.Builder{}
	tmpl := template.Must(template.New("hostVar").Parse(hostsTemplate))
	if err := tmpl.Execute(hostData, hostsContent); err != nil {
		return false, err
	}
	name := fmt.Sprintf("%s-%s", customCluster.Name, HostYamlFileName)
	namespace := customCluster.Namespace
	log.Info("$$$$$$$$$$$$$$$$$$$$$$~~~~~~~~~~createHostVars CreatConfigMapWithTemplate begin~~~~~~~~~")

	return r.CreatConfigMapWithTemplate(name, namespace, HostYamlFileName, hostData.String())
}

func (r *CustomClusterController) CreateVarsConfigMap(ctx context.Context, c *clusterv1.Cluster, kcp *controlplanev1.KubeadmControlPlane, cc *v1alpha1.CustomCluster) (bool, error) {
	log := ctrl.LoggerFrom(ctx)
	log.Info("$$$$$$$$$$$$$$$$$$$$$$~~~~~~~~~~CreateVarsConfigMap begin~~~~~~~~~")
	VarsContent := GetVarContent(c, kcp)

	VarsData := &strings.Builder{}
	tmpl := template.Must(template.New("hostVar").Parse(hostsTemplate))
	if err := tmpl.Execute(VarsData, VarsContent); err != nil {
		return false, err
	}
	name := fmt.Sprintf("%s-%s", cc.Name, VarsYamlFileName)
	namespace := cc.Namespace
	log.Info("$$$$$$$$$$$$$$$$$$$$$$~~~~~~~~~~createHostVars CreatConfigMapWithTemplate begin~~~~~~~~~")

	return r.CreatConfigMapWithTemplate(name, namespace, VarsYamlFileName, VarsData.String())
}
