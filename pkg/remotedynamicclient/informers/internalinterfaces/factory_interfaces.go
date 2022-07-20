package internalinterfaces

import (
	"time"

	"github.com/scylladb/scylla-operator/pkg/remotedynamicclient/client"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/cache"
)

type UnboundInformer interface {
	HasSynced() bool
	AddEventHandler(handler cache.ResourceEventHandler)

	Region(string) cache.SharedIndexInformer
}

// NewInformerFunc takes versioned.Interface and time.Duration to return a SharedIndexInformer.
type NewInformerFunc func(client.RemoteInterface, string, time.Duration, []cache.ResourceEventHandler) cache.SharedIndexInformer

// NewUnboundInformerFunc takes versioned.Interface and time.Duration to return a SharedIndexInformer.
type NewUnboundInformerFunc func() UnboundInformer

// SharedInformerFactory a small interface to allow for adding an informer without an import cycle
type SharedInformerFactory interface {
	Start(stopCh <-chan struct{})
	InformerFor(gvr schema.GroupVersionResource, region string, newFunc NewInformerFunc) cache.SharedIndexInformer
	UnboundInformerFor(gvr schema.GroupVersionResource, newFunc NewUnboundInformerFunc) UnboundInformer
	AddEventHandlerFor(gvr schema.GroupVersionResource, handler cache.ResourceEventHandler)
	HasSyncedFor(gvr schema.GroupVersionResource) bool
}

// TweakListOptionsFunc is a function that transforms a v1.ListOptions.
type TweakListOptionsFunc func(*v1.ListOptions)
