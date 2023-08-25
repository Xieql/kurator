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

// Code generated by informer-gen. DO NOT EDIT.

package externalversions

import (
	"fmt"

	schema "k8s.io/apimachinery/pkg/runtime/schema"
	cache "k8s.io/client-go/tools/cache"
	v1alpha1 "kurator.dev/kurator/pkg/apis/apps/v1alpha1"
	backupsv1alpha1 "kurator.dev/kurator/pkg/apis/backups/v1alpha1"
	clusterv1alpha1 "kurator.dev/kurator/pkg/apis/cluster/v1alpha1"
	fleetv1alpha1 "kurator.dev/kurator/pkg/apis/fleet/v1alpha1"
	infrav1alpha1 "kurator.dev/kurator/pkg/apis/infra/v1alpha1"
)

// GenericInformer is type of SharedIndexInformer which will locate and delegate to other
// sharedInformers based on type
type GenericInformer interface {
	Informer() cache.SharedIndexInformer
	Lister() cache.GenericLister
}

type genericInformer struct {
	informer cache.SharedIndexInformer
	resource schema.GroupResource
}

// Informer returns the SharedIndexInformer.
func (f *genericInformer) Informer() cache.SharedIndexInformer {
	return f.informer
}

// Lister returns the GenericLister.
func (f *genericInformer) Lister() cache.GenericLister {
	return cache.NewGenericLister(f.Informer().GetIndexer(), f.resource)
}

// ForResource gives generic access to a shared informer of the matching type
// TODO extend this to unknown resources with a client pool
func (f *sharedInformerFactory) ForResource(resource schema.GroupVersionResource) (GenericInformer, error) {
	switch resource {
	// Group=apps.kurator.dev, Version=v1alpha1
	case v1alpha1.SchemeGroupVersion.WithResource("applications"):
		return &genericInformer{resource: resource.GroupResource(), informer: f.Apps().V1alpha1().Applications().Informer()}, nil

		// Group=backup.kurator.dev, Version=v1alpha1
	case backupsv1alpha1.SchemeGroupVersion.WithResource("backups"):
		return &genericInformer{resource: resource.GroupResource(), informer: f.Backup().V1alpha1().Backups().Informer()}, nil

		// Group=cluster.kurator.dev, Version=v1alpha1
	case clusterv1alpha1.SchemeGroupVersion.WithResource("attachedclusters"):
		return &genericInformer{resource: resource.GroupResource(), informer: f.Cluster().V1alpha1().AttachedClusters().Informer()}, nil
	case clusterv1alpha1.SchemeGroupVersion.WithResource("clusters"):
		return &genericInformer{resource: resource.GroupResource(), informer: f.Cluster().V1alpha1().Clusters().Informer()}, nil

		// Group=fleet.kurator.dev, Version=v1alpha1
	case fleetv1alpha1.SchemeGroupVersion.WithResource("fleets"):
		return &genericInformer{resource: resource.GroupResource(), informer: f.Fleet().V1alpha1().Fleets().Informer()}, nil

		// Group=infrastructure.cluster.x-k8s.io, Version=v1alpha1
	case infrav1alpha1.SchemeGroupVersion.WithResource("customclusters"):
		return &genericInformer{resource: resource.GroupResource(), informer: f.Infrastructure().V1alpha1().CustomClusters().Informer()}, nil
	case infrav1alpha1.SchemeGroupVersion.WithResource("custommachines"):
		return &genericInformer{resource: resource.GroupResource(), informer: f.Infrastructure().V1alpha1().CustomMachines().Informer()}, nil

	}

	return nil, fmt.Errorf("no informer found for %v", resource)
}
