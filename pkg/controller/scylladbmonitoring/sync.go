package scylladbmonitoring

import (
	"context"
	"fmt"
	"time"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	integreatlyv1alpha1 "github.com/scylladb/scylla-operator/pkg/externalapi/integreatly/v1alpha1"
	monitoringv1 "github.com/scylladb/scylla-operator/pkg/externalapi/monitoring/v1"
	"github.com/scylladb/scylla-operator/pkg/naming"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
)

type CT = *scyllav1alpha1.ScyllaDBMonitoring

func getLabels(sm *scyllav1alpha1.ScyllaDBMonitoring) labels.Set {
	return labels.Set{
		naming.ScyllaDBMonitoringNameLabel: sm.Name,
	}
}
func getSelector(sm *scyllav1alpha1.ScyllaDBMonitoring) labels.Selector {
	return labels.SelectorFromSet(getLabels(sm))
}

func (smc *Controller) sync(ctx context.Context, key string) error {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		klog.ErrorS(err, "Failed to split meta namespace cache key", "cacheKey", key)
		return err
	}

	startTime := time.Now()
	klog.V(4).InfoS("Started syncing ScyllaDBMonitoring", "ScyllaDBMonitoring", klog.KRef(namespace, name), "startTime", startTime)
	defer func() {
		klog.V(4).InfoS("Finished syncing ScyllaDBMonitoring", "ScyllaDBMonitoring", klog.KRef(namespace, name), "duration", time.Since(startTime))
	}()

	sm, err := smc.scylladbMonitoringLister.ScyllaDBMonitorings(namespace).Get(name)
	if errors.IsNotFound(err) {
		klog.V(2).InfoS("ScyllaDBMonitoring has been deleted", "ScyllaDBMonitoring", klog.KObj(sm))
		return nil
	}
	if err != nil {
		return err
	}

	smSelector := getSelector(sm)

	prometheuses, err := controllerhelpers.GetObjects[CT, *monitoringv1.Prometheus](
		ctx,
		sm,
		scylladbMonitoringControllerGVK,
		smSelector,
		controllerhelpers.ControlleeManagerGetObjectsFuncs[CT, *monitoringv1.Prometheus]{
			GetControllerUncachedFunc: smc.scyllaV1alpha1Client.ScyllaDBMonitorings(sm.Namespace).Get,
			ListObjectsFunc:           smc.prometheusLister.Prometheuses(sm.Namespace).List,
			PatchObjectFunc:           smc.monitoringClient.Prometheuses(sm.Namespace).Patch,
		},
	)

	grafanas, err := controllerhelpers.GetObjects[CT, *integreatlyv1alpha1.Grafana](
		ctx,
		sm,
		scylladbMonitoringControllerGVK,
		smSelector,
		controllerhelpers.ControlleeManagerGetObjectsFuncs[CT, *integreatlyv1alpha1.Grafana]{
			GetControllerUncachedFunc: smc.scyllaV1alpha1Client.ScyllaDBMonitorings(sm.Namespace).Get,
			ListObjectsFunc:           smc.grafanaLister.Grafanas(sm.Namespace).List,
			PatchObjectFunc:           smc.integreatlyClient.Grafanas(sm.Namespace).Patch,
		},
	)

	grafanaFolders, err := controllerhelpers.GetObjects[CT, *integreatlyv1alpha1.GrafanaFolder](
		ctx,
		sm,
		scylladbMonitoringControllerGVK,
		smSelector,
		controllerhelpers.ControlleeManagerGetObjectsFuncs[CT, *integreatlyv1alpha1.GrafanaFolder]{
			GetControllerUncachedFunc: smc.scyllaV1alpha1Client.ScyllaDBMonitorings(sm.Namespace).Get,
			ListObjectsFunc:           smc.grafanaFolderLister.GrafanaFolders(sm.Namespace).List,
			PatchObjectFunc:           smc.integreatlyClient.GrafanaFolders(sm.Namespace).Patch,
		},
	)

	dashboards, err := controllerhelpers.GetObjects[CT, *integreatlyv1alpha1.GrafanaDashboard](
		ctx,
		sm,
		scylladbMonitoringControllerGVK,
		smSelector,
		controllerhelpers.ControlleeManagerGetObjectsFuncs[CT, *integreatlyv1alpha1.GrafanaDashboard]{
			GetControllerUncachedFunc: smc.scyllaV1alpha1Client.ScyllaDBMonitorings(sm.Namespace).Get,
			ListObjectsFunc:           smc.grafanaDashboardLister.GrafanaDashboards(sm.Namespace).List,
			PatchObjectFunc:           smc.integreatlyClient.GrafanaDashboards(sm.Namespace).Patch,
		},
	)

	datasources, err := controllerhelpers.GetObjects[CT, *integreatlyv1alpha1.GrafanaDataSource](
		ctx,
		sm,
		scylladbMonitoringControllerGVK,
		smSelector,
		controllerhelpers.ControlleeManagerGetObjectsFuncs[CT, *integreatlyv1alpha1.GrafanaDataSource]{
			GetControllerUncachedFunc: smc.scyllaV1alpha1Client.ScyllaDBMonitorings(sm.Namespace).Get,
			ListObjectsFunc:           smc.grafanaDataSourceLister.GrafanaDataSources(sm.Namespace).List,
			PatchObjectFunc:           smc.integreatlyClient.GrafanaDataSources(sm.Namespace).Patch,
		},
	)

	ingresses, err := controllerhelpers.GetObjects[CT, *networkingv1.Ingress](
		ctx,
		sm,
		scylladbMonitoringControllerGVK,
		smSelector,
		controllerhelpers.ControlleeManagerGetObjectsFuncs[CT, *networkingv1.Ingress]{
			GetControllerUncachedFunc: smc.scyllaV1alpha1Client.ScyllaDBMonitorings(sm.Namespace).Get,
			ListObjectsFunc:           smc.ingressLister.Ingresses(sm.Namespace).List,
			PatchObjectFunc:           smc.kubeClient.NetworkingV1().Ingresses(sm.Namespace).Patch,
		},
	)

	secrets, err := controllerhelpers.GetObjects[CT, *corev1.Secret](
		ctx,
		sm,
		scylladbMonitoringControllerGVK,
		smSelector,
		controllerhelpers.ControlleeManagerGetObjectsFuncs[CT, *corev1.Secret]{
			GetControllerUncachedFunc: smc.scyllaV1alpha1Client.ScyllaDBMonitorings(sm.Namespace).Get,
			ListObjectsFunc:           smc.secretLister.Secrets(sm.Namespace).List,
			PatchObjectFunc:           smc.kubeClient.CoreV1().Secrets(sm.Namespace).Patch,
		},
	)

	configMaps, err := controllerhelpers.GetObjects[CT, *corev1.ConfigMap](
		ctx,
		sm,
		scylladbMonitoringControllerGVK,
		smSelector,
		controllerhelpers.ControlleeManagerGetObjectsFuncs[CT, *corev1.ConfigMap]{
			GetControllerUncachedFunc: smc.scyllaV1alpha1Client.ScyllaDBMonitorings(sm.Namespace).Get,
			ListObjectsFunc:           smc.configMapLister.ConfigMaps(sm.Namespace).List,
			PatchObjectFunc:           smc.kubeClient.CoreV1().ConfigMaps(sm.Namespace).Patch,
		},
	)

	serviceAccounts, err := controllerhelpers.GetObjects[CT, *corev1.ServiceAccount](
		ctx,
		sm,
		scylladbMonitoringControllerGVK,
		smSelector,
		controllerhelpers.ControlleeManagerGetObjectsFuncs[CT, *corev1.ServiceAccount]{
			GetControllerUncachedFunc: smc.scyllaV1alpha1Client.ScyllaDBMonitorings(sm.Namespace).Get,
			ListObjectsFunc:           smc.serviceAccountLister.ServiceAccounts(sm.Namespace).List,
			PatchObjectFunc:           smc.kubeClient.CoreV1().ServiceAccounts(sm.Namespace).Patch,
		},
	)

	services, err := controllerhelpers.GetObjects[CT, *corev1.Service](
		ctx,
		sm,
		scylladbMonitoringControllerGVK,
		smSelector,
		controllerhelpers.ControlleeManagerGetObjectsFuncs[CT, *corev1.Service]{
			GetControllerUncachedFunc: smc.scyllaV1alpha1Client.ScyllaDBMonitorings(sm.Namespace).Get,
			ListObjectsFunc:           smc.serviceLister.Services(sm.Namespace).List,
			PatchObjectFunc:           smc.kubeClient.CoreV1().Services(sm.Namespace).Patch,
		},
	)

	roleBindings, err := controllerhelpers.GetObjects[CT, *rbacv1.RoleBinding](
		ctx,
		sm,
		scylladbMonitoringControllerGVK,
		smSelector,
		controllerhelpers.ControlleeManagerGetObjectsFuncs[CT, *rbacv1.RoleBinding]{
			GetControllerUncachedFunc: smc.scyllaV1alpha1Client.ScyllaDBMonitorings(sm.Namespace).Get,
			ListObjectsFunc:           smc.roleBindingLister.RoleBindings(sm.Namespace).List,
			PatchObjectFunc:           smc.kubeClient.RbacV1().RoleBindings(sm.Namespace).Patch,
		},
	)

	serviceMonitors, err := controllerhelpers.GetObjects[CT, *monitoringv1.ServiceMonitor](
		ctx,
		sm,
		scylladbMonitoringControllerGVK,
		smSelector,
		controllerhelpers.ControlleeManagerGetObjectsFuncs[CT, *monitoringv1.ServiceMonitor]{
			GetControllerUncachedFunc: smc.scyllaV1alpha1Client.ScyllaDBMonitorings(sm.Namespace).Get,
			ListObjectsFunc:           smc.serviceMonitorLister.ServiceMonitors(sm.Namespace).List,
			PatchObjectFunc:           smc.monitoringClient.ServiceMonitors(sm.Namespace).Patch,
		},
	)

	prometheusRules, err := controllerhelpers.GetObjects[CT, *monitoringv1.PrometheusRule](
		ctx,
		sm,
		scylladbMonitoringControllerGVK,
		smSelector,
		controllerhelpers.ControlleeManagerGetObjectsFuncs[CT, *monitoringv1.PrometheusRule]{
			GetControllerUncachedFunc: smc.scyllaV1alpha1Client.ScyllaDBMonitorings(sm.Namespace).Get,
			ListObjectsFunc:           smc.prometheusRuleLister.PrometheusRules(sm.Namespace).List,
			PatchObjectFunc:           smc.monitoringClient.PrometheusRules(sm.Namespace).Patch,
		},
	)

	prometheusSelector := getPrometheusSelector(sm)
	grafanaSelector := getGrafanaSelector(sm)

	status := &scyllav1alpha1.ScyllaDBMonitoringStatus{}
	// status := mc.calculateStatus(sc, statefulSetMap, serviceMap)

	if sm.DeletionTimestamp != nil {
		return smc.updateStatus(ctx, sm, status)
	}

	var errs []error

	err = controllerhelpers.RunSync(
		&status.Conditions,
		prometheusControllerProgressingCondition,
		prometheusControllerDegradedCondition,
		sm.Generation,
		func() ([]metav1.Condition, error) {
			return smc.syncPrometheus(
				ctx,
				sm,
				controllerhelpers.FilterObjectMapByLabel(prometheuses, prometheusSelector),
				controllerhelpers.FilterObjectMapByLabel(prometheusRules, prometheusSelector),
				controllerhelpers.FilterObjectMapByLabel(serviceMonitors, prometheusSelector),
				controllerhelpers.FilterObjectMapByLabel(secrets, prometheusSelector),
				controllerhelpers.FilterObjectMapByLabel(configMaps, prometheusSelector),
				controllerhelpers.FilterObjectMapByLabel(serviceAccounts, prometheusSelector),
				controllerhelpers.FilterObjectMapByLabel(services, prometheusSelector),
				controllerhelpers.FilterObjectMapByLabel(roleBindings, prometheusSelector),
			)
		},
	)
	if err != nil {
		errs = append(errs, fmt.Errorf("can't sync prometheus: %w", err))
	}

	err = controllerhelpers.RunSync(
		&status.Conditions,
		grafanaControllerProgressingCondition,
		grafanaControllerDegradedCondition,
		sm.Generation,
		func() ([]metav1.Condition, error) {
			return smc.syncGrafana(
				ctx,
				sm,
				controllerhelpers.FilterObjectMapByLabel(grafanas, grafanaSelector),
				controllerhelpers.FilterObjectMapByLabel(grafanaFolders, grafanaSelector),
				controllerhelpers.FilterObjectMapByLabel(dashboards, grafanaSelector),
				controllerhelpers.FilterObjectMapByLabel(datasources, grafanaSelector),
				controllerhelpers.FilterObjectMapByLabel(secrets, grafanaSelector),
				controllerhelpers.FilterObjectMapByLabel(configMaps, grafanaSelector),
				controllerhelpers.FilterObjectMapByLabel(ingresses, grafanaSelector),
			)
		},
	)
	if err != nil {
		errs = append(errs, fmt.Errorf("can't sync grafana: %w", err))
	}

	// Aggregate conditions.
	err = controllerhelpers.SetAggregatedWorkloadConditions(&status.Conditions, sm.Generation)
	if err != nil {
		errs = append(errs, fmt.Errorf("can't aggregate workload conditions: %w", err))
	} else {
		err = smc.updateStatus(ctx, sm, status)
		errs = append(errs, err)
	}

	return utilerrors.NewAggregate(errs)
}
