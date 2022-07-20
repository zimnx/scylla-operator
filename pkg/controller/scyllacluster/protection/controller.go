// Copyright (c) 2022 ScyllaDB.

package protection

import (
	"context"
	"fmt"
	"sync"
	"time"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	scyllav2alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v2alpha1"
	scyllav2alpha1client "github.com/scylladb/scylla-operator/pkg/client/scylla/clientset/versioned/typed/scylla/v2alpha1"
	scyllav2alpha1informers "github.com/scylladb/scylla-operator/pkg/client/scylla/informers/externalversions/scylla/v2alpha1"
	scyllav2alpha1listers "github.com/scylladb/scylla-operator/pkg/client/scylla/listers/scylla/v2alpha1"
	"github.com/scylladb/scylla-operator/pkg/naming"
	multiregiondynamicclient "github.com/scylladb/scylla-operator/pkg/remotedynamicclient/client"
	multiregiondynamicinformers "github.com/scylladb/scylla-operator/pkg/remotedynamicclient/informers"
	"github.com/scylladb/scylla-operator/pkg/resource"
	"github.com/scylladb/scylla-operator/pkg/scheme"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/component-base/metrics/prometheus/ratelimiter"
	"k8s.io/klog/v2"
)

const (
	ControllerName = "ScyllaClusterProtectionController"
	// maxSyncDuration enforces preemption. Do not raise the value! Controllers shouldn't actively wait,
	// but rather use the queue.
	maxSyncDuration = 1 * time.Minute
)

var (
	keyFunc = cache.DeletionHandlingMetaNamespaceKeyFunc
)

type Controller struct {
	kubeClient   kubernetes.Interface
	scyllaClient scyllav2alpha1client.ScyllaV2alpha1Interface
	scyllaLister scyllav2alpha1listers.ScyllaClusterLister

	remoteDynamicClient          multiregiondynamicclient.RemoteInterface
	remoteScyllaDatacenterLister multiregiondynamicinformers.GenericRemoteLister

	cachesToSync []cache.InformerSynced

	eventRecorder record.EventRecorder

	queue workqueue.RateLimitingInterface
}

func NewController(
	kubeClient kubernetes.Interface,
	scyllaClient scyllav2alpha1client.ScyllaV2alpha1Interface,
	localScyllaClusterInformer scyllav2alpha1informers.ScyllaClusterInformer,
	remoteDynamicClient multiregiondynamicclient.RemoteInterface,
	remoteScyllaDatacenterInformer multiregiondynamicinformers.GenericRemoteInformer,
) (*Controller, error) {
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartStructuredLogging(0)
	eventBroadcaster.StartRecordingToSink(&corev1client.EventSinkImpl{Interface: kubeClient.CoreV1().Events("")})

	if kubeClient.CoreV1().RESTClient().GetRateLimiter() != nil {
		err := ratelimiter.RegisterMetricAndTrackRateLimiterUsage(
			"scyllaclusterprotection_controller",
			kubeClient.CoreV1().RESTClient().GetRateLimiter(),
		)
		if err != nil {
			return nil, err
		}
	}

	scpc := &Controller{
		kubeClient:   kubeClient,
		scyllaClient: scyllaClient,

		scyllaLister: localScyllaClusterInformer.Lister(),

		remoteDynamicClient:          remoteDynamicClient,
		remoteScyllaDatacenterLister: remoteScyllaDatacenterInformer.Lister(),

		cachesToSync: []cache.InformerSynced{
			localScyllaClusterInformer.Informer().HasSynced,
			remoteScyllaDatacenterInformer.Informer().HasSynced,
		},

		eventRecorder: eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: "scyllaclusterprotection-controller"}),

		queue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "scyllaclusterprotection"),
	}

	localScyllaClusterInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    scpc.addScyllaCluster,
		UpdateFunc: scpc.updateScyllaCluster,
		DeleteFunc: scpc.deleteScyllaCluster,
	})

	remoteScyllaDatacenterInformer.Informer().AddEventHandler(
		multiregiondynamicinformers.ResourceConvertingEventHandler(
			scheme.Scheme,
			&scyllav1alpha1.ScyllaDatacenter{},
			cache.ResourceEventHandlerFuncs{
				AddFunc:    scpc.addRemoteScyllaDatacenter,
				UpdateFunc: scpc.updateRemoteScyllaDatacenter,
				DeleteFunc: scpc.deleteRemoteScyllaDatacenter,
			}))

	return scpc, nil
}

func (scpc *Controller) processNextItem(ctx context.Context) bool {
	key, quit := scpc.queue.Get()
	if quit {
		return false
	}
	defer scpc.queue.Done(key)

	ctx, cancel := context.WithTimeout(ctx, maxSyncDuration)
	defer cancel()
	err := scpc.sync(ctx, key.(string))
	// TODO: Do smarter filtering then just Reduce to handle cases like 2 conflict errors.
	err = utilerrors.Reduce(err)
	switch {
	case err == nil:
		scpc.queue.Forget(key)
		return true

	case apierrors.IsConflict(err):
		klog.V(2).InfoS("Hit conflict, will retry in a bit", "Key", key, "Error", err)

	case apierrors.IsAlreadyExists(err):
		klog.V(2).InfoS("Hit already exists, will retry in a bit", "Key", key, "Error", err)

	default:
		utilruntime.HandleError(fmt.Errorf("syncing key '%v' failed: %v", key, err))
	}

	scpc.queue.AddRateLimited(key)

	return true
}

func (scpc *Controller) runWorker(ctx context.Context) {
	for scpc.processNextItem(ctx) {
	}
}

func (scpc *Controller) Run(ctx context.Context, workers int) {
	defer utilruntime.HandleCrash()

	klog.InfoS("Starting controller", "controller", ControllerName)

	var wg sync.WaitGroup
	defer func() {
		klog.InfoS("Shutting down controller", "controller", ControllerName)
		scpc.queue.ShutDown()
		wg.Wait()
		klog.InfoS("Shut down controller", "controller", ControllerName)
	}()

	if !cache.WaitForNamedCacheSync(ControllerName, ctx.Done(), scpc.cachesToSync...) {
		return
	}

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			wait.UntilWithContext(ctx, scpc.runWorker, time.Second)
		}()
	}

	<-ctx.Done()
}

func (scpc *Controller) enqueue(sc *scyllav2alpha1.ScyllaCluster) {
	key, err := keyFunc(sc)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("couldn't get key for object %#v: %v", sc, err))
		return
	}

	klog.V(4).InfoS("Enqueuing", "ScyllaCluster", klog.KObj(sc))
	scpc.queue.Add(key)
}

func (scpc *Controller) addScyllaCluster(obj interface{}) {
	sc := obj.(*scyllav2alpha1.ScyllaCluster)
	klog.V(4).InfoS("Observed addition of ScyllaCluster", "ScyllaCluster", klog.KObj(sc))
	scpc.enqueue(sc)
}

func (scpc *Controller) updateScyllaCluster(old, cur interface{}) {
	oldSC := old.(*scyllav2alpha1.ScyllaCluster)
	currentSC := cur.(*scyllav2alpha1.ScyllaCluster)

	if currentSC.UID != oldSC.UID {
		key, err := keyFunc(oldSC)
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("couldn't get key for object %#v: %v", oldSC, err))
			return
		}
		scpc.deleteScyllaCluster(cache.DeletedFinalStateUnknown{
			Key: key,
			Obj: oldSC,
		})
	}

	klog.V(4).InfoS("Observed update of ScyllaCluster", "ScyllaCluster", klog.KObj(oldSC))
	scpc.enqueue(currentSC)
}

func (scpc *Controller) deleteScyllaCluster(obj interface{}) {
	sc, ok := obj.(*scyllav2alpha1.ScyllaCluster)
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("couldn't get object from tombstone %#v", obj))
			return
		}
		sc, ok = tombstone.Obj.(*scyllav2alpha1.ScyllaCluster)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("tombstone contained object that is not a ScyllaCluster %#v", obj))
			return
		}
	}
	klog.V(4).InfoS("Observed deletion of ScyllaCluster", "ScyllaCluster", klog.KObj(sc))
	scpc.enqueue(sc)
}

func (scpc *Controller) addRemoteScyllaDatacenter(obj interface{}) {
	sd := obj.(*scyllav1alpha1.ScyllaDatacenter)
	klog.V(4).InfoS("Observed addition of remote ScyllaDatacenter", "ScyllaDatacenter", klog.KObj(sd))
	scpc.enqueueParent(sd)
}

func (scpc *Controller) updateRemoteScyllaDatacenter(old, cur interface{}) {
	oldSD := old.(*scyllav1alpha1.ScyllaDatacenter)
	currentSD := cur.(*scyllav1alpha1.ScyllaDatacenter)

	if currentSD.UID != oldSD.UID {
		key, err := keyFunc(oldSD)
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("couldn't get key for object %#v: %v", oldSD, err))
			return
		}
		scpc.deleteRemoteScyllaDatacenter(cache.DeletedFinalStateUnknown{
			Key: key,
			Obj: oldSD,
		})
	}

	klog.V(4).InfoS("Observed update of remote ScyllaDatacenter", "ScyllaDatacenter", klog.KObj(oldSD))
	scpc.enqueueParent(currentSD)
}

func (scpc *Controller) deleteRemoteScyllaDatacenter(obj interface{}) {
	sd, ok := obj.(*scyllav1alpha1.ScyllaDatacenter)
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("couldn't get object from tombstone %#v", obj))
			return
		}
		sd, ok = tombstone.Obj.(*scyllav1alpha1.ScyllaDatacenter)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("tombstone contained object that is not a ScyllaDatacenter %#v", obj))
			return
		}
	}
	klog.V(4).InfoS("Observed deletion of ScyllaDatacenter", "ScyllaDatacenter", klog.KObj(sd))
	scpc.enqueueParent(sd)
}

func (scpc *Controller) enqueueParent(obj metav1.Object) {
	labels := obj.GetLabels()
	name, namespace := labels[naming.ParentClusterNameLabel], labels[naming.ParentClusterNamespaceLabel]
	sc, err := scpc.scyllaLister.ScyllaClusters(namespace).Get(name)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("couldn't find parent ScyllaCluster for object %#v", obj))
		return
	}

	gvk, err := resource.GetObjectGVK(obj.(runtime.Object))
	if err != nil {
		utilruntime.HandleError(err)
		return
	}

	klog.V(4).InfoS("Enqueuing parent", gvk.Kind, klog.KObj(obj), "ScyllaCluster", klog.KObj(sc))
	scpc.enqueue(sc)
}
