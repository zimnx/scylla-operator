// Copyright (c) 2023 ScyllaDB.

package nodesetup

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"

	scyllav1alpha1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1alpha1"
	"github.com/scylladb/scylla-operator/pkg/disks"
	"github.com/scylladb/scylla-operator/pkg/util/blkutils"
	corev1 "k8s.io/api/core/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/klog/v2"
)

func (nsc *Controller) syncRaids(ctx context.Context, nc *scyllav1alpha1.NodeConfig, nodeStatus *scyllav1alpha1.NodeConfigNodeStatus) (bool, error) {
	var errs []error

	blockDevices, err := blkutils.ListBlockDevices(ctx, nsc.executor)
	if err != nil {
		return false, fmt.Errorf("can't list block devices: %w", err)
	}

	udevControlEnabled := false
	if _, err := os.Stat("/run/udev/control"); !os.IsNotExist(err) {
		udevControlEnabled = true
	}

	for _, rc := range nc.Spec.LocalDiskSetup.Raids {
		switch rc.Type {
		case scyllav1alpha1.RAID0Type:
			modelRe, err := regexp.Compile(rc.RAID0.Devices.ModelRegex)
			if err != nil {
				errs = append(errs, fmt.Errorf("can't compile device model regexp %q: %w", rc.RAID0.Devices.ModelRegex, err))
				continue
			}

			nameRe, err := regexp.Compile(rc.RAID0.Devices.NameRegex)
			if err != nil {
				errs = append(errs, fmt.Errorf("can't compile device name regexp %q: %w", rc.RAID0.Devices.NameRegex, err))
				continue
			}

			var devices []string

			for _, blockDevice := range blockDevices {
				if !modelRe.MatchString(blockDevice.Model) {
					continue
				}

				if !nameRe.MatchString(blockDevice.Name) {
					continue
				}

				devices = append(devices, blockDevice.Name)
			}

			raidDevice := fmt.Sprintf("/dev/md/%s", rc.Name)
			changed, err := disks.MakeRAID0(ctx, nsc.executor, nsc.sysfsPath, raidDevice, devices, udevControlEnabled)
			if err != nil {
				nsc.eventRecorder.Eventf(
					nc,
					corev1.EventTypeWarning,
					"CreateRAIDFailed",
					"Failed to create RAID0 array from %s devices at %s: %v",
					strings.Join(devices, ","), raidDevice, err,
				)
				errs = append(errs, fmt.Errorf("can't create RAID0 array %q out of %q: %w", raidDevice, strings.Join(devices, ","), err))
				continue
			}

			nodeStatus.RaidsCreated = append(nodeStatus.RaidsCreated, raidDevice)

			if !changed {
				klog.V(4).InfoS("RAID0 array already created, nothing to do", "raidDevice", raidDevice, "devices", strings.Join(devices, ","))
				continue
			}

			klog.V(2).InfoS("RAID0 array has been created", "raidDevice", raidDevice, "devices", strings.Join(devices, ","))
			nsc.eventRecorder.Eventf(
				nc,
				corev1.EventTypeNormal,
				"RAIDCreated",
				"RAID0 array at %s using %s devices has been created",
				raidDevice, strings.Join(devices, ","),
			)
		default:
			errs = append(errs, fmt.Errorf("unsupported RAID type: %q", rc.Type))
			continue
		}
	}

	sort.Strings(nodeStatus.RaidsCreated)

	err = utilerrors.NewAggregate(errs)
	if err != nil {
		return false, fmt.Errorf("failed to create raids: %w", err)
	}

	return true, nil
}
