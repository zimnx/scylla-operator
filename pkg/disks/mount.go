// Copyright (c) 2023 ScyllaDB.

package disks

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"k8s.io/klog/v2"
	"k8s.io/mount-utils"
)

func Mount(ctx context.Context, mounter mount.Interface, sourceDevice string, fsType string, targetDir string, options []string) (bool, error) {
	if _, err := os.Stat(sourceDevice); os.IsNotExist(err) {
		return false, fmt.Errorf("source device at %q doesn't exists", sourceDevice)
	}

	if _, err := os.Stat(targetDir); !os.IsNotExist(err) {
		mounted, err := mounter.IsMountPoint(targetDir)
		if err != nil {
			return false, fmt.Errorf("can't check if mount target directory at %q is mounted: %w", targetDir, err)
		}
		if mounted {
			mountedDevice, _, err := mount.GetDeviceNameFromMount(mounter, targetDir)
			if err != nil {
				return false, fmt.Errorf("can't determine which device is mounted at %q: %w", targetDir, err)
			}
			realDevice, err := filepath.EvalSymlinks(sourceDevice)
			if err != nil {
				return false, fmt.Errorf("can't evaluate source device %q symlink: %w", sourceDevice, err)
			}
			if mountedDevice != realDevice {
				return false, fmt.Errorf("mount point %q is alredy mounted by %q device, expected one is %q", targetDir, mountedDevice, realDevice)
			}
			klog.V(4).Infof("Target directory at %q is already mounted, nothing to do.", targetDir)
			return false, nil
		}
	}

	if err := os.MkdirAll(targetDir, os.ModePerm); err != nil {
		return false, fmt.Errorf("can't create target directory at %q: %w", targetDir, err)
	}

	err := mounter.Mount(sourceDevice, targetDir, fsType, options)
	if err != nil {
		return false, fmt.Errorf("can't mount source device %q at target %q with fsType %q and %v options: %w", sourceDevice, targetDir, fsType, options, err)
	}

	return true, nil
}
