// Copyright (C) 2021 ScyllaDB

package nodeconfigdaemon

import (
	"context"
	"fmt"
	"sync"
	"time"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	scyllaclient "github.com/scylladb/scylla-operator/pkg/client/scylla/clientset/versioned"
	scyllav1alpha1informers "github.com/scylladb/scylla-operator/pkg/client/scylla/informers/externalversions/scylla/v1alpha1"
	scyllav1alpha1listers "github.com/scylladb/scylla-operator/pkg/client/scylla/listers/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/cri"
	"github.com/scylladb/scylla-operator/pkg/scheme"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	appsv1informers "k8s.io/client-go/informers/apps/v1"
	batchv1informers "k8s.io/client-go/informers/batch/v1"
	corev1informers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	appsv1listers "k8s.io/client-go/listers/apps/v1"
	batchv1listers "k8s.io/client-go/listers/batch/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/component-base/metrics/prometheus/ratelimiter"
	"k8s.io/klog/v2"
	"k8s.io/utils/pointer"
)

const (
	ControllerName = "NodeConfigInstanceController"

	maxSyncDuration = 30 * time.Second
)

var (
	controllerKey = "key"
	keyFunc       = cache.DeletionHandlingMetaNamespaceKeyFunc

	controllerGVK          = scyllav1alpha1.GroupVersion.WithKind("NodeConfig")
	daemonSetControllerGVK = appsv1.SchemeGroupVersion.WithKind("DaemonSet")
)

type Controller struct {
	kubeClient   kubernetes.Interface
	scyllaClient scyllaclient.Interface

	criClient cri.Client

	nodeConfigLister          scyllav1alpha1listers.NodeConfigLister
	localScyllaPodsLister     corev1listers.PodLister
	configMapLister           corev1listers.ConfigMapLister
	namespacedDaemonSetLister appsv1listers.DaemonSetLister
	namespacedJobLister       batchv1listers.JobLister
	selfPodLister             corev1listers.PodLister

	namespace      string
	podName        string
	nodeName       string
	nodeConfigName string
	nodeConfigUID  types.UID
	scyllaImage    string

	cachesToSync []cache.InformerSynced

	eventRecorder record.EventRecorder

	queue workqueue.RateLimitingInterface
}

func NewController(
	kubeClient kubernetes.Interface,
	scyllaClient scyllaclient.Interface,
	criClient cri.Client,
	nodeConfigInformer scyllav1alpha1informers.NodeConfigInformer,
	localScyllaPodsInformer corev1informers.PodInformer,
	namespacedDaemonSetInformer appsv1informers.DaemonSetInformer,
	namespacedJobInformer batchv1informers.JobInformer,
	selfPodInformer corev1informers.PodInformer,
	namespace string,
	podName string,
	nodeName string,
	nodeConfigName string,
	nodeConfigUID types.UID,
	scyllaImage string,
) (*Controller, error) {
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartStructuredLogging(0)
	eventBroadcaster.StartRecordingToSink(&corev1client.EventSinkImpl{Interface: kubeClient.CoreV1().Events("")})

	if kubeClient.CoreV1().RESTClient().GetRateLimiter() != nil {
		err := ratelimiter.RegisterMetricAndTrackRateLimiterUsage(
			"nodeconfiginstance_controller",
			kubeClient.CoreV1().RESTClient().GetRateLimiter(),
		)
		if err != nil {
			return nil, err
		}
	}

	snc := &Controller{
		kubeClient:   kubeClient,
		scyllaClient: scyllaClient,
		criClient:    criClient,

		nodeConfigLister:          nodeConfigInformer.Lister(),
		localScyllaPodsLister:     localScyllaPodsInformer.Lister(),
		namespacedDaemonSetLister: namespacedDaemonSetInformer.Lister(),
		namespacedJobLister:       namespacedJobInformer.Lister(),
		selfPodLister:             selfPodInformer.Lister(),

		namespace:      namespace,
		podName:        podName,
		nodeName:       nodeName,
		nodeConfigName: nodeConfigName,
		nodeConfigUID:  nodeConfigUID,
		scyllaImage:    scyllaImage,

		cachesToSync: []cache.InformerSynced{
			nodeConfigInformer.Informer().HasSynced,
			localScyllaPodsInformer.Informer().HasSynced,
			namespacedDaemonSetInformer.Informer().HasSynced,
			namespacedJobInformer.Informer().HasSynced,
			selfPodInformer.Informer().HasSynced,
		},

		eventRecorder: eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: "nodeconfig-controller"}),

		queue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "nodeconfig"),
	}

	localScyllaPodsInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    snc.addPod,
		UpdateFunc: snc.updatePod,
	})

	namespacedJobInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    snc.addJob,
		UpdateFunc: snc.updateJob,
		DeleteFunc: snc.deleteJob,
	})

	return snc, nil
}

func (ncdc *Controller) processNextItem(ctx context.Context) bool {
	key, quit := ncdc.queue.Get()
	if quit {
		return false
	}
	defer ncdc.queue.Done(key)

	ctx, cancel := context.WithTimeout(ctx, maxSyncDuration)
	defer cancel()
	err := ncdc.sync(ctx, key.(string))
	// TODO: Do smarter filtering then just Reduce to handle cases like 2 conflict errors.
	err = utilerrors.Reduce(err)
	switch {
	case err == nil:
		ncdc.queue.Forget(key)
		return true
	default:
		utilruntime.HandleError(fmt.Errorf("syncing key '%v' failed: %v", key, err))
	}

	ncdc.queue.AddRateLimited(key)

	return true
}

func (ncdc *Controller) runWorker(ctx context.Context) {
	for ncdc.processNextItem(ctx) {
	}
}

func (ncdc *Controller) Run(ctx context.Context) {
	defer utilruntime.HandleCrash()

	klog.InfoS("Starting controller", "controller", ControllerName)

	var wg sync.WaitGroup
	defer func() {
		klog.InfoS("Shutting down controller", "controller", ControllerName)
		ncdc.queue.ShutDown()
		wg.Wait()
		klog.InfoS("Shut down controller", "controller", ControllerName)
	}()

	if !cache.WaitForNamedCacheSync(ControllerName, ctx.Done(), ncdc.cachesToSync...) {
		return
	}

	wg.Add(1)
	// Running tuning script in parallel on the same node is pointless.
	go func() {
		defer wg.Done()
		wait.UntilWithContext(ctx, ncdc.runWorker, time.Second)
	}()

	<-ctx.Done()
}

func (ncdc *Controller) enqueue() {
	ncdc.queue.Add(controllerKey)
}

func (ncdc *Controller) addPod(obj interface{}) {
	pod := obj.(*corev1.Pod)
	klog.V(4).InfoS("Observed addition of Pod", "Pod", klog.KObj(pod))
	ncdc.enqueue()
}

func (ncdc *Controller) updatePod(old, cur interface{}) {
	oldPod := old.(*corev1.Pod)
	currentPod := cur.(*corev1.Pod)

	klog.V(4).InfoS(
		"Observed update of Pod",
		"Pod", klog.KObj(currentPod),
		"RV", fmt.Sprintf("%s-%s", oldPod.ResourceVersion, currentPod.ResourceVersion),
		"UID", fmt.Sprintf("%s-%s", oldPod.UID, currentPod.UID),
	)
	ncdc.enqueue()
}

func (ncdc *Controller) ownsObject(obj metav1.Object) bool {
	ownerRef := metav1.GetControllerOfNoCopy(obj)
	if ownerRef == nil {
		return false
	}

	return ownerRef.UID == ncdc.nodeConfigUID
}

func (ncdc *Controller) addJob(obj interface{}) {
	job := obj.(*batchv1.Job)

	if !ncdc.ownsObject(job) {
		klog.V(5).InfoS("Not enqueueing Job not owned by us", "Job", klog.KObj(job), "RV", job.ResourceVersion)
	}

	klog.V(4).InfoS("Observed addition of Job", "Job", klog.KObj(job), "RV", job.ResourceVersion)
	ncdc.enqueue()
}

func (ncdc *Controller) updateJob(old, cur interface{}) {
	oldJob := old.(*batchv1.Job)
	currentJob := cur.(*batchv1.Job)

	if currentJob.UID != oldJob.UID {
		key, err := keyFunc(oldJob)
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("couldn't get key for object %#v: %v", oldJob, err))
			return
		}
		ncdc.deleteJob(cache.DeletedFinalStateUnknown{
			Key: key,
			Obj: oldJob,
		})
	}

	if !ncdc.ownsObject(currentJob) {
		klog.V(5).InfoS("Not enqueueing Job not owned by us", "Job", klog.KObj(currentJob), "RV", currentJob.ResourceVersion)
	}

	klog.V(4).InfoS(
		"Observed update of Job",
		"Job", klog.KObj(currentJob),
		"RV", fmt.Sprintf("%s->%s", oldJob.ResourceVersion, currentJob.ResourceVersion),
		"UID", fmt.Sprintf("%s->%s", oldJob.UID, currentJob.UID),
	)
	ncdc.enqueue()
}

func (ncdc *Controller) deleteJob(obj interface{}) {
	job, ok := obj.(*batchv1.Job)
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("couldn't get object from tombstone %#v", obj))
			return
		}
		job, ok = tombstone.Obj.(*batchv1.Job)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("tombstone contained object that is not a Job %#v", obj))
			return
		}
	}

	if !ncdc.ownsObject(job) {
		klog.V(5).InfoS("Not enqueueing Job not owned by us", "Job", klog.KObj(job), "RV", job.ResourceVersion)
	}

	klog.V(4).InfoS("Observed deletion of Job", "Job", klog.KObj(job), "RV", job.ResourceVersion)
	ncdc.enqueue()
}

func (ncdc *Controller) newControllerRef() *metav1.OwnerReference {
	return &metav1.OwnerReference{
		APIVersion:         controllerGVK.Version,
		Kind:               controllerGVK.Kind,
		Name:               ncdc.nodeConfigName,
		UID:                ncdc.nodeConfigUID,
		BlockOwnerDeletion: pointer.Bool(true),
		Controller:         pointer.Bool(true),
	}
}
