// Copyright (C) 2021 ScyllaDB

package nodeconfig

import (
	"context"
	"fmt"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/resourceapply"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
)

func (ncc *Controller) pruneDaemonSets(ctx context.Context, requiredDaemonSet *appsv1.DaemonSet, daemonSets map[string]*appsv1.DaemonSet) error {
	var errs []error
	for _, ds := range daemonSets {
		if ds.DeletionTimestamp != nil {
			continue
		}

		if ds.Name == requiredDaemonSet.Name {
			continue
		}

		propagationPolicy := metav1.DeletePropagationBackground
		err := ncc.kubeClient.AppsV1().DaemonSets(ds.Namespace).Delete(ctx, ds.Name, metav1.DeleteOptions{
			Preconditions: &metav1.Preconditions{
				UID: &ds.UID,
			},
			PropagationPolicy: &propagationPolicy,
		})
		if err != nil {
			errs = append(errs, err)
			continue
		}
	}
	return utilerrors.NewAggregate(errs)
}

func (ncc *Controller) syncDaemonSets(
	ctx context.Context,
	nc *scyllav1alpha1.NodeConfig,
	soc *scyllav1alpha1.ScyllaOperatorConfig,
	status *scyllav1alpha1.NodeConfigStatus,
	daemonSets map[string]*appsv1.DaemonSet,
) error {
	scyllaUtilsImage := soc.Spec.ScyllaUtilsImage
	// FIXME: check that its not empty, emit event
	// FIXME: add webhook validation for the format
	requiredDaemonSet := makeNodeConfigDaemonSet(nc, ncc.operatorImage, scyllaUtilsImage)

	// Delete any excessive DaemonSets.
	// Delete has to be the first action to avoid getting stuck on quota.
	err := ncc.pruneDaemonSets(ctx, requiredDaemonSet, daemonSets)
	if err != nil {
		return fmt.Errorf("can't delete DaemonSet(s): %w", err)
	}

	if requiredDaemonSet != nil {
		updatedDaemonSet, _, err := resourceapply.ApplyDaemonSet(ctx, ncc.kubeClient.AppsV1(), ncc.daemonSetLister, ncc.eventRecorder, requiredDaemonSet)
		if err != nil {
			return fmt.Errorf("can't apply statefulset update: %w", err)
		}

		status.Updated.Desired = updatedDaemonSet.Status.DesiredNumberScheduled
		status.Updated.Actual = updatedDaemonSet.Status.CurrentNumberScheduled
		status.Updated.Ready = updatedDaemonSet.Status.NumberReady
	}

	return nil
}
