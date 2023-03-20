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
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"

	"kurator.dev/kurator/pkg/apis/infra/v1alpha1"
)

var master1 = v1alpha1.Machine{
	HostName:  "master1",
	PrivateIP: "1.1.1.1",
	PublicIP:  "2.2.2.2",
}

var node1 = v1alpha1.Machine{
	HostName:  "node1",
	PrivateIP: "3.3.3.3",
	PublicIP:  "4.4.4.4",
}

var node2 = v1alpha1.Machine{
	HostName:  "node2",
	PrivateIP: "5.5.5.5",
	PublicIP:  "6.6.6.6",
}

var curCustomMachineSingle = &v1alpha1.CustomMachine{
	Spec: v1alpha1.CustomMachineSpec{
		Master: []v1alpha1.Machine{master1},
		Nodes:  []v1alpha1.Machine{node1},
	},
}

var curCustomMachineMulti = &v1alpha1.CustomMachine{
	Spec: v1alpha1.CustomMachineSpec{
		Master: []v1alpha1.Machine{master1},
		Nodes:  []v1alpha1.Machine{node1, node2},
	},
}

func TestGetHostsContent(t *testing.T) {
	expectHost1 := &HostTemplateContent{
		NodeAndIP:    []string{"master1 ansible_host=2.2.2.2 ip=1.1.1.1", "node1 ansible_host=4.4.4.4 ip=3.3.3.3"},
		MasterName:   []string{"master1"},
		NodeName:     []string{"node1"},
		EtcdNodeName: []string{"master1"},
	}
	assert.Equal(t, expectHost1, GetHostsContent(curCustomMachineSingle))

	expectHost2 := &HostTemplateContent{
		NodeAndIP:    []string{"master1 ansible_host=2.2.2.2 ip=1.1.1.1", "node1 ansible_host=4.4.4.4 ip=3.3.3.3", "node2 ansible_host=6.6.6.6 ip=5.5.5.5"},
		MasterName:   []string{"master1"},
		NodeName:     []string{"node1", "node2"},
		EtcdNodeName: []string{"master1"},
	}

	assert.Equal(t, expectHost2, GetHostsContent(curCustomMachineMulti))
}

var targetWorkerNodesSingle = []NodeInfo{
	{
		NodeName:  "node1",
		PrivateIP: "3.3.3.3",
		PublicIP:  "4.4.4.4",
	},
}

var targetClusterInfoSingle = &ClusterInfo{
	WorkerNodes: targetWorkerNodesSingle,
}

var targetWorkerNodesMulti = []NodeInfo{
	{
		NodeName:  "node1",
		PrivateIP: "3.3.3.3",
		PublicIP:  "4.4.4.4",
	},
	{
		NodeName:  "node2",
		PrivateIP: "5.5.5.5",
		PublicIP:  "6.6.6.6",
	},
}

var targetClusterInfoMulti = &ClusterInfo{
	WorkerNodes: targetWorkerNodesMulti,
}

func TestGetWorkerNodesFromCustomMachine(t *testing.T) {
	workerNodes1 := getWorkerNodesFromCustomMachine(curCustomMachineSingle)
	assert.Equal(t, targetWorkerNodesSingle, workerNodes1)

	workerNodes2 := getWorkerNodesFromCustomMachine(curCustomMachineMulti)
	assert.Equal(t, targetWorkerNodesMulti, workerNodes2)
}

func TestDesiredClusterInfo(t *testing.T) {
	cc := &v1alpha1.CustomCluster{}
	clusterInfo1 := getDesiredClusterInfo(curCustomMachineSingle, cc)
	assert.Equal(t, targetClusterInfoSingle, clusterInfo1)

	clusterInfo2 := getDesiredClusterInfo(curCustomMachineMulti, cc)
	assert.Equal(t, targetClusterInfoMulti, clusterInfo2)
}

var workerNode1 = NodeInfo{
	NodeName:  "node1",
	PublicIP:  "200.1.1.1",
	PrivateIP: "127.1.1.1",
}

var workerNode2 = NodeInfo{
	NodeName:  "node2",
	PublicIP:  "200.1.1.2",
	PrivateIP: "127.1.1.2",
}

var workerNode3 = NodeInfo{
	NodeName:  "node3",
	PublicIP:  "200.1.1.3",
	PrivateIP: "127.1.1.3",
}

var provisionedNodes = []NodeInfo{workerNode1, workerNode3}

var curNodes1 = []NodeInfo{workerNode2, workerNode3}

var curNodes2 = []NodeInfo{workerNode2, workerNode3, workerNode1}

var curNodes3 = []NodeInfo{workerNode1}

func TestFindScaleUpWorkerNodes(t *testing.T) {
	scaleUpNodes1 := findScaleUpWorkerNodes(provisionedNodes, curNodes1)
	assert.Equal(t, []NodeInfo{workerNode2}, scaleUpNodes1)

	scaleUpNodes2 := findScaleUpWorkerNodes(provisionedNodes, curNodes2)
	assert.Equal(t, []NodeInfo{workerNode2}, scaleUpNodes2)

	scaleUpNodes3 := findScaleUpWorkerNodes(provisionedNodes, curNodes3)
	assert.Equal(t, 0, len(scaleUpNodes3))

	scaleUpNodes4 := findScaleUpWorkerNodes(nil, curNodes2)
	assert.Equal(t, curNodes2, scaleUpNodes4)

	scaleUpNodes5 := findScaleUpWorkerNodes(curNodes2, nil)
	assert.Equal(t, 0, len(scaleUpNodes5))
}

func TestFindScaleDownWorkerNodes(t *testing.T) {
	scaleDoneNodes1 := findScaleDownWorkerNodes(provisionedNodes, curNodes1)
	assert.Equal(t, []NodeInfo{workerNode1}, scaleDoneNodes1)

	scaleDoneNodes2 := findScaleDownWorkerNodes(provisionedNodes, curNodes2)
	assert.Equal(t, 0, len(scaleDoneNodes2))

	scaleDoneNodes3 := findScaleDownWorkerNodes(provisionedNodes, curNodes3)
	assert.Equal(t, []NodeInfo{workerNode3}, scaleDoneNodes3)

	scaleDoneNodes4 := findScaleDownWorkerNodes(nil, curNodes2)
	assert.Equal(t, 0, len(scaleDoneNodes4))

	scaleDoneNodes5 := findScaleDownWorkerNodes(curNodes2, nil)
	assert.Equal(t, curNodes2, scaleDoneNodes5)
}

var nodeNeedDelete1 []NodeInfo
var nodeNeedDelete2 = []NodeInfo{workerNode1}
var nodeNeedDelete3 = []NodeInfo{workerNode1, workerNode2, workerNode3}

func TestGenerateScaleDownManageCMD(t *testing.T) {
	scaleDownCMD1 := generateScaleDownManageCMD(nodeNeedDelete1)
	assert.Equal(t, customClusterManageCMD(""), scaleDownCMD1)

	scaleDownCMD2 := generateScaleDownManageCMD(nodeNeedDelete2)
	assert.Equal(t, customClusterManageCMD("ansible-playbook -i inventory/cluster-hosts --private-key /root/.ssh/ssh-privatekey remove-node.yml -vvv -e skip_confirmation=yes --extra-vars \"node=node1\" "), scaleDownCMD2)

	scaleDownCMD3 := generateScaleDownManageCMD(nodeNeedDelete3)
	assert.Equal(t, customClusterManageCMD("ansible-playbook -i inventory/cluster-hosts --private-key /root/.ssh/ssh-privatekey remove-node.yml -vvv -e skip_confirmation=yes --extra-vars \"node=node1,node2,node3\" "), scaleDownCMD3)
}

var clusterHostDataStr1 = "[all]\n\nmaster1 ansible_host=200.1.1.0 ip=127.1.1.0\n\nnode1 ansible_host=200.1.1.1 ip=127.1.1.1\n\n[kube_control_plane]\n\nmaster1\n\n[etcd]\nmaster1\n[kube_node]\nnode1\n[k8s-cluster:children]\nkube_node\nkube_control_plane"
var clusterHostDataStr2 = "[all]\n\nmaster1 ansible_host=200.1.1.0 ip=127.1.1.0\n\nnode1 ansible_host=200.1.1.1 ip=127.1.1.1\n\n[kube_control_plane]\n\nmaster1\n\n[etcd]\nmaster1\n[kube_node]\n\n[k8s-cluster:children]\nkube_node\nkube_control_plane"
var clusterHostDataStr3 = "[all]\n\nmaster1 ansible_host=200.1.1.0 ip=127.1.1.0\n\nnode1 ansible_host=200.1.1.1 ip=127.1.1.1\n\n\nnode2 ansible_host=200.1.1.2 ip=127.1.1.2\nnode3 ansible_host=200.1.1.3 ip=127.1.1.3\n[kube_control_plane]\n\nmaster1\n\n[etcd]\nmaster1\n[kube_node]\nnode1\n\nnode2\nnode3\n[k8s-cluster:children]\nkube_node\nkube_control_plane"

var clusterHost1 = &corev1.ConfigMap{
	Data: map[string]string{
		ClusterHostsName: clusterHostDataStr1,
	},
}

var clusterHost2 = &corev1.ConfigMap{
	Data: map[string]string{
		ClusterHostsName: clusterHostDataStr2,
	},
}

var clusterHost3 = &corev1.ConfigMap{
	Data: map[string]string{
		ClusterHostsName: clusterHostDataStr3,
	},
}

var masterNode = NodeInfo{
	NodeName:  "master1",
	PublicIP:  "200.1.1.0",
	PrivateIP: "127.1.1.0",
}

func TestGetWorkerNodeInfoFromClusterHost(t *testing.T) {
	nodeInfoArr1 := getWorkerNodeInfoFromClusterHost(clusterHost1)
	assert.Equal(t, []NodeInfo{workerNode1}, nodeInfoArr1)

	nodeInfoArr2 := getWorkerNodeInfoFromClusterHost(clusterHost2)
	assert.Equal(t, 0, len(nodeInfoArr2))

	nodeInfoArr3 := getWorkerNodeInfoFromClusterHost(clusterHost3)
	assert.Equal(t, []NodeInfo{workerNode1, workerNode2, workerNode3}, nodeInfoArr3)
}

var nodeStr1 = "master1 ansible_host=200.1.1.0 ip=127.1.1.0"
var nodeStr2 = "node1 ansible_host=200.1.1.1 ip=127.1.1.1"

func TestGetNodeInfoFromNodeStr(t *testing.T) {
	hostName1, nodeInfo1 := getNodeInfoFromNodeStr(nodeStr1)
	assert.Equal(t, "master1", hostName1)
	assert.Equal(t, masterNode, nodeInfo1)

	hostName2, nodeInfo2 := getNodeInfoFromNodeStr(nodeStr2)
	assert.Equal(t, "node1", hostName2)
	assert.Equal(t, workerNode1, nodeInfo2)
}

func TestGetScaleUpConfigMapData(t *testing.T) {
	ans := getScaleUpConfigMapData(clusterHostDataStr1, curNodes1)
	assert.Equal(t, clusterHostDataStr3, ans)
}

func TestIsKubeadmUpgradeSupported(t *testing.T) {
	// Test case 1: Same version
	assert.True(t, isKubeadmUpgradeSupported("v1.18.0", "v1.18.0"))
	assert.True(t, isKubeadmUpgradeSupported("1.18.0", "1.18.0"))

	// Test case 2: Minor version difference is 1
	assert.True(t, isKubeadmUpgradeSupported("v1.18.0", "v1.19.0"))
	assert.True(t, isKubeadmUpgradeSupported("v1.19.0", "v1.18.0"))
	assert.True(t, isKubeadmUpgradeSupported("1.19.9", "1.18.0"))
	assert.True(t, isKubeadmUpgradeSupported("1.18.9", "1.19.1"))
	assert.True(t, isKubeadmUpgradeSupported("v1.18.9", "1.19.1"))

	// Test case 3: Minor version difference is more than 1
	assert.False(t, isKubeadmUpgradeSupported("v1.18.0", "v1.20.0"))
	assert.False(t, isKubeadmUpgradeSupported("1.20.0", "1.18.0"))
	assert.False(t, isKubeadmUpgradeSupported("1.18.0", "v1.20.0"))

	// Test case 4: Invalid version string
	assert.False(t, isKubeadmUpgradeSupported("1.18", "1.19.0"))
	assert.False(t, isKubeadmUpgradeSupported("1.18.0", "1.19"))
	assert.False(t, isKubeadmUpgradeSupported("1.18.0", "1.x.0"))
	assert.False(t, isKubeadmUpgradeSupported("vv1.18.9", "1.19.1"))
}

func TestIsSupportedVersion(t *testing.T) {
	tests := []struct {
		name           string
		desiredVersion string
		minVersion     string
		maxVersion     string
		expectedResult bool
	}{
		{
			name:           "desired version is between min and max versions",
			desiredVersion: "1.2.3",
			minVersion:     "1.0.0",
			maxVersion:     "2.0.0",
			expectedResult: true,
		},
		{
			name:           "desired version is between min and max versions",
			desiredVersion: "v1.2.3",
			minVersion:     "1.0.0",
			maxVersion:     "2.0.0",
			expectedResult: true,
		},
		{
			name:           "desired version is equal to min version",
			desiredVersion: "1.0.0",
			minVersion:     "1.0.0",
			maxVersion:     "2.0.0",
			expectedResult: true,
		},
		{
			name:           "desired version is equal to max version",
			desiredVersion: "2.0.0",
			minVersion:     "1.0.0",
			maxVersion:     "2.0.0",
			expectedResult: true,
		},
		{
			name:           "desired version is less than min version",
			desiredVersion: "0.9.0",
			minVersion:     "1.0.0",
			maxVersion:     "2.0.0",
			expectedResult: false,
		},
		{
			name:           "desired version is greater than max version",
			desiredVersion: "3.0.0",
			minVersion:     "1.0.0",
			maxVersion:     "2.0.0",
			expectedResult: false,
		},
		{
			name:           "desired version is invalid",
			desiredVersion: "1.2.3.4",
			minVersion:     "1.0.0",
			maxVersion:     "2.0.0",
			expectedResult: false,
		},
	}

	for _, test := range tests {
		result := isSupportedVersion(test.desiredVersion, test.minVersion, test.maxVersion)
		assert.Equal(t, test.expectedResult, result)
	}
}

func TestGenerateUpgradeManageCMD(t *testing.T) {
	tests := []struct {
		name        string
		kubeVersion string
		want        customClusterManageCMD
	}{
		{
			name:        "valid kube version",
			kubeVersion: "v1.20.0",
			want:        "ansible-playbook -i inventory/cluster-hosts --private-key /root/.ssh/ssh-privatekey upgrade-cluster.yml -vvv  -e kube_version=v1.20.0",
		},
		{
			name:        "empty kube version",
			kubeVersion: "1.23.4",
			want:        "ansible-playbook -i inventory/cluster-hosts --private-key /root/.ssh/ssh-privatekey upgrade-cluster.yml -vvv  -e kube_version=v1.23.4",
		},
		{
			name:        "empty kube version",
			kubeVersion: "",
			want:        "",
		},
	}
	for _, tt := range tests {
		got := generateUpgradeManageCMD(tt.kubeVersion)
		assert.Equal(t, tt.want, got)
	}
}

func TestCalculateTargetVersion(t *testing.T) {
	// Test case 1: Provisioned version and desired version are the same
	provisionedVersion := "v1.19.0"
	desiredVersion := "v1.19.0"
	midVersion := "v1.20.0"
	expectedVersion := "v1.19.0"
	result := calculateTargetVersion(provisionedVersion, desiredVersion, midVersion)
	assert.Equal(t, expectedVersion, result)

	// Test case 2:  Provisioned version and desired version have a minor version difference of 1
	provisionedVersion = "v1.20.0"
	desiredVersion = "v1.21.2"
	midVersion = "v1.21.0"
	expectedVersion = "v1.21.2"
	result = calculateTargetVersion(provisionedVersion, desiredVersion, midVersion)
	assert.Equal(t, expectedVersion, result)

	// Test case 3: Provisioned version and desired version have a minor version difference of more than 1
	provisionedVersion = "v1.19.0"
	desiredVersion = "v1.21.0"
	midVersion = "v1.20.0"
	expectedVersion = "v1.20.0"
	result = calculateTargetVersion(provisionedVersion, desiredVersion, midVersion)
	assert.Equal(t, expectedVersion, result)
	provisionedVersion = "v1.23.0"
	desiredVersion = "v1.24.6"
	midVersion = "v1.23.0"
	expectedVersion = "v1.24.6"
	result = calculateTargetVersion(provisionedVersion, desiredVersion, midVersion)
	assert.Equal(t, expectedVersion, result)
}
