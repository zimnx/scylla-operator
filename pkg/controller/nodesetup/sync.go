// Copyright (c) 2023 ScyllaDB.

package nodesetup

import (
	"context"
	"fmt"
	"time"

	"github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/klog/v2"
)

func (nsc *Controller) sync(ctx context.Context) error {
	startTime := time.Now()
	klog.V(4).InfoS("Started syncing NodeConfig", "NodeConfig", nsc.nodeConfigName, "startTime", startTime)
	defer func() {
		klog.V(4).InfoS("Finished syncing NodeConfig", "NodeConfig", nsc.nodeConfigName, "duration", time.Since(startTime))
	}()

	nc, err := nsc.nodeConfigLister.Get(nsc.nodeConfigName)
	if apierrors.IsNotFound(err) {
		klog.V(2).InfoS("NodeConfig has been deleted", "NodeConfig", klog.KObj(nc))
		return nil
	}
	if err != nil {
		return fmt.Errorf("can't list nodeconfigs: %w", err)
	}

	if nc.UID != nsc.nodeConfigUID {
		return fmt.Errorf("nodeConfig UID %q doesn't match the expected UID %q", nc.UID, nc.UID)
	}

	nodeStatus := controllerhelpers.FindNodeStatus(nc.Status.NodeStatuses, nsc.nodeName)
	if nodeStatus == nil {
		nodeStatus = &v1alpha1.NodeConfigNodeStatus{
			Name: nsc.nodeName,
		}
	}

	if nc.DeletionTimestamp != nil {
		return nsc.updateNodeStatus(ctx, nc, nodeStatus)
	}

	var errs []error

	if nc.Spec.LocalDiskSetup != nil {
		nodeStatus.DisksSetUp = true

		raidsFinished, err := nsc.syncRaids(ctx, nc, nodeStatus)
		if err != nil {
			errs = append(errs, fmt.Errorf("can't sync raids: %w", err))
		}
		nodeStatus.DisksSetUp = nodeStatus.DisksSetUp && raidsFinished

		filesystemsFinished, err := nsc.syncFilesystems(ctx, nc, nodeStatus)
		if err != nil {
			errs = append(errs, fmt.Errorf("can't sync filesystems: %w", err))
		}
		nodeStatus.DisksSetUp = nodeStatus.DisksSetUp && filesystemsFinished

		_, err = nsc.syncMounts(ctx, nc, nodeStatus)
		if err != nil {
			errs = append(errs, fmt.Errorf("can't sync mounts: %w", err))
		}
		// FIXME: this should be conditions
		nodeStatus.DisksSetUp = false
	}

	err = nsc.updateNodeStatus(ctx, nc, nodeStatus)
	if err != nil {
		errs = append(errs, fmt.Errorf("can't update node status: %w", err))
	}

	err = utilerrors.NewAggregate(errs)
	if err != nil {
		return fmt.Errorf("failed to sync nodeconfig %q: %w", nsc.nodeConfigName, err)
	}

	return nil
}
