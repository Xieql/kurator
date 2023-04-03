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
)

func TestFindScaleUpWorkerNodes(t *testing.T) {
	cases := []struct {
		name         string
		provisioned  []NodeInfo
		currentNodes []NodeInfo
		expected     []NodeInfo
	}{
		{
			name: "scale up one worker node",
			provisioned: []NodeInfo{
				workerNode1,
				workerNode2,
			},
			currentNodes: []NodeInfo{
				workerNode1,
			},
			expected: []NodeInfo{
				workerNode2,
			},
		},
		{
			name: "no need to scale up worker nodes",
			provisioned: []NodeInfo{
				workerNode1,
				workerNode2,
			},
			currentNodes: []NodeInfo{
				workerNode1,
				workerNode2,
			},
			expected: []NodeInfo{},
		},
		{
			name:        "no provisioned worker nodes",
			provisioned: []NodeInfo{},
			currentNodes: []NodeInfo{
				workerNode1,
				workerNode2,
			},
			expected: []NodeInfo{
				workerNode1,
				workerNode2,
			},
		},
		{
			name:        "provisioned nodes are nil",
			provisioned: nil,
			currentNodes: []NodeInfo{
				workerNode1,
				workerNode2,
			},
			expected: []NodeInfo{
				workerNode1,
				workerNode2,
			},
		},
		{
			name: "current nodes are nil",
			provisioned: []NodeInfo{
				workerNode1,
				workerNode2,
			},
			currentNodes: nil,
			expected:     []NodeInfo{},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			scaleUpNodes := findScaleUpWorkerNodes(tc.provisioned, tc.currentNodes)
			assert.Equal(t, tc.expected, scaleUpNodes)
		})
	}
}

func TestFindScaleDownWorkerNodes(t *testing.T) {
	cases := []struct {
		name             string
		provisionedNodes []NodeInfo
		curNodes         []NodeInfo
		expected         []NodeInfo
	}{
		{
			name:             "scale down one worker node",
			provisionedNodes: []NodeInfo{workerNode1, workerNode2},
			curNodes:         []NodeInfo{workerNode1, workerNode2, workerNode3},
			expected:         []NodeInfo{workerNode1},
		},
		{
			name:             "no worker nodes to scale down",
			provisionedNodes: []NodeInfo{workerNode1, workerNode2},
			curNodes:         []NodeInfo{workerNode1, workerNode2},
			expected:         []NodeInfo{},
		},
		{
			name:             "scale down one worker node with different provisioned and current nodes",
			provisionedNodes: []NodeInfo{workerNode1, workerNode2, workerNode3},
			curNodes:         []NodeInfo{workerNode1, workerNode2},
			expected:         []NodeInfo{workerNode3},
		},
		{
			name:             "no provisioned nodes to scale down",
			provisionedNodes: nil,
			curNodes:         []NodeInfo{workerNode1, workerNode2},
			expected:         []NodeInfo{},
		},
		{
			name:             "no current nodes to scale down",
			provisionedNodes: []NodeInfo{workerNode1, workerNode2},
			curNodes:         nil,
			expected:         []NodeInfo{workerNode1, workerNode2},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			scaleDoneNodes := findScaleDownWorkerNodes(tc.provisionedNodes, tc.curNodes)
			assert.Equal(t, tc.expected, scaleDoneNodes)
		})
	}
}

func TestGenerateScaleDownManageCMD(t *testing.T) {
	cases := []struct {
		name         string
		nodeToDelete []NodeInfo
		expected     customClusterManageCMD
	}{
		{
			name:         "No nodes to delete",
			nodeToDelete: nodeNeedDelete1,
			expected:     customClusterManageCMD(""),
		},
		{
			name:         "Single node to delete",
			nodeToDelete: nodeNeedDelete2,
			expected:     customClusterManageCMD("ansible-playbook -i inventory/cluster-hosts --private-key /root/.ssh/ssh-privatekey remove-node.yml -vvv -e skip_confirmation=yes --extra-vars \"node=node1\" "),
		},
		{
			name:         "Multiple nodes to delete",
			nodeToDelete: nodeNeedDelete3,
			expected:     customClusterManageCMD("ansible-playbook -i inventory/cluster-hosts --private-key /root/.ssh/ssh-privatekey remove-node.yml -vvv -e skip_confirmation=yes --extra-vars \"node=node1,node2,node3\" "),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			scaleDownCMD := generateScaleDownManageCMD(tc.nodeToDelete)
			assert.Equal(t, tc.expected, scaleDownCMD)
		})
	}
}

func TestGetScaleUpConfigMapData(t *testing.T) {
	cases := []struct {
		name     string
		dataStr  string
		curNodes []NodeInfo
		expected string
	}{
		{
			name:     "test case 1",
			dataStr:  clusterHostDataStr1,
			curNodes: curNodes1,
			expected: clusterHostDataStr1,
		},
		{
			name:     "test case 2",
			dataStr:  clusterHostDataStr2,
			curNodes: curNodes2,
			expected: clusterHostDataStr2,
		},
		{
			name:     "test case 3",
			dataStr:  clusterHostDataStr3,
			curNodes: curNodes3,
			expected: clusterHostDataStr3,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ans := getScaleUpConfigMapData(tc.dataStr, tc.curNodes)
			assert.Equal(t, tc.expected, ans)
		})
	}
}
