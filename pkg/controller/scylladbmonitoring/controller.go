package scylladbmonitoring

import (
	"context"
	"fmt"
	"sync"
	"time"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	scyllav1alpha1client "github.com/scylladb/scylla-operator/pkg/client/scylla/clientset/versioned/typed/scylla/v1alpha1"
	scyllav1alpha1informers "github.com/scylladb/scylla-operator/pkg/client/scylla/informers/externalversions/scylla/v1alpha1"
	scyllav1alpha1listers "github.com/scylladb/scylla-operator/pkg/client/scylla/listers/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	integreatlyv1alpha1 "github.com/scylladb/scylla-operator/pkg/externalapi/integreatly/v1alpha1"
	monitoringv1 "github.com/scylladb/scylla-operator/pkg/externalapi/monitoring/v1"
	integreatlyv1alpha1client "github.com/scylladb/scylla-operator/pkg/externalclient/integreatly/clientset/versioned/typed/integreatly/v1alpha1"
	integreatlyv1alpha1informers "github.com/scylladb/scylla-operator/pkg/externalclient/integreatly/informers/externalversions/integreatly/v1alpha1"
	integreatlyv1alpha1listers "github.com/scylladb/scylla-operator/pkg/externalclient/integreatly/listers/integreatly/v1alpha1"
	monitoringv1client "github.com/scylladb/scylla-operator/pkg/externalclient/monitoring/clientset/versioned/typed/monitoring/v1"
	monitoringv1informers "github.com/scylladb/scylla-operator/pkg/externalclient/monitoring/informers/externalversions/monitoring/v1"
	monitoringv1listers "github.com/scylladb/scylla-operator/pkg/externalclient/monitoring/listers/monitoring/v1"
	"github.com/scylladb/scylla-operator/pkg/scheme"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	corev1informers "k8s.io/client-go/informers/core/v1"
	networkingv1informers "k8s.io/client-go/informers/networking/v1"
	policyv1informers "k8s.io/client-go/informers/policy/v1"
	rbacv1informers "k8s.io/client-go/informers/rbac/v1"
	"k8s.io/client-go/kubernetes"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
	networkingv1listers "k8s.io/client-go/listers/networking/v1"
	rbacv1listers "k8s.io/client-go/listers/rbac/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/component-base/metrics/prometheus/ratelimiter"
	"k8s.io/klog/v2"
)

const (
	ControllerName = "ScyllaDBMonitoringController"
)

var (
	keyFunc                         = cache.DeletionHandlingMetaNamespaceKeyFunc
	scylladbMonitoringControllerGVK = scyllav1alpha1.GroupVersion.WithKind("ScyllaDBMonitoring")
)

type Controller struct {
	kubeClient           kubernetes.Interface
	scyllaV1alpha1Client scyllav1alpha1client.ScyllaV1alpha1Interface
	monitoringClient     monitoringv1client.MonitoringV1Interface
	integreatlyClient    integreatlyv1alpha1client.IntegreatlyV1alpha1Interface

	endpointsLister      corev1listers.EndpointsLister
	secretLister         corev1listers.SecretLister
	configMapLister      corev1listers.ConfigMapLister
	serviceLister        corev1listers.ServiceLister
	serviceAccountLister corev1listers.ServiceAccountLister
	roleBindingLister    rbacv1listers.RoleBindingLister
	// pdbLister            policyv1listers.PodDisruptionBudgetLister
	ingressLister networkingv1listers.IngressLister

	scylladbMonitoringLister scyllav1alpha1listers.ScyllaDBMonitoringLister

	prometheusLister     monitoringv1listers.PrometheusLister
	prometheusRuleLister monitoringv1listers.PrometheusRuleLister
	serviceMonitorLister monitoringv1listers.ServiceMonitorLister

	grafanaLister           integreatlyv1alpha1listers.GrafanaLister
	grafanaFolderLister     integreatlyv1alpha1listers.GrafanaFolderLister
	grafanaDashboardLister  integreatlyv1alpha1listers.GrafanaDashboardLister
	grafanaDataSourceLister integreatlyv1alpha1listers.GrafanaDataSourceLister

	cachesToSync []cache.InformerSynced

	eventRecorder record.EventRecorder

	queue    workqueue.RateLimitingInterface
	handlers *controllerhelpers.Handlers[*scyllav1alpha1.ScyllaDBMonitoring]
}

func NewController(
	kubeClient kubernetes.Interface,
	scyllaV1alpha1Client scyllav1alpha1client.ScyllaV1alpha1Interface,
	monitoringClient monitoringv1client.MonitoringV1Interface,
	integreatlyClient integreatlyv1alpha1client.IntegreatlyV1alpha1Interface,
	endpointsInformer corev1informers.EndpointsInformer,
	secretInformer corev1informers.SecretInformer,
	configMapInformer corev1informers.ConfigMapInformer,
	serviceInformer corev1informers.ServiceInformer,
	serviceAccountInformer corev1informers.ServiceAccountInformer,
	roleBindingInformer rbacv1informers.RoleBindingInformer,
	pdbInformer policyv1informers.PodDisruptionBudgetInformer,
	ingressInformer networkingv1informers.IngressInformer,
	scyllaDBMonitoringInformer scyllav1alpha1informers.ScyllaDBMonitoringInformer,
	prometheusInformer monitoringv1informers.PrometheusInformer,
	prometheusRuleInformer monitoringv1informers.PrometheusRuleInformer,
	serviceMonitorInformer monitoringv1informers.ServiceMonitorInformer,
	grafanaInformer integreatlyv1alpha1informers.GrafanaInformer,
	grafanaFolderInformer integreatlyv1alpha1informers.GrafanaFolderInformer,
	grafanaDashboardInformer integreatlyv1alpha1informers.GrafanaDashboardInformer,
	grafanaDataSourceInformer integreatlyv1alpha1informers.GrafanaDataSourceInformer,
) (*Controller, error) {
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartStructuredLogging(0)
	eventBroadcaster.StartRecordingToSink(&corev1client.EventSinkImpl{Interface: kubeClient.CoreV1().Events("")})

	if kubeClient.CoreV1().RESTClient().GetRateLimiter() != nil {
		err := ratelimiter.RegisterMetricAndTrackRateLimiterUsage(
			"scylladbmonitoring_controller",
			kubeClient.CoreV1().RESTClient().GetRateLimiter(),
		)
		if err != nil {
			return nil, err
		}
	}

	smc := &Controller{
		kubeClient:           kubeClient,
		scyllaV1alpha1Client: scyllaV1alpha1Client,
		monitoringClient:     monitoringClient,
		integreatlyClient:    integreatlyClient,

		endpointsLister:      endpointsInformer.Lister(),
		secretLister:         secretInformer.Lister(),
		configMapLister:      configMapInformer.Lister(),
		serviceLister:        serviceInformer.Lister(),
		serviceAccountLister: serviceAccountInformer.Lister(),
		roleBindingLister:    roleBindingInformer.Lister(),
		// pdbLister:            pdbInformer.Lister(),
		ingressLister: ingressInformer.Lister(),

		scylladbMonitoringLister: scyllaDBMonitoringInformer.Lister(),

		prometheusLister:     prometheusInformer.Lister(),
		prometheusRuleLister: prometheusRuleInformer.Lister(),
		serviceMonitorLister: serviceMonitorInformer.Lister(),

		grafanaLister:           grafanaInformer.Lister(),
		grafanaFolderLister:     grafanaFolderInformer.Lister(),
		grafanaDashboardLister:  grafanaDashboardInformer.Lister(),
		grafanaDataSourceLister: grafanaDataSourceInformer.Lister(),

		cachesToSync: []cache.InformerSynced{
			endpointsInformer.Informer().HasSynced,
			secretInformer.Informer().HasSynced,
			configMapInformer.Informer().HasSynced,
			serviceInformer.Informer().HasSynced,
			serviceAccountInformer.Informer().HasSynced,
			roleBindingInformer.Informer().HasSynced,
			pdbInformer.Informer().HasSynced,
			ingressInformer.Informer().HasSynced,
			scyllaDBMonitoringInformer.Informer().HasSynced,
			prometheusInformer.Informer().HasSynced,
			prometheusRuleInformer.Informer().HasSynced,
			serviceMonitorInformer.Informer().HasSynced,
			grafanaInformer.Informer().HasSynced,
			grafanaFolderInformer.Informer().HasSynced,
			grafanaDashboardInformer.Informer().HasSynced,
			grafanaDataSourceInformer.Informer().HasSynced,
		},

		eventRecorder: eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: "scylladbmonitoring-controller"}),

		queue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "scylladbmonitoring"),
	}

	var err error
	smc.handlers, err = controllerhelpers.NewHandlers[*scyllav1alpha1.ScyllaDBMonitoring](
		smc.queue,
		keyFunc,
		scheme.Scheme,
		scylladbMonitoringControllerGVK,
		func(namespace, name string) (*scyllav1alpha1.ScyllaDBMonitoring, error) {
			return smc.scylladbMonitoringLister.ScyllaDBMonitorings(namespace).Get(name)
		},
	)
	if err != nil {
		return nil, fmt.Errorf("can't create handlers: %w", err)
	}

	scyllaDBMonitoringInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    smc.addScyllaDBMonitoring,
		UpdateFunc: smc.updateScyllaDBMonitoring,
		DeleteFunc: smc.deleteScyllaDBMonitoring,
	})

	prometheusInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    smc.addPrometheus,
		UpdateFunc: smc.updatePrometheus,
		DeleteFunc: smc.deletePrometheus,
	})

	prometheusRuleInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    smc.addPrometheusRule,
		UpdateFunc: smc.updatePrometheusRule,
		DeleteFunc: smc.deletePrometheusRule,
	})

	grafanaInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    smc.addGrafana,
		UpdateFunc: smc.updateGrafana,
		DeleteFunc: smc.deleteGrafana,
	})

	grafanaFolderInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    smc.addGrafanaFolder,
		UpdateFunc: smc.updateGrafanaFolder,
		DeleteFunc: smc.deleteGrafanaFolder,
	})

	grafanaDashboardInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    smc.addGrafanaDashboard,
		UpdateFunc: smc.updateGrafanaDashboard,
		DeleteFunc: smc.deleteGrafanaDashboard,
	})

	grafanaDataSourceInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    smc.addGrafanaDataSource,
		UpdateFunc: smc.updateGrafanaDataSource,
		DeleteFunc: smc.deleteGrafanaDataSource,
	})

	endpointsInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    smc.addEndpoints,
		UpdateFunc: smc.updateEndpoints,
		DeleteFunc: smc.deleteEndpoints,
	})

	// FIXME: more handlers

	return smc, nil
}

func (smc *Controller) addScyllaDBMonitoring(obj interface{}) {
	smc.handlers.HandleAdd(
		obj.(*scyllav1alpha1.ScyllaDBMonitoring),
		smc.handlers.Enqueue,
	)
}

func (smc *Controller) updateScyllaDBMonitoring(old, cur interface{}) {
	smc.handlers.HandleUpdate(
		old.(*scyllav1alpha1.ScyllaDBMonitoring),
		cur.(*scyllav1alpha1.ScyllaDBMonitoring),
		smc.handlers.Enqueue,
		smc.deleteScyllaDBMonitoring,
	)
}

func (smc *Controller) deleteScyllaDBMonitoring(obj interface{}) {
	smc.handlers.HandleDelete(
		obj,
		smc.handlers.Enqueue,
	)
}

func (smc *Controller) addEndpoints(obj interface{}) {
	smc.handlers.HandleAdd(
		obj.(*corev1.Endpoints),
		smc.handlers.EnqueueOwner,
	)
}

func (smc *Controller) updateEndpoints(old, cur interface{}) {
	smc.handlers.HandleUpdate(
		old.(*corev1.Endpoints),
		cur.(*corev1.Endpoints),
		smc.handlers.EnqueueOwner,
		smc.deleteEndpoints,
	)
}

func (smc *Controller) deleteEndpoints(obj interface{}) {
	smc.handlers.HandleDelete(
		obj,
		smc.handlers.EnqueueOwner,
	)
}

func (smc *Controller) addGrafana(obj interface{}) {
	smc.handlers.HandleAdd(
		obj.(*integreatlyv1alpha1.Grafana),
		smc.handlers.EnqueueOwner,
	)
}

func (smc *Controller) updateGrafana(old, cur interface{}) {
	smc.handlers.HandleUpdate(
		old.(*integreatlyv1alpha1.Grafana),
		cur.(*integreatlyv1alpha1.Grafana),
		smc.handlers.EnqueueOwner,
		smc.deleteGrafana,
	)
}

func (smc *Controller) deleteGrafana(obj interface{}) {
	smc.handlers.HandleDelete(
		obj,
		smc.handlers.EnqueueOwner,
	)
}

func (smc *Controller) addGrafanaFolder(obj interface{}) {
	smc.handlers.HandleAdd(
		obj.(*integreatlyv1alpha1.GrafanaFolder),
		smc.handlers.EnqueueOwner,
	)
}

func (smc *Controller) updateGrafanaFolder(old, cur interface{}) {
	smc.handlers.HandleUpdate(
		old.(*integreatlyv1alpha1.GrafanaFolder),
		cur.(*integreatlyv1alpha1.GrafanaFolder),
		smc.handlers.EnqueueOwner,
		smc.deleteGrafanaFolder,
	)
}

func (smc *Controller) deleteGrafanaFolder(obj interface{}) {
	smc.handlers.HandleDelete(
		obj,
		smc.handlers.EnqueueOwner,
	)
}

func (smc *Controller) addGrafanaDashboard(obj interface{}) {
	smc.handlers.HandleAdd(
		obj.(*integreatlyv1alpha1.GrafanaDashboard),
		smc.handlers.EnqueueOwner,
	)
}

func (smc *Controller) updateGrafanaDashboard(old, cur interface{}) {
	smc.handlers.HandleUpdate(
		old.(*integreatlyv1alpha1.GrafanaDashboard),
		cur.(*integreatlyv1alpha1.GrafanaDashboard),
		smc.handlers.EnqueueOwner,
		smc.deleteGrafanaDashboard,
	)
}

func (smc *Controller) deleteGrafanaDashboard(obj interface{}) {
	smc.handlers.HandleDelete(
		obj,
		smc.handlers.EnqueueOwner,
	)
}

func (smc *Controller) addGrafanaDataSource(obj interface{}) {
	smc.handlers.HandleAdd(
		obj.(*integreatlyv1alpha1.GrafanaDataSource),
		smc.handlers.EnqueueOwner,
	)
}

func (smc *Controller) updateGrafanaDataSource(old, cur interface{}) {
	smc.handlers.HandleUpdate(
		old.(*integreatlyv1alpha1.GrafanaDataSource),
		cur.(*integreatlyv1alpha1.GrafanaDataSource),
		smc.handlers.EnqueueOwner,
		smc.deleteGrafanaDataSource,
	)
}

func (smc *Controller) deleteGrafanaDataSource(obj interface{}) {
	smc.handlers.HandleDelete(
		obj,
		smc.handlers.EnqueueOwner,
	)
}

func (smc *Controller) addPrometheus(obj interface{}) {
	smc.handlers.HandleAdd(
		obj.(*monitoringv1.Prometheus),
		smc.handlers.EnqueueOwner,
	)
}

func (smc *Controller) updatePrometheus(old, cur interface{}) {
	smc.handlers.HandleUpdate(
		old.(*monitoringv1.Prometheus),
		cur.(*monitoringv1.Prometheus),
		smc.handlers.EnqueueOwner,
		smc.deletePrometheus,
	)
}

func (smc *Controller) deletePrometheus(obj interface{}) {
	smc.handlers.HandleDelete(
		obj,
		smc.handlers.EnqueueOwner,
	)
}

func (smc *Controller) addPrometheusRule(obj interface{}) {
	smc.handlers.HandleAdd(
		obj.(*monitoringv1.PrometheusRule),
		smc.handlers.EnqueueOwner,
	)
}

func (smc *Controller) updatePrometheusRule(old, cur interface{}) {
	smc.handlers.HandleUpdate(
		old.(*monitoringv1.PrometheusRule),
		cur.(*monitoringv1.PrometheusRule),
		smc.handlers.EnqueueOwner,
		smc.deletePrometheusRule,
	)
}

func (smc *Controller) deletePrometheusRule(obj interface{}) {
	smc.handlers.HandleDelete(
		obj,
		smc.handlers.EnqueueOwner,
	)
}

func (smc *Controller) processNextItem(ctx context.Context) bool {
	key, quit := smc.queue.Get()
	if quit {
		return false
	}
	defer smc.queue.Done(key)

	err := smc.sync(ctx, key.(string))
	// TODO: Do smarter filtering then just Reduce to handle cases like 2 conflict errors.
	err = utilerrors.Reduce(err)
	switch {
	case err == nil:
		smc.queue.Forget(key)
		return true

	case apierrors.IsConflict(err):
		klog.V(2).InfoS("Hit conflict, will retry in a bit", "Key", key, "Error", err)

	case apierrors.IsAlreadyExists(err):
		klog.V(2).InfoS("Hit already exists, will retry in a bit", "Key", key, "Error", err)

	default:
		utilruntime.HandleError(fmt.Errorf("syncing key '%v' failed: %v", key, err))
	}

	smc.queue.AddRateLimited(key)

	return true
}

func (smc *Controller) runWorker(ctx context.Context) {
	for smc.processNextItem(ctx) {
	}
}

func (smc *Controller) Run(ctx context.Context, workers int) {
	defer utilruntime.HandleCrash()

	klog.InfoS("Starting controller", "controller", "ScyllaDBMonitoring")

	var wg sync.WaitGroup
	defer func() {
		klog.InfoS("Shutting down controller", "controller", "ScyllaDBMonitoring")
		smc.queue.ShutDown()
		wg.Wait()
		klog.InfoS("Shut down controller", "controller", "ScyllaDBMonitoring")
	}()

	if !cache.WaitForNamedCacheSync(ControllerName, ctx.Done(), smc.cachesToSync...) {
		return
	}

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			wait.UntilWithContext(ctx, smc.runWorker, time.Second)
		}()
	}

	<-ctx.Done()
}
