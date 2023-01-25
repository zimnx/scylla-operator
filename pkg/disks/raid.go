// Copyright (c) 2023 ScyllaDB.

package disks

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	oexec "github.com/scylladb/scylla-operator/pkg/util/exec"
	"k8s.io/apimachinery/pkg/api/equality"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/utils/exec"
)

func MakeRAID0(ctx context.Context, executor exec.Interface, sysfsPath string, raidDevice string, devices []string, udevControlEnabled bool) (changed bool, err error) {
	if _, err := os.Stat(raidDevice); !os.IsNotExist(err) {
		mdLevel, slaves, err := getRAIDInfo(sysfsPath, raidDevice)
		if err != nil {
			return false, fmt.Errorf("can't get raid info about %q device: %w", raidDevice, err)
		}

		if mdLevel != "raid0" {
			return false, fmt.Errorf(`expected "raid0" md level of existing raid device, got %q`, mdLevel)
		}

		deviceNames := make([]string, 0, len(devices))
		for _, d := range devices {
			deviceNames = append(deviceNames, path.Base(d))
		}

		sort.Strings(deviceNames)
		sort.Strings(slaves)

		if !equality.Semantic.DeepEqual(slaves, deviceNames) {
			return false, fmt.Errorf("existing raid device at %q consists of %q devices, expected %q", raidDevice, strings.Join(slaves, ","), strings.Join(deviceNames, ","))
		}

		return false, nil
	}

	for _, device := range devices {
		deviceName := path.Base(device)
		if _, err := os.Stat(device); os.IsNotExist(err) {
			return false, fmt.Errorf("device %q does not exists", deviceName)
		}

		// Before array is created, devices must be discarded, otherwise raid creation would fail.
		// Not all device types support it, we discard them only if discard_granularity is set.
		discardPath := path.Join(sysfsPath, fmt.Sprintf("/block/%s/queue/discard_granularity", deviceName))
		if _, err := os.Stat(discardPath); !os.IsNotExist(err) {
			discard, err := os.ReadFile(discardPath)
			if err != nil {
				return false, fmt.Errorf("can't read discard file at %q: %w", discardPath, err)
			}

			if strings.TrimSpace(string(discard)) != "0" {
				if stdout, stderr, err := oexec.RunCommand(ctx, executor, "blkdiscard", device); err != nil {
					return false, fmt.Errorf("can't discard device %q: %w, stdout: %q, stderr: %q", deviceName, err, stdout.String(), stderr.String())
				}
			}
		}
	}

	if udevControlEnabled {
		// Settle udev events and temporarily stop exec queue to prevent contention on device handles.
		if stdout, stderr, err := oexec.RunCommand(ctx, executor, "udevadm", "settle"); err != nil {
			return false, fmt.Errorf("can't run udevadm settle: %w, stdout: %q, stderr: %q", err, stdout.String(), stderr.String())
		}

		if stdout, stderr, err := oexec.RunCommand(ctx, executor, "udevadm", "control", "--stop-exec-queue"); err != nil {
			return false, fmt.Errorf("can't stop udevadm exec queue: %w, stdout: %q, stderr: %q", err, stdout.String(), stderr.String())
		}
		defer func() {
			// It's reentrant.
			stdout, stderr, startErr := oexec.RunCommand(ctx, executor, "udevadm", "control", "--start-exec-queue")
			if startErr != nil {
				err = utilerrors.NewAggregate([]error{err, fmt.Errorf("can't start udevadm exec queue: %w, stdout: %q, stderr: %q", startErr, stdout.String(), stderr.String())})
			}
		}()
	}

	createRaidArgs := []string{
		"--create",
		"--verbose",
		"--run",
		raidDevice,
		"--level=0",
		"--chunk=1024",
		fmt.Sprintf("--raid-devices=%d", len(devices)),
	}

	createRaidArgs = append(createRaidArgs, devices...)

	if len(devices) == 1 {
		createRaidArgs = append(createRaidArgs, "--force")
	}

	if stdout, stderr, err := oexec.RunCommand(ctx, executor, "mdadm", createRaidArgs...); err != nil {
		return false, fmt.Errorf("can't run mdadm with args %q: %w, stdout: %q, stderr: %q", createRaidArgs, err, stdout.String(), stderr.String())
	}

	if udevControlEnabled {
		// Enable udev exec queue again and wait for settle.
		if stdout, stderr, err := oexec.RunCommand(ctx, executor, "udevadm", "control", "--start-exec-queue"); err != nil {
			return false, fmt.Errorf("can't stop udevadm exec queue: %w, stdout: %q, stderr: %q", err, stdout.String(), stderr.String())
		}

		if stdout, stderr, err := oexec.RunCommand(ctx, executor, "udevadm", "settle"); err != nil {
			return false, fmt.Errorf("can't run udevadm settle: %w, stdout: %q, stderr: %q", err, stdout.String(), stderr.String())
		}
	}

	return true, nil
}

func getRAIDInfo(sysfsPath, device string) (string, []string, error) {
	realDevice, err := filepath.EvalSymlinks(device)
	if err != nil {
		return "", nil, fmt.Errorf("can't evaluate device %q symlink: %w", device, err)
	}

	mdLevelRaw, err := os.ReadFile(path.Join(sysfsPath, fmt.Sprintf("/block/%s/md/level", path.Base(realDevice))))
	if err != nil {
		return "", nil, fmt.Errorf("can't check existing raid device md level: %w", err)
	}

	mdLevel := strings.TrimSpace(string(mdLevelRaw))

	var slaves []string
	slavesDirPath := path.Join(sysfsPath, fmt.Sprintf("/block/%s/slaves", path.Base(realDevice)))

	err = filepath.WalkDir(slavesDirPath, func(f string, de fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if f == slavesDirPath {
			return nil
		}

		if de.IsDir() {
			return filepath.SkipDir
		}

		if de.Type()&fs.ModeSymlink == fs.ModeSymlink {
			slaves = append(slaves, path.Base(f))
		}
		return nil
	})
	if err != nil {
		return "", nil, fmt.Errorf("can't determine raid slaves: %w", err)
	}

	return mdLevel, slaves, nil
}
