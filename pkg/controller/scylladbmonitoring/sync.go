package scylladbmonitoring

import (
	"context"
	"fmt"
	"time"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	monitoringv1 "github.com/scylladb/scylla-operator/pkg/externalapi/monitoring/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
)

type CT = *scyllav1alpha1.ScyllaDBMonitoring

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

	prometheuses, err := controllerhelpers.GetObjects[CT, *monitoringv1.Prometheus](
		ctx,
		sm,
		scylladbMonitoringControllerGVK,
		smc.getPrometheusSelector(sm),
		controllerhelpers.ControlleeManagerGetObjectsFuncs[CT, *monitoringv1.Prometheus]{
			GetControllerUncachedFunc: smc.scyllaV1alpha1Client.ScyllaDBMonitorings(sm.Namespace).Get,
			ListObjectsFunc:           smc.prometheusLister.List,
			PatchObjectFunc:           smc.monitoringClient.Prometheuses(sm.Namespace).Patch,
		},
	)

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
			return smc.syncPrometheus(ctx, sm, prometheuses)
		},
	)
	if err != nil {
		errs = append(errs, fmt.Errorf("can't sync prometheus: %w", err))
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
