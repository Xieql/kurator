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
