package controllerhelpers

import (
	"fmt"

	"github.com/scylladb/scylla-operator/pkg/kubeinterfaces"
	"github.com/scylladb/scylla-operator/pkg/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
)

type InformerHandler struct {
	Informer cache.SharedIndexInformer
	Handler  cache.ResourceEventHandler
}

type KeyFuncType func(obj interface{}) (string, error)

type handlerOperationType string

const (
	handlerOperationTypeAdd    = "HandleAdd"
	handlerOperationTypeUpdate = "Update"
	handlerOperationTypeDelete = "Delete"
)

func getObjectLogContext(cur, old kubeinterfaces.ObjectInterface) []any {
	res := []any{"GVK", resource.GetObjectGVKOrUnknown(cur), "Ref", klog.KObj(cur), "UID", cur.GetUID()}

	if old != nil {
		res = append(res, "OldUID", old.GetUID())
	}

	return res
}

type GetFuncType[T any] func(namespace, name string) (T, error)
type enqueueFuncType func(kubeinterfaces.ObjectInterface, handlerOperationType)
type deleteFuncType = func(any)

// func Enqueue[T ObjectInterface](obj T,queue   workqueue.RateLimitingInterface, keyFunc KeyFuncType, op handlerOperationType) {
// 	EnqueueDepth[T](2, obj,queue, keyFunc, op)
// }
//
// func EnqueueDepth[T ObjectInterface](depth int, obj T, queue   workqueue.RateLimitingInterface, keyFunc KeyFuncType, op handlerOperationType) {
// 	key, err := keyFunc(obj)
// 	if err != nil {
// 		utilruntime.HandleError(fmt.Errorf("couldn't get key for object %#v: %v", obj, err))
// 		return
// 	}
//
// 	// klog.V(4).InfoSDepth(depth+1, "Enqueuing object", []any{"Operation", op, getObjectLogContext(obj, nil)...}...)
// 	klog.V(4).InfoSDepth(depth+1, "Enqueuing object", getObjectLogContext(obj, nil)...)
// 	queue.Add(key)
// }
//
// func EnqueueOwner[T ObjectInterface](obj T,queue   workqueue.RateLimitingInterface, keyFunc KeyFuncType, op handlerOperationType) {
// 	EnqueueOwnerDepth[T](2, obj,queue, keyFunc, op)
// }
//
// func EnqueueOwnerDepth[T ObjectInterface](depth int, obj T, controller queue   workqueue.RateLimitingInterface, keyFunc KeyFuncType, op handlerOperationType) {
// 	controllerRef := metav1.GetControllerOf(obj)
// 	if controllerRef == nil {
// 		return
// 	}
//
// 	if controllerRef.Kind != q.gvk.Kind {
// 		return
// 	}
//
// 	owner, err := q.getFunc(obj.GetNamespace(), controllerRef.Name)
// 	if err != nil {
// 		utilruntime.HandleError(err)
// 		return
// 	}
//
// 	if owner.GetUID() != controllerRef.UID {
// 		utilruntime.HandleError(err)
// 		return
// 	}
//
// 	// klog.V(4).InfoSDepth(depth, "Enqueuing owner", "OwnerGVK", q.gvk, "OwnerRef", klog.KObj(owner), "OwnerUID", owner.GetUID(), getObjectLogContext(obj, nil)...)
// 	klog.V(4).InfoSDepth(depth, "Enqueuing owner", "OwnerGVK", q.gvk, "OwnerRef", klog.KObj(owner), "OwnerUID", owner.GetUID(), "TODO")
// 	q.enqueue(depth+1, owner, operation)
// }
//
// func HandleAdd[T ObjectInterface](, obj any, enqueueFunc enqueueFuncType) {
// 	klog.V(5).InfoSDepth(depth, "Observed addition", getObjectLogContext(obj.(ObjectInterface), nil)...)
//
// 	enqueueFunc(obj.(ObjectInterface), handlerOperationTypeAdd)
// }
//
// func (h *Handlers[QT]) updateHandler(depth int, oldUntyped, curUntyped any, enqueueFunc enqueueFuncType, deleteFunc deleteFuncType) {
// 	old := oldUntyped.(ObjectInterface)
// 	cur := curUntyped.(ObjectInterface)
//
// 	klog.V(5).InfoSDepth(depth, "Observed update", getObjectLogContext(cur, old)...)
//
// 	if cur.GetUID() != old.GetUID() {
// 		key, err := h.keyFunc(old)
// 		if err != nil {
// 			utilruntime.HandleError(fmt.Errorf("couldn't get key for object %#v: %v", old, err))
// 			return
// 		}
//
// 		deleteFunc(cache.DeletedFinalStateUnknown{
// 			Key: key,
// 			Obj: old,
// 		})
// 	}
//
// 	enqueueFunc(cur, handlerOperationTypeUpdate)
// }
//
// func (h *Handlers[QT]) deleteHandler(depth int, obj any, enqueueFunc enqueueFuncType) {
// 	tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
// 	if ok {
// 		klog.V(5).InfoSDepth(depth, "Observed deletion", getObjectLogContext(tombstone.Obj.(ObjectInterface), nil)...)
// 		enqueueFunc(tombstone.Obj.(ObjectInterface), handlerOperationTypeDelete)
// 		return
// 	}
//
// 	klog.V(5).InfoSDepth(depth, "Observed deletion", getObjectLogContext(obj.(ObjectInterface), nil)...)
//
// 	enqueueFunc(obj.(ObjectInterface), handlerOperationTypeDelete)
// }

type Handlers[T kubeinterfaces.ObjectInterface] struct {
	queue   workqueue.RateLimitingInterface
	keyFunc KeyFuncType
	scheme  *runtime.Scheme
	gvk     schema.GroupVersionKind
	getFunc GetFuncType[T]
}

func NewHandlers[T kubeinterfaces.ObjectInterface](queue workqueue.RateLimitingInterface, keyFunc KeyFuncType, scheme *runtime.Scheme, gvk schema.GroupVersionKind, getFunc GetFuncType[T]) (*Handlers[T], error) {
	return &Handlers[T]{
		queue:   queue,
		keyFunc: keyFunc,
		scheme:  scheme,
		gvk:     gvk,
		getFunc: getFunc,
	}, nil
}

func (h *Handlers[T]) EnqueueWithDepth(depth int, untypedObj kubeinterfaces.ObjectInterface, op handlerOperationType) {
	obj := untypedObj.(T)

	key, err := h.keyFunc(obj)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("couldn't get key for object %#v: %v", obj, err))
		return
	}

	// klog.V(4).InfoSDepth(depth, "Enqueuing object", []any{"Operation", op, getObjectLogContext(obj, nil)...}...)
	klog.V(4).InfoSDepth(depth, "Enqueuing object", getObjectLogContext(obj, nil)...)
	h.queue.Add(key)
}

func (h *Handlers[T]) Enqueue(obj kubeinterfaces.ObjectInterface, op handlerOperationType) {
	h.EnqueueWithDepth(2, obj, op)
}

func (h *Handlers[QT]) EnqueueOwnerWithDepth(depth int, obj kubeinterfaces.ObjectInterface, operation handlerOperationType) {
	controllerRef := metav1.GetControllerOf(obj)
	if controllerRef == nil {
		return
	}

	if controllerRef.Kind != h.gvk.Kind {
		return
	}

	owner, err := h.getFunc(obj.GetNamespace(), controllerRef.Name)
	if err != nil {
		utilruntime.HandleError(err)
		return
	}

	if owner.GetUID() != controllerRef.UID {
		utilruntime.HandleError(err)
		return
	}

	// klog.V(4).InfoSDepth(depth, "Enqueuing owner", "OwnerGVK", q.gvk, "OwnerRef", klog.KObj(owner), "OwnerUID", owner.GetUID(), getObjectLogContext(obj, nil)...)
	klog.V(4).InfoSDepth(depth, "Enqueuing owner", "OwnerGVK", h.gvk, "OwnerRef", klog.KObj(owner), "OwnerUID", owner.GetUID(), "TODO")
	h.EnqueueWithDepth(depth+1, owner, operation)
}

func (h *Handlers[T]) EnqueueOwner(obj kubeinterfaces.ObjectInterface, operation handlerOperationType) {
	h.EnqueueOwnerWithDepth(2, obj, operation)
}

func (h *Handlers[T]) HandleAdd(obj kubeinterfaces.ObjectInterface, enqueueFunc enqueueFuncType) {
	h.HandleAddWithDepth(1, obj, enqueueFunc)
}

func (h *Handlers[T]) HandleAddWithDepth(depth int, obj any, enqueueFunc enqueueFuncType) {
	klog.V(5).InfoSDepth(depth, "Observed addition", getObjectLogContext(obj.(kubeinterfaces.ObjectInterface), nil)...)

	enqueueFunc(obj.(kubeinterfaces.ObjectInterface), handlerOperationTypeAdd)
}

func (h *Handlers[QT]) HandleUpdateWithDepth(depth int, oldUntyped, curUntyped any, enqueueFunc enqueueFuncType, deleteFunc deleteFuncType) {
	old := oldUntyped.(kubeinterfaces.ObjectInterface)
	cur := curUntyped.(kubeinterfaces.ObjectInterface)

	klog.V(5).InfoSDepth(depth, "Observed update", getObjectLogContext(cur, old)...)

	if cur.GetUID() != old.GetUID() {
		key, err := h.keyFunc(old)
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("couldn't get key for object %#v: %v", old, err))
			return
		}

		deleteFunc(cache.DeletedFinalStateUnknown{
			Key: key,
			Obj: old,
		})
	}

	enqueueFunc(cur, handlerOperationTypeUpdate)
}

func (h *Handlers[QT]) HandleUpdate(old, cur any, enqueueFunc enqueueFuncType, deleteFunc deleteFuncType) {
	h.HandleUpdateWithDepth(2, old, cur, enqueueFunc, deleteFunc)
}

func (h *Handlers[T]) HandleDeleteWithDepth(depth int, obj any, enqueueFunc enqueueFuncType) {
	tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
	if ok {
		klog.V(5).InfoSDepth(depth, "Observed deletion", getObjectLogContext(tombstone.Obj.(kubeinterfaces.ObjectInterface), nil)...)
		enqueueFunc(tombstone.Obj.(kubeinterfaces.ObjectInterface), handlerOperationTypeDelete)
		return
	}

	klog.V(5).InfoSDepth(depth, "Observed deletion", getObjectLogContext(obj.(kubeinterfaces.ObjectInterface), nil)...)

	enqueueFunc(obj.(kubeinterfaces.ObjectInterface), handlerOperationTypeDelete)
}

func (h *Handlers[T]) HandleDelete(obj any, enqueueFunc enqueueFuncType) {
	h.HandleDeleteWithDepth(2, obj, enqueueFunc)
}
