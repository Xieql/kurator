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

	"github.com/coreos/go-semver/semver"
	corev1 "k8s.io/api/core/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	controlplanev1 "sigs.k8s.io/cluster-api/controlplane/kubeadm/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/conditions"
	ctrl "sigs.k8s.io/controller-runtime"

	"kurator.dev/kurator/pkg/apis/infra/v1alpha1"
)

// reconcileUpgrade is responsible for handling the customCluster reconciliation process of cluster upgrading to targetVersion
func (r *CustomClusterController) reconcileUpgrade(ctx context.Context, customCluster *v1alpha1.CustomCluster, kcp *controlplanev1.KubeadmControlPlane, targetVersion string) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)

	cmd := generateUpgradeManageCMD(targetVersion)
	// Checks whether the worker node for upgrading already exists. If it does not exist, then create it.
	workerPod, err1 := r.ensureWorkerPodCreated(ctx, customCluster, CustomClusterUpgradeAction, cmd, generateClusterHostsName(customCluster), generateClusterConfigName(customCluster))
	if err1 != nil {
		conditions.MarkFalse(customCluster, v1alpha1.UpgradeCondition, v1alpha1.FailedCreateUpgradeWorker,
			clusterv1.ConditionSeverityWarning, "upgrade worker not ready %s/%s.", customCluster.Namespace, customCluster.Name)
		log.Error(err1, "failed to ensure that upgrade WorkerPod is created", "customCluster", customCluster.Name)
		return ctrl.Result{}, err1
	}

	// Check the current customCluster status.
	if customCluster.Status.Phase != v1alpha1.UpgradingPhase {
		customCluster.Status.Phase = v1alpha1.UpgradingPhase
		log.Info(fmt.Sprintf("customCluster's phase changes from %s to %s", string(v1alpha1.ProvisionedPhase), string(v1alpha1.UpgradingPhase)))
	}

	// Determine the progress of upgrading based on the status of the workerPod.
	if workerPod.Status.Phase == corev1.PodSucceeded {
		// Update the cluster-config to ensure that the current cluster-config and kcp represents the cluster after upgrading.
		if err := r.updateKubeVersion(ctx, customCluster, targetVersion); err != nil {
			log.Error(err, "failed to update the kubeVersion of configmap cluster-config after upgrading")
			return ctrl.Result{}, err
		}
		// Restore the workerPod's status to "provisioned" after upgrading.
		customCluster.Status.Phase = v1alpha1.ProvisionedPhase
		log.Info(fmt.Sprintf("customCluster's phase changes from %s to %s", string(v1alpha1.UpgradingPhase), string(v1alpha1.ProvisionedPhase)))

		// Delete the upgrading worker.
		if err := r.ensureWorkerPodDeleted(ctx, generateWorkerKey(customCluster, CustomClusterUpgradeAction)); err != nil {
			log.Error(err, "failed to delete upgrade worker pod", "worker", generateWorkerKey(customCluster, CustomClusterUpgradeAction))
			return ctrl.Result{}, err
		}
		conditions.MarkTrue(customCluster, v1alpha1.UpgradeCondition)

		return ctrl.Result{}, nil
	}

	if workerPod.Status.Phase == corev1.PodFailed {
		customCluster.Status.Phase = v1alpha1.UnknownPhase
		conditions.MarkFalse(customCluster, v1alpha1.UpgradeCondition, v1alpha1.UpgradeWorkerRunFailedReason,
			clusterv1.ConditionSeverityWarning, "upgrade worker run failed %s/%s", customCluster.Namespace, customCluster.Name)

		return ctrl.Result{}, nil
	}

	return ctrl.Result{}, nil
}

// generateScaleDownManageCMD generate a kubespray cmd to upgrade cluster to desired kubeVersion.
func generateUpgradeManageCMD(kubeVersion string) customClusterManageCMD {
	if len(kubeVersion) == 0 {
		return ""
	}

	cmd := string(KubesprayUpgradeCMDPrefix) + " -e kube_version=v" + strings.TrimPrefix(kubeVersion, "v")

	return customClusterManageCMD(cmd)
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

// updateKubeVersion update the kubeVersion in configmap.
func (r *CustomClusterController) updateKubeVersion(ctx context.Context, customCluster *v1alpha1.CustomCluster, newKubeVersion string) error {
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

	return nil
}
