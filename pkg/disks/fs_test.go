// Copyright (c) 2023 ScyllaDB.

package disks

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	"github.com/scylladb/scylla-operator/pkg/util/exectest"
	testingexec "k8s.io/utils/exec/testing"
)

func TestMakeFS(t *testing.T) {
	t.Parallel()

	tt := []struct {
		name             string
		device           string
		blockSize        int
		fsType           string
		expectedCommands []exectest.Command
		expectedFinished bool
		expectedErr      error
	}{
		{
			name:      "nothing to do if device is already formatted to required filesystem",
			device:    "/dev/md0",
			blockSize: 1024,
			fsType:    "xfs",
			expectedCommands: []exectest.Command{
				{
					Cmd:    "blkid",
					Args:   []string{"--output=value", "--match-tag=TYPE", "/dev/md0"},
					Stdout: []byte(`xfs`),
					Stderr: nil,
					Err:    nil,
				},
			},
			expectedFinished: true,
			expectedErr:      nil,
		},
		{
			name:      "fail if cannot determine existing filesystem type",
			device:    "/dev/md0",
			blockSize: 1024,
			fsType:    "xfs",
			expectedCommands: []exectest.Command{
				{
					Cmd:    "blkid",
					Args:   []string{"--output=value", "--match-tag=TYPE", "/dev/md0"},
					Stdout: nil,
					Stderr: nil,
					Err:    testingexec.FakeExitError{Status: 42},
				},
			},
			expectedFinished: false,
			expectedErr:      fmt.Errorf(`can't determine existing filesystem type at "/dev/md0": %w`, fmt.Errorf(`failed to run blkid with args: [--output=value --match-tag=TYPE /dev/md0]: %w, stdout: "", stderr: ""`, testingexec.FakeExitError{Status: 42})),
		},
		{
			name:      "format the device if existing disk filesystem is empty",
			device:    "/dev/md0",
			blockSize: 1024,
			fsType:    "xfs",
			expectedCommands: []exectest.Command{
				{
					Cmd:    "blkid",
					Args:   []string{"--output=value", "--match-tag=TYPE", "/dev/md0"},
					Stdout: []byte(``),
					Stderr: nil,
					Err:    nil,
				},
				{
					Cmd:    "mkfs",
					Args:   []string{"-t", "xfs", "-b", "size=1024", "-K", "/dev/md0"},
					Stdout: nil,
					Stderr: nil,
					Err:    nil,
				},
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

			executor := exectest.NewFakeExec(tc.expectedCommands...)

			finished, err := MakeFS(ctx, executor, tc.device, tc.blockSize, tc.fsType)
			if !reflect.DeepEqual(err, tc.expectedErr) {
				t.Fatalf("expected %v error, got %v", tc.expectedErr, err)
			}
			if !reflect.DeepEqual(finished, tc.expectedFinished) {
				t.Fatalf("expected finished %v, got %v", tc.expectedFinished, finished)
			}
			if executor.CommandCalls != len(tc.expectedCommands) {
				t.Fatalf("expected %d command calls, got %d", len(tc.expectedCommands), executor.CommandCalls)
			}
		})
	}
}
