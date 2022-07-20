package informers

import (
	"time"

	"github.com/scylladb/scylla-operator/pkg/remotedynamicclient/client"
	"github.com/scylladb/scylla-operator/pkg/remotedynamicclient/informers/internalinterfaces"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/cache"
)

// GenericRemoteInformer provides access to a shared generic informer and lister.
type GenericRemoteInformer interface {
	Informer() internalinterfaces.UnboundInformer
	Lister() GenericRemoteLister
}

type genericRemoteInformer struct {
	factory          internalinterfaces.SharedInformerFactory
	tweakListOptions internalinterfaces.TweakListOptionsFunc
	namespace        string
	gvr              schema.GroupVersionResource
	gvk              schema.GroupVersionKind
}

func (i *genericRemoteInformer) Informer() internalinterfaces.UnboundInformer {
	return i.factory.UnboundInformerFor(i.gvr, func() internalinterfaces.UnboundInformer {
		return &genericUnboundInformer{
			factory:          i.factory,
			tweakListOptions: i.tweakListOptions,
			gvr:              i.gvr,
			gvk:              i.gvk,
		}
	})
}

type genericUnboundInformer struct {
	factory          internalinterfaces.SharedInformerFactory
	tweakListOptions internalinterfaces.TweakListOptionsFunc
	namespace        string
	gvr              schema.GroupVersionResource
	gvk              schema.GroupVersionKind
}

func (f *genericUnboundInformer) defaultInformer(client client.RemoteInterface, region string, resyncPeriod time.Duration, eventHandlers []cache.ResourceEventHandler) cache.SharedIndexInformer {
	informer := NewFilteredInformer(client, region, f.namespace, f.gvr, f.gvk, resyncPeriod, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc}, f.tweakListOptions)
	for _, eventHandler := range eventHandlers {
		informer.AddEventHandler(eventHandler)
	}
	return informer
}

func (f *genericUnboundInformer) Region(datacenter string) cache.SharedIndexInformer {
	return f.factory.InformerFor(f.gvr, datacenter, f.defaultInformer)
}

func (f *genericUnboundInformer) HasSynced() bool {
	return f.factory.HasSyncedFor(f.gvr)
}

func (f *genericUnboundInformer) AddEventHandler(handler cache.ResourceEventHandler) {
	f.factory.AddEventHandlerFor(f.gvr, handler)
}

func (f *genericRemoteInformer) Lister() GenericRemoteLister {
	return NewGenericRemoteLister(f.gvr, f.gvk, f.Informer().Region)
}

func (f *sharedInformerFactory) ForResource(gvr schema.GroupVersionResource, gvk schema.GroupVersionKind) GenericRemoteInformer {
	return &genericRemoteInformer{
		factory:          f,
		tweakListOptions: f.tweakListOptions,
		namespace:        f.namespace,
		gvr:              gvr,
		gvk:              gvk,
	}
}
