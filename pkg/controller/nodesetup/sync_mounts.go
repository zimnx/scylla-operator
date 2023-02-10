// Copyright (c) 2023 ScyllaDB.

package nodesetup

import (
	"context"
	"fmt"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/systemd"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/klog/v2"
)

func (nsc *Controller) syncMounts(ctx context.Context, nc *scyllav1alpha1.NodeConfig, nodeStatus *scyllav1alpha1.NodeConfigNodeStatus) ([]metav1.Condition, error) {
	var errs []error
	var progressingConditions []metav1.Condition

	var mountUnits []*systemd.NamedUnit
	for _, mc := range nc.Spec.LocalDiskSetup.Mounts {
		mount := systemd.Mount{
			Description: fmt.Sprintf("Managed mount by Scylla Operator"),
			Device:      mc.Device,
			MountPoint:  mc.MountPoint,
			FSType:      mc.FsType,
			Options:     mc.UnsupportedOptions,
		}

		mountUnit, err := mount.MakeUnit()
		if err != nil {
			errs = append(errs, fmt.Errorf("can't make unit: %w", err))
			continue
		}

		mountUnits = append(mountUnits, mountUnit)

		klog.V(4).InfoS("Mount unit has been generated and queued for apply.", "Name", mountUnit.FileName, "Device", mc.Device, "MountPoint", mc.MountPoint)
	}

	err := nsc.systemdUnitManager.EnsureUnits(ctx, mountUnits, nsc.systemdControl)
	if err != nil {
		errs = append(errs, fmt.Errorf("can't ensure units: %w", err))
	}

	err = utilerrors.NewAggregate(errs)
	if err != nil {
		return progressingConditions, fmt.Errorf("can't create mounts: %w", err)
	}

	return progressingConditions, nil
}
