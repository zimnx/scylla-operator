// Copyright (c) 2023 ScyllaDB.

package nodesetup

import (
	"context"
	"fmt"
	"sort"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/disks"
	corev1 "k8s.io/api/core/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/klog/v2"
)

func (nsc *Controller) syncFilesystems(ctx context.Context, nc *scyllav1alpha1.NodeConfig, nodeStatus *scyllav1alpha1.NodeConfigNodeStatus) (bool, error) {
	var errs []error

	for _, fs := range nc.Spec.LocalDiskSetup.Filesystems {
		blockSize, err := disks.GetBlockSize(fs.Device, nsc.sysfsPath)
		if err != nil {
			errs = append(errs, fmt.Errorf("can't determine block size of device %q: %w", fs.Device, err))
			continue
		}

		changed, err := disks.MakeFS(ctx, nsc.executor, fs.Device, blockSize, string(fs.Type))
		if err != nil {
			nsc.eventRecorder.Eventf(
				nc,
				corev1.EventTypeWarning,
				"CreateFilesystemFailed",
				"Failed to create filesystem %s on %s device: %v",
				fs.Type, fs.Device, err,
			)
			errs = append(errs, fmt.Errorf("can't create filesystem %q on device %q: %w", fs.Type, fs.Device, err))
			continue
		}

		nodeStatus.FilesystemsCreated = append(nodeStatus.FilesystemsCreated, fs.Device)

		if !changed {
			klog.V(4).InfoS("Device already formatted, nothing to do", "device", fs.Device, "filesystem", fs.Type)
			continue
		}

		klog.V(2).InfoS("Filesystem has been created", "device", fs.Device, "filesystem", fs.Type)
		nsc.eventRecorder.Eventf(
			nc,
			corev1.EventTypeNormal,
			"FilesystemCreated",
			"Filesystem %s on %s device has been created",
			fs.Type, fs.Device,
		)
	}

	sort.Strings(nodeStatus.FilesystemsCreated)

	err := utilerrors.NewAggregate(errs)
	if err != nil {
		return false, fmt.Errorf("failed to create filesystems: %w", err)
	}

	return true, nil
}
