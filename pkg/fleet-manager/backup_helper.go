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

package fleet

import (
	backupapi "kurator.dev/kurator/pkg/apis/backups/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
)

// getDestinationClusters get Destination Clusters from backup.destination. This func will return union set of the cluster from fleet and the direct clusters
func (b *BackupManager) getDestinationClusters(backup *backupapi.Backup) []ctrl.Request {

	if backup.Spec.Destination == nil {

	}

	if backup.Spec.Destination len(backup.Spec.Destination.Fleet) == 0 {

	}

	return nil
}
