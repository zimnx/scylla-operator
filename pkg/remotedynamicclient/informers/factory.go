// Copyright (c) 2022 ScyllaDB.

package informers

import (
	"context"
	"crypto/sha512"
	"sync"
	"time"

	"github.com/scylladb/scylla-operator/pkg/remotedynamicclient/client"
	"github.com/scylladb/scylla-operator/pkg/remotedynamicclient/informers/internalinterfaces"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic/dynamiclister"
	"k8s.io/client-go/tools/cache"
)

// SharedInformerOption defines the functional option type for SharedInformerFactory.
type SharedInformerOption func(*sharedInformerFactory) *sharedInformerFactory

type sharedInformerFactory struct {
	scheme           runtime.Scheme
	client           client.RemoteInterface
	namespace        string
	tweakListOptions internalinterfaces.TweakListOptionsFunc
	lock             sync.Mutex
	defaultResync    time.Duration
	customResync     map[schema.GroupVersionResource]time.Duration

	informers         map[schema.GroupVersionResource]map[string]cache.SharedIndexInformer
	startedInformers  map[schema.GroupVersionResource]map[string]chan struct{}
	unboundInformers  map[schema.GroupVersionResource]internalinterfaces.UnboundInformer
	eventHandlers     map[schema.GroupVersionResource][]cache.ResourceEventHandler
	credentialsHashes map[string]string
	stopCh            <-chan struct{}
}

// WithCustomResyncConfig sets a custom resync period for the specified informer types.
func WithCustomResyncConfig(resyncConfig map[schema.GroupVersionResource]time.Duration) SharedInformerOption {
	return func(factory *sharedInformerFactory) *sharedInformerFactory {
		for k, v := range resyncConfig {
			factory.customResync[k] = v
		}
		return factory
	}
}

// WithTweakListOptions sets a custom filter on all listers of the configured SharedInformerFactory.
func WithTweakListOptions(tweakListOptions internalinterfaces.TweakListOptionsFunc) SharedInformerOption {
	return func(factory *sharedInformerFactory) *sharedInformerFactory {
		factory.tweakListOptions = tweakListOptions
		return factory
	}
}

// WithNamespace limits the SharedInformerFactory to the specified namespace.
func WithNamespace(namespace string) SharedInformerOption {
	return func(factory *sharedInformerFactory) *sharedInformerFactory {
		factory.namespace = namespace
		return factory
	}
}

// NewSharedInformerFactory constructs a new instance of sharedInformerFactory for all namespaces.
func NewSharedInformerFactory(scheme runtime.Scheme, client client.RemoteInterface, defaultResync time.Duration) SharedInformerFactory {
	return NewSharedInformerFactoryWithOptions(scheme, client, defaultResync)
}

// NewFilteredSharedInformerFactory constructs a new instance of sharedInformerFactory.
// Listers obtained via this SharedInformerFactory will be subject to the same filters
// as specified here.
// Deprecated: Please use NewSharedInformerFactoryWithOptions instead
func NewFilteredSharedInformerFactory(scheme runtime.Scheme, client client.RemoteInterface, defaultResync time.Duration, namespace string, tweakListOptions internalinterfaces.TweakListOptionsFunc) SharedInformerFactory {
	return NewSharedInformerFactoryWithOptions(scheme, client, defaultResync, WithNamespace(namespace), WithTweakListOptions(tweakListOptions))
}

// NewSharedInformerFactoryWithOptions constructs a new instance of a SharedInformerFactory with additional options.
func NewSharedInformerFactoryWithOptions(scheme runtime.Scheme, client client.RemoteInterface, defaultResync time.Duration, options ...SharedInformerOption) SharedInformerFactory {
	factory := &sharedInformerFactory{
		scheme:           scheme,
		client:           client,
		namespace:        v1.NamespaceAll,
		defaultResync:    defaultResync,
		informers:        make(map[schema.GroupVersionResource]map[string]cache.SharedIndexInformer),
		startedInformers: make(map[schema.GroupVersionResource]map[string]chan struct{}),
		unboundInformers: make(map[schema.GroupVersionResource]internalinterfaces.UnboundInformer),
		eventHandlers:    make(map[schema.GroupVersionResource][]cache.ResourceEventHandler),
		customResync:     make(map[schema.GroupVersionResource]time.Duration),
	}

	// Apply all options
	for _, opt := range options {
		factory = opt(factory)
	}

	return factory
}

// Start initializes all requested informers.
func (f *sharedInformerFactory) Start(stopCh <-chan struct{}) {
	f.lock.Lock()
	defer f.lock.Unlock()

	if f.stopCh == nil {
		f.stopCh = stopCh
	}
	for gvr, informers := range f.informers {
		for dc, informer := range informers {
			if _, started := f.startedInformers[gvr][dc]; !started {
				sc := make(chan struct{})
				go func() {
					<-stopCh
					close(sc)
				}()

				go informer.Run(sc)
				if _, ok := f.startedInformers[gvr]; !ok {
					f.startedInformers[gvr] = map[string]chan struct{}{}
				}
				f.startedInformers[gvr][dc] = sc
			}
		}
	}
}

// InternalInformerFor returns the SharedIndexInformer for obj using an internal
// client.
func (f *sharedInformerFactory) InformerFor(gvr schema.GroupVersionResource, datacenter string, newFunc internalinterfaces.NewInformerFunc) cache.SharedIndexInformer {
	f.lock.Lock()
	defer f.lock.Unlock()
	datacenterInformers, registered := f.informers[gvr]
	if registered {
		informer, exists := datacenterInformers[datacenter]
		if exists {
			return informer
		}
	}

	resyncPeriod, exists := f.customResync[gvr]
	if !exists {
		resyncPeriod = f.defaultResync
	}

	informer := newFunc(f.client, datacenter, resyncPeriod, f.eventHandlers[gvr])
	if _, ok := f.informers[gvr]; !ok {
		f.informers[gvr] = map[string]cache.SharedIndexInformer{}
	}
	f.informers[gvr][datacenter] = informer

	if f.stopCh != nil {
		sc := make(chan struct{})
		go func() {
			<-f.stopCh
			select {
			case <-sc:
			default:
				close(sc)
			}
		}()
		go informer.Run(sc)

		if _, ok := f.startedInformers[gvr]; !ok {
			f.startedInformers[gvr] = map[string]chan struct{}{}
		}
		f.startedInformers[gvr][datacenter] = sc
	}

	return informer
}

func (f *sharedInformerFactory) UnboundInformerFor(gvr schema.GroupVersionResource, newFunc internalinterfaces.NewUnboundInformerFunc) internalinterfaces.UnboundInformer {
	f.lock.Lock()
	defer f.lock.Unlock()

	informer, exists := f.unboundInformers[gvr]
	if exists {
		return informer
	}

	informer = newFunc()
	f.unboundInformers[gvr] = informer

	return informer
}

func (f *sharedInformerFactory) AddEventHandlerFor(gvr schema.GroupVersionResource, handler cache.ResourceEventHandler) {
	f.lock.Lock()
	defer f.lock.Unlock()

	f.eventHandlers[gvr] = append(f.eventHandlers[gvr], handler)
}

func (f *sharedInformerFactory) HasSyncedFor(gvr schema.GroupVersionResource) bool {
	f.lock.Lock()
	defer f.lock.Unlock()

	for _, informer := range f.informers[gvr] {
		if !informer.HasSynced() {
			return false
		}
	}
	return true
}

// SharedInformerFactory provides shared informers for resources in all known
// API group versions.
type SharedInformerFactory interface {
	internalinterfaces.SharedInformerFactory

	ForResource(gvr schema.GroupVersionResource, gvk schema.GroupVersionKind) GenericRemoteInformer
}

type DynamicRegion interface {
	Update(key string, config []byte) error
	Delete(key string)
}

type DynamicSharedInformerFactory interface {
	SharedInformerFactory
	DynamicRegion
}

func (f *sharedInformerFactory) Update(region string, config []byte) error {
	f.lock.Lock()
	defer f.lock.Unlock()

	hash := string(sha512.New().Sum(config))
	if credentialsHash, found := f.credentialsHashes[region]; found && credentialsHash == hash {
		return nil
	}

	f.stopRegionInformer(region)

	// TODO: start already created ones

	return nil
}

func (f *sharedInformerFactory) stopRegionInformer(region string) {
	for informerType, regionInformers := range f.startedInformers {
		if stopCh, started := regionInformers[region]; started {
			select {
			case <-stopCh:
			default:
				close(stopCh)
			}
		}
		delete(f.startedInformers[informerType], region)
		delete(f.informers[informerType], region)
	}

	delete(f.credentialsHashes, region)
}

func (f *sharedInformerFactory) Delete(key string) error {
	f.lock.Lock()
	defer f.lock.Unlock()

	f.stopRegionInformer(key)

	return nil
}

// NewFilteredInformer constructs a new informer for decorated Unstructured type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewFilteredInformer(client client.RemoteInterface, region string, namespace string, gvr schema.GroupVersionResource, gvk schema.GroupVersionKind, resyncPeriod time.Duration, indexers cache.Indexers, tweakListOptions internalinterfaces.TweakListOptionsFunc) cache.SharedIndexInformer {
	return cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				c, err := client.Region(region)
				if err != nil {
					return nil, err
				}
				return c.Resource(gvr).Namespace(namespace).List(context.TODO(), options)
			},
			WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				c, err := client.Region(region)
				if err != nil {
					return nil, err
				}
				return c.Resource(gvr).Namespace(namespace).Watch(context.TODO(), options)
			},
		},
		&unstructured.Unstructured{Object: map[string]interface{}{
			"apiVersion": gvk.GroupVersion().String(),
			"kind":       gvk.Kind,
		}},
		resyncPeriod,
		indexers,
	)
}

type GenericRemoteLister interface {
	Region(string) cache.GenericLister
}

type genericRemoteLister struct {
	gvr          schema.GroupVersionResource
	gvk          schema.GroupVersionKind
	makeInformer func(string) cache.SharedIndexInformer
}

func (s *genericRemoteLister) Region(region string) cache.GenericLister {
	return dynamiclister.NewRuntimeObjectShim(dynamiclister.New(s.makeInformer(region).GetIndexer(), s.gvr))
}

func NewGenericRemoteLister(gvr schema.GroupVersionResource, gvk schema.GroupVersionKind, f func(string) cache.SharedIndexInformer) *genericRemoteLister {
	return &genericRemoteLister{gvr: gvr, gvk: gvk, makeInformer: f}
}
