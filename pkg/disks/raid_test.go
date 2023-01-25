// Copyright (c) 2023 ScyllaDB.

package disks

import (
	"context"
	"fmt"
	"os"
	"path"
	"reflect"
	"testing"

	"github.com/scylladb/scylla-operator/pkg/util/exectest"
)

func TestMakeRAID0(t *testing.T) {
	t.Parallel()

	makeNotExistingDevice := func(name string) string {
		tempDir, err := os.MkdirTemp(os.TempDir(), "local-disk-setup-raid-")
		if err != nil {
			t.Fatal(err)
		}

		devPath := path.Join(tempDir, "dev")
		err = os.MkdirAll(devPath, os.ModePerm)
		if err != nil {
			t.Fatal(err)
		}

		return path.Join(devPath, name)
	}

	makeExistingDevice := func(name string) string {
		tempDir, err := os.MkdirTemp(os.TempDir(), "local-disk-setup-raid-")
		if err != nil {
			t.Fatal(err)
		}

		devPath := path.Join(tempDir, "dev")
		err = os.MkdirAll(devPath, os.ModePerm)
		if err != nil {
			t.Fatal(err)
		}

		f, err := os.Create(path.Join(devPath, name))
		if err != nil {
			t.Fatal(err)
		}
		err = f.Close()
		if err != nil {
			t.Fatal(err)
		}

		return path.Join(devPath, name)
	}

	discardableDevice := func(sysfsPath string, name string) string {
		device := makeExistingDevice(name)

		diskQueueDir := path.Join(sysfsPath, fmt.Sprintf("/block/%s/queue", name))
		err := os.MkdirAll(diskQueueDir, os.ModePerm)
		if err != nil {
			t.Fatal(err)
		}

		err = os.WriteFile(path.Join(diskQueueDir, "discard_granularity"), []byte("1"), os.ModePerm)
		if err != nil {
			t.Fatal(err)
		}

		return device
	}

	tt := []struct {
		name             string
		raidDevice       string
		makeDevices      func(sysfsPath string) []string
		udevEnabled      bool
		expectedCommands func(string, []string) []exectest.Command
		expectedFinished bool
		expectedErr      error
	}{
		{
			name:       "nothing to do when raid device already exists",
			raidDevice: makeExistingDevice("loop0"),
			makeDevices: func(sysfsPath string) []string {
				return nil
			},
			expectedCommands: func(string, []string) []exectest.Command { return nil },
			expectedFinished: true,
			expectedErr:      nil,
		},
		{
			name:       "makes a RAID0 from provided devices",
			raidDevice: makeNotExistingDevice("md0"),
			makeDevices: func(sysfsPath string) []string {
				return []string{
					discardableDevice(sysfsPath, "nvme1n1"),
					discardableDevice(sysfsPath, "nvme2n1"),
				}
			},
			udevEnabled: true,
			expectedCommands: func(raidDevice string, devices []string) []exectest.Command {
				return []exectest.Command{
					{
						Cmd:    "lsblk",
						Args:   []string{devices[0]},
						Stdout: []byte(fmt.Sprintf(`{"blockdevices":[{"name":"%s","model":"Amazon EC2 NVMe Instance Storage","fstype":"xfs","partuuid":"1bbeb48b-101f-4d49-8ba4-67adc9878721"}]}`, devices[0])),
					},
					{
						Cmd:    "lsblk",
						Args:   []string{"--json", "--nodeps", "--paths", "--fs", "--output=NAME,MODEL,FSTYPE,PARTUUID", devices[0]},
						Stdout: []byte(fmt.Sprintf(`{"blockdevices":[{"name":"%s","model":"Amazon EC2 NVMe Instance Storage","fstype":"xfs","partuuid":"1bbeb48b-101f-4d49-8ba4-67adc9878721"}]}`, devices[0])),
					},
					{
						Cmd:  "blkdiscard",
						Args: []string{devices[0]},
					},
					{
						Cmd:    "lsblk",
						Args:   []string{devices[1]},
						Stdout: []byte(fmt.Sprintf(`{"blockdevices":[{"name":"%s","model":"Amazon EC2 NVMe Instance Storage","fstype":"xfs","partuuid":"84b7c16b-a920-4742-bcbe-7ddabe2fe063"}]}`, devices[1])),
					},
					{
						Cmd:    "lsblk",
						Args:   []string{"--json", "--nodeps", "--paths", "--fs", "--output=NAME,MODEL,FSTYPE,PARTUUID", devices[1]},
						Stdout: []byte(fmt.Sprintf(`{"blockdevices":[{"name":"%s","model":"Amazon EC2 NVMe Instance Storage","fstype":"xfs","partuuid":"84b7c16b-a920-4742-bcbe-7ddabe2fe063"}]}`, devices[1])),
					},
					{
						Cmd:  "blkdiscard",
						Args: []string{devices[1]},
					},
					{
						Cmd:  "udevadm",
						Args: []string{"settle"},
					},
					{
						Cmd:  "udevadm",
						Args: []string{"control", "--stop-exec-queue"},
					},
					{
						Cmd:  "mdadm",
						Args: []string{"--create", "--verbose", "--run", raidDevice, "--level=0", "--chunk=1024", "--raid-devices=2", devices[0], devices[1]},
					},
					{
						Cmd:  "udevadm",
						Args: []string{"control", "--start-exec-queue"},
					},
					{
						Cmd:  "udevadm",
						Args: []string{"settle"},
					},
				}
			},
			expectedFinished: true,
			expectedErr:      nil,
		},
		{
			name:       "forces a RAID0 when there's just one device",
			raidDevice: makeNotExistingDevice("md0"),
			makeDevices: func(sysfsPath string) []string {
				return []string{
					discardableDevice(sysfsPath, "nvme1n1"),
				}
			},
			udevEnabled: true,
			expectedCommands: func(raidDevice string, devices []string) []exectest.Command {
				return []exectest.Command{
					{
						Cmd:    "lsblk",
						Args:   []string{devices[0]},
						Stdout: []byte(fmt.Sprintf(`{"blockdevices":[{"name":"%s","model":"Amazon EC2 NVMe Instance Storage","fstype":"xfs","partuuid":"1bbeb48b-101f-4d49-8ba4-67adc9878721"}]}`, devices[0])),
					},
					{
						Cmd:    "lsblk",
						Args:   []string{"--json", "--nodeps", "--paths", "--fs", "--output=NAME,MODEL,FSTYPE,PARTUUID", devices[0]},
						Stdout: []byte(fmt.Sprintf(`{"blockdevices":[{"name":"%s","model":"Amazon EC2 NVMe Instance Storage","fstype":"xfs","partuuid":"1bbeb48b-101f-4d49-8ba4-67adc9878721"}]}`, devices[0])),
					},
					{
						Cmd:  "blkdiscard",
						Args: []string{devices[0]},
					},
					{
						Cmd:  "udevadm",
						Args: []string{"settle"},
					},
					{
						Cmd:  "udevadm",
						Args: []string{"control", "--stop-exec-queue"},
					},
					{
						Cmd:  "mdadm",
						Args: []string{"--create", "--verbose", "--run", raidDevice, "--level=0", "--chunk=1024", "--raid-devices=1", devices[0], "--force"},
					},
					{
						Cmd:  "udevadm",
						Args: []string{"control", "--start-exec-queue"},
					},
					{
						Cmd:  "udevadm",
						Args: []string{"settle"},
					},
				}
			},
			expectedFinished: true,
			expectedErr:      nil,
		},
		{
			name:       "skips the device discard if it doesn't requires it",
			raidDevice: makeNotExistingDevice("md0"),
			makeDevices: func(sysfsPath string) []string {
				return []string{
					makeExistingDevice("nvme1n1"),
				}
			},
			udevEnabled: true,
			expectedCommands: func(raidDevice string, devices []string) []exectest.Command {
				return []exectest.Command{
					{
						Cmd:    "lsblk",
						Args:   []string{devices[0]},
						Stdout: []byte(fmt.Sprintf(`{"blockdevices":[{"name":"%s","model":"Amazon EC2 NVMe Instance Storage","fstype":"xfs","partuuid":"1bbeb48b-101f-4d49-8ba4-67adc9878721"}]}`, devices[0])),
					},
					{
						Cmd:    "lsblk",
						Args:   []string{"--json", "--nodeps", "--paths", "--fs", "--output=NAME,MODEL,FSTYPE,PARTUUID", devices[0]},
						Stdout: []byte(fmt.Sprintf(`{"blockdevices":[{"name":"%s","model":"Amazon EC2 NVMe Instance Storage","fstype":"xfs","partuuid":"1bbeb48b-101f-4d49-8ba4-67adc9878721"}]}`, devices[0])),
					},
					{
						Cmd:  "udevadm",
						Args: []string{"settle"},
					},
					{
						Cmd:  "udevadm",
						Args: []string{"control", "--stop-exec-queue"},
					},
					{
						Cmd:  "mdadm",
						Args: []string{"--create", "--verbose", "--run", raidDevice, "--level=0", "--chunk=1024", "--raid-devices=1", devices[0], "--force"},
					},
					{
						Cmd:  "udevadm",
						Args: []string{"control", "--start-exec-queue"},
					},
					{
						Cmd:  "udevadm",
						Args: []string{"settle"},
					},
				}
			},
			expectedFinished: true,
			expectedErr:      nil,
		},
		{
			name:       "skips udev control stop/start when it's not supported",
			raidDevice: makeNotExistingDevice("md0"),
			makeDevices: func(sysfsPath string) []string {
				return []string{
					discardableDevice(sysfsPath, "nvme1n1"),
				}
			},
			udevEnabled: false,
			expectedCommands: func(raidDevice string, devices []string) []exectest.Command {
				return []exectest.Command{
					{
						Cmd:    "lsblk",
						Args:   []string{devices[0]},
						Stdout: []byte(fmt.Sprintf(`{"blockdevices":[{"name":"%s","model":"Amazon EC2 NVMe Instance Storage","fstype":"xfs","partuuid":"1bbeb48b-101f-4d49-8ba4-67adc9878721"}]}`, devices[0])),
					},
					{
						Cmd:    "lsblk",
						Args:   []string{"--json", "--nodeps", "--paths", "--fs", "--output=NAME,MODEL,FSTYPE,PARTUUID", devices[0]},
						Stdout: []byte(fmt.Sprintf(`{"blockdevices":[{"name":"%s","model":"Amazon EC2 NVMe Instance Storage","fstype":"xfs","partuuid":"1bbeb48b-101f-4d49-8ba4-67adc9878721"}]}`, devices[0])),
					},
					{
						Cmd:  "blkdiscard",
						Args: []string{devices[0]},
					},
					{
						Cmd:  "mdadm",
						Args: []string{"--create", "--verbose", "--run", raidDevice, "--level=0", "--chunk=1024", "--raid-devices=1", devices[0], "--force"},
					},
				}
			},
			expectedFinished: true,
			expectedErr:      nil,
		},
	}

	for i := range tt {
		tc := tt[i]
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			sysfsPath, err := os.MkdirTemp(os.TempDir(), "local-disk-setup-raid-sysfs")
			if err != nil {
				t.Fatal(err)
			}
			defer os.RemoveAll(sysfsPath)

			devices := tc.makeDevices(sysfsPath)
			expectedCommands := tc.expectedCommands(tc.raidDevice, devices)
			executor := exectest.NewFakeExec(expectedCommands...)

			finished, err := MakeRAID0(ctx, executor, sysfsPath, tc.raidDevice, devices, tc.udevEnabled)
			if !reflect.DeepEqual(err, tc.expectedErr) {
				t.Fatalf("expected %v error, got %v", tc.expectedErr, err)
			}
			if !reflect.DeepEqual(finished, tc.expectedFinished) {
				t.Fatalf("expected %v finished, got %v", tc.expectedFinished, err)
			}
			if executor.CommandCalls != len(expectedCommands) {
				t.Fatalf("expected %d command calls, got %d", len(expectedCommands), executor.CommandCalls)
			}
		})
	}
}
