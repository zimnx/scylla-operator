// Code generated by informer-gen. DO NOT EDIT.

package externalversions

import (
	"fmt"

	v1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	v1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	v2alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v2alpha1"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	cache "k8s.io/client-go/tools/cache"
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
	// Group=scylla.scylladb.com, Version=v1
	case v1.SchemeGroupVersion.WithResource("scyllaclusters"):
		return &genericInformer{resource: resource.GroupResource(), informer: f.Scylla().V1().ScyllaClusters().Informer()}, nil

		// Group=scylla.scylladb.com, Version=v1alpha1
	case v1alpha1.SchemeGroupVersion.WithResource("nodeconfigs"):
		return &genericInformer{resource: resource.GroupResource(), informer: f.Scylla().V1alpha1().NodeConfigs().Informer()}, nil
	case v1alpha1.SchemeGroupVersion.WithResource("remotekubeclusterconfigs"):
		return &genericInformer{resource: resource.GroupResource(), informer: f.Scylla().V1alpha1().RemoteKubeClusterConfigs().Informer()}, nil
	case v1alpha1.SchemeGroupVersion.WithResource("scylladatacenters"):
		return &genericInformer{resource: resource.GroupResource(), informer: f.Scylla().V1alpha1().ScyllaDatacenters().Informer()}, nil
	case v1alpha1.SchemeGroupVersion.WithResource("scyllaoperatorconfigs"):
		return &genericInformer{resource: resource.GroupResource(), informer: f.Scylla().V1alpha1().ScyllaOperatorConfigs().Informer()}, nil

		// Group=scylla.scylladb.com, Version=v2alpha1
	case v2alpha1.SchemeGroupVersion.WithResource("scyllaclusters"):
		return &genericInformer{resource: resource.GroupResource(), informer: f.Scylla().V2alpha1().ScyllaClusters().Informer()}, nil

	}

	return nil, fmt.Errorf("no informer found for %v", resource)
}
