// Copyright (c) 2023 ScyllaDB.

package nodesetup

import (
	"context"
	"fmt"
	"sort"
	"strings"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/disks"
	corev1 "k8s.io/api/core/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/klog/v2"
)

func (nsc *Controller) syncMounts(ctx context.Context, nc *scyllav1alpha1.NodeConfig, nodeStatus *scyllav1alpha1.NodeConfigNodeStatus) (bool, error) {
	var errs []error

	for _, mc := range nc.Spec.LocalDiskSetup.Mounts {
		changed, err := disks.Mount(ctx, nsc.mounter, mc.Device, mc.FsType, mc.MountPoint, mc.UnsupportedOptions)
		if err != nil {
			nsc.eventRecorder.Eventf(
				nc,
				corev1.EventTypeWarning,
				"CreateMountFailed",
				"Failed to create mount from %s device at %s: %v",
				mc.Device, mc.MountPoint, err,
			)
			errs = append(errs, fmt.Errorf("can't mount %q device into %q with %q filesystem, using %q options: %w", mc.Device, mc.MountPoint, mc.FsType, strings.Join(mc.UnsupportedOptions, ","), err))
			continue
		}

		nodeStatus.MountsCreated = append(nodeStatus.MountsCreated, mc.Device)

		if !changed {
			klog.V(4).InfoS("Device already mounted, nothing to do", "device", mc.Device, "mountPoint", mc.MountPoint)
			continue
		}

		klog.V(2).InfoS("Mount has been created", "device", mc.Device, "mountPoint", mc.MountPoint)
		nsc.eventRecorder.Eventf(
			nc,
			corev1.EventTypeNormal,
			"MountCreated",
			"Mount of %s at %s mount point has been created",
			mc.Device, mc.MountPoint,
		)
	}

	sort.Strings(nodeStatus.MountsCreated)

	err := utilerrors.NewAggregate(errs)
	if err != nil {
		return false, fmt.Errorf("failed to create mounts: %w", err)
	}

	return true, nil
}
