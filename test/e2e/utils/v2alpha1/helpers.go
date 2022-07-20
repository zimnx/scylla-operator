// Copyright (c) 2022 ScyllaDB.

package v2alpha1

import (
	"context"
	"fmt"
	"time"

	scyllav2alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v2alpha1"
	scyllav2alpha1client "github.com/scylladb/scylla-operator/pkg/client/scylla/clientset/versioned/typed/scylla/v2alpha1"
	"github.com/scylladb/scylla-operator/pkg/helpers"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
	watchtools "k8s.io/client-go/tools/watch"
	"k8s.io/klog/v2"
)

func RolloutTimeoutForScyllaCluster(sc *scyllav2alpha1.ScyllaCluster) time.Duration {
	return baseRolloutTimout + time.Duration(GetScyllaClusterTotalNodeCount(sc))*memberRolloutTimeout
}

func ContextForScyllaClusterRollout(parent context.Context, sc *scyllav2alpha1.ScyllaCluster) (context.Context, context.CancelFunc) {
	return context.WithTimeout(parent, RolloutTimeoutForScyllaCluster(sc))
}

func GetScyllaClusterTotalNodeCount(sc *scyllav2alpha1.ScyllaCluster) int32 {
	var totalNodeCount int32
	for _, dc := range sc.Spec.Datacenters {
		if dc.NodesPerRack != nil {
			totalNodeCount += int32(len(dc.Racks)) * *dc.NodesPerRack
		}
	}
	return totalNodeCount
}

type listerWatcher[ListObject runtime.Object] interface {
	List(context.Context, metav1.ListOptions) (ListObject, error)
	Watch(context.Context, metav1.ListOptions) (watch.Interface, error)
}

type WaitForStateOptions struct {
	TolerateDelete bool
}

func IsScyllaClusterRolledOut(sc *scyllav2alpha1.ScyllaCluster) (bool, error) {
	// ObservedGeneration == nil will filter out the case when the object is initially created
	// so no other optional (but required) field should be nil after this point and we should error out.
	if sc.Status.ObservedGeneration == nil || *sc.Status.ObservedGeneration < sc.Generation {
		return false, nil
	}

	totalNodeCount := GetScyllaClusterTotalNodeCount(sc)
	if sc.Status.Nodes == nil {
		return false, fmt.Errorf("nodes shouldn't be nil")
	}

	if *sc.Status.Nodes != totalNodeCount {
		return false, nil
	}

	if sc.Status.UpdatedNodes == nil {
		return false, fmt.Errorf("updatedNodes shouldn't be nil")
	}

	if *sc.Status.UpdatedNodes != totalNodeCount {
		return false, nil
	}

	if sc.Status.ReadyNodes == nil {
		return false, fmt.Errorf("readyNodes shouldn't be nil")
	}

	if *sc.Status.ReadyNodes != totalNodeCount {
		return false, nil
	}

	framework.Infof("ScyllaCluster %s (RV=%s) is rolled out", klog.KObj(sc), sc.ResourceVersion)

	return true, nil
}

func WaitForObjectState[Object, ListObject runtime.Object](ctx context.Context, client listerWatcher[ListObject], name string, options WaitForStateOptions, condition func(obj Object) (bool, error), additionalConditions ...func(obj Object) (bool, error)) (Object, error) {
	fieldSelector := fields.OneTermEqualSelector("metadata.name", name).String()
	lw := &cache.ListWatch{
		ListFunc: helpers.UncachedListFunc(func(options metav1.ListOptions) (runtime.Object, error) {
			options.FieldSelector = fieldSelector
			return client.List(ctx, options)
		}),
		WatchFunc: func(options metav1.ListOptions) (i watch.Interface, e error) {
			options.FieldSelector = fieldSelector
			return client.Watch(ctx, options)
		},
	}

	conditions := make([]func(Object) (bool, error), 0, 1+len(additionalConditions))
	conditions = append(conditions, condition)
	if len(additionalConditions) != 0 {
		conditions = append(conditions, additionalConditions...)
	}
	aggregatedCond := func(obj Object) (bool, error) {
		allDone := true
		for _, c := range conditions {
			var err error
			var done bool

			done, err = c(obj)
			if err != nil {
				return done, err
			}
			if !done {
				allDone = false
			}
		}
		return allDone, nil
	}

	event, err := watchtools.UntilWithSync(ctx, lw, *new(Object), nil, func(event watch.Event) (bool, error) {
		switch event.Type {
		case watch.Added, watch.Modified:
			return aggregatedCond(event.Object.(Object))

		case watch.Deleted:
			if options.TolerateDelete {
				return aggregatedCond(event.Object.(Object))
			}
			fallthrough

		default:
			return true, fmt.Errorf("unexpected event: %#v", event)
		}
	})
	if err != nil {
		return *new(Object), err
	}

	return event.Object.(Object), nil
}

func WaitForScyllaClusterState(ctx context.Context, client scyllav2alpha1client.ScyllaV2alpha1Interface, namespace string, name string, options WaitForStateOptions, condition func(sc *scyllav2alpha1.ScyllaCluster) (bool, error), additionalConditions ...func(sc *scyllav2alpha1.ScyllaCluster) (bool, error)) (*scyllav2alpha1.ScyllaCluster, error) {
	return WaitForObjectState[*scyllav2alpha1.ScyllaCluster, *scyllav2alpha1.ScyllaClusterList](ctx, client.ScyllaClusters(namespace), name, options, condition, additionalConditions...)
}
