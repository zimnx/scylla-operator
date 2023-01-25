// Copyright (c) 2023 ScyllaDB.

package nodesetup

import (
	"context"
	"fmt"

	"github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
)

func (nsc *Controller) updateNodeStatus(ctx context.Context, oldNC *v1alpha1.NodeConfig, nodeStatus *v1alpha1.NodeConfigNodeStatus) error {
	nc := oldNC.DeepCopy()

	nc.Status.NodeStatuses = controllerhelpers.SetNodeStatus(nc.Status.NodeStatuses, nodeStatus)

	if apiequality.Semantic.DeepEqual(nc.Status.NodeStatuses, oldNC.Status.NodeStatuses) {
		return nil
	}

	klog.V(2).InfoS("Updating status", "NodeConfig", klog.KObj(oldNC), "Node", nodeStatus.Name)

	_, err := nsc.scyllaClient.NodeConfigs().UpdateStatus(ctx, nc, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("can't update node config status %q: %w", nsc.nodeConfigName, err)
	}

	klog.V(2).InfoS("Status updated", "NodeConfig", klog.KObj(oldNC), "Node", nodeStatus.Name)

	return nil
}
