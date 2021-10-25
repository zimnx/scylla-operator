// Copyright (C) 2021 ScyllaDB

package nodeconfig

import (
	"context"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
)

func (ncc *Controller) calculateStatus(snc *scyllav1alpha1.NodeConfig, daemonSets map[string]*appsv1.DaemonSet) *scyllav1alpha1.NodeConfigStatus {
	status := snc.Status.DeepCopy()
	status.ObservedGeneration = snc.Generation

	for _, ds := range daemonSets {
		if snc.Name == ds.Name {
			status.Current.Ready = ds.Status.NumberReady
			status.Current.Desired = ds.Status.DesiredNumberScheduled
			status.Current.Actual = ds.Status.CurrentNumberScheduled
		}
	}

	return status
}

func (ncc *Controller) updateStatus(ctx context.Context, currentNodeConfig *scyllav1alpha1.NodeConfig, status *scyllav1alpha1.NodeConfigStatus) error {
	if apiequality.Semantic.DeepEqual(&currentNodeConfig.Status, status) {
		return nil
	}

	soc := currentNodeConfig.DeepCopy()
	soc.Status = *status

	klog.V(2).InfoS("Updating status", "NodeConfig", klog.KObj(soc))

	_, err := ncc.scyllaClient.NodeConfigs().UpdateStatus(ctx, soc, metav1.UpdateOptions{})
	if err != nil {
		return err
	}

	klog.V(2).InfoS("Status updated", "NodeConfig", klog.KObj(soc))

	return nil
}
