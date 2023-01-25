// Copyright (c) 2023 ScyllaDB.

package blkutils_test

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	"github.com/scylladb/scylla-operator/pkg/util/blkutils"
	"github.com/scylladb/scylla-operator/pkg/util/exectest"
	testingexec "k8s.io/utils/exec/testing"
)

func TestGetFilesystemType(t *testing.T) {
	t.Parallel()

	tt := []struct {
		name                   string
		device                 string
		commands               []exectest.Command
		expectedFilesystemType string
		expectedError          error
	}{
		{
			name:   "blkid fails with random error",
			device: "/dev/md0",
			commands: []exectest.Command{
				{
					Cmd:    "blkid",
					Args:   []string{"--output=value", "--match-tag=TYPE", "/dev/md0"},
					Stderr: []byte("some random error"),
					Err:    testingexec.FakeExitError{Status: 666},
				},
			},
			expectedFilesystemType: "",
			expectedError:          fmt.Errorf(`failed to run blkid with args: [--output=value --match-tag=TYPE /dev/md0]: %w, stderr: "some random error"`, testingexec.FakeExitError{Status: 666}),
		},
		{
			name:   "returns filesystem type",
			device: "/dev/md0",
			commands: []exectest.Command{
				{
					Cmd:    "blkid",
					Args:   []string{"--output=value", "--match-tag=TYPE", "/dev/md0"},
					Stdout: []byte("xfs"),
				},
			},
			expectedFilesystemType: "xfs",
			expectedError:          nil,
		},
		{
			name:   "empty filesystem on special error code",
			device: "/dev/md0",
			commands: []exectest.Command{
				{
					Cmd:    "blkid",
					Args:   []string{"--output=value", "--match-tag=TYPE", "/dev/md0"},
					Stderr: []byte("some random error"),
					Err:    testingexec.FakeExitError{Status: 2},
				},
			},
			expectedFilesystemType: "",
			expectedError:          nil,
		},
	}

	for i := range tt {
		tc := tt[i]
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			executor := exectest.NewFakeExec(tc.commands...)
			fsType, err := blkutils.GetFilesystemType(ctx, executor, tc.device)
			if !reflect.DeepEqual(err, tc.expectedError) {
				t.Fatalf("expected %v error, got %v", tc.expectedError, err)
			}
			if !reflect.DeepEqual(fsType, tc.expectedFilesystemType) {
				t.Fatalf("expected %v, got %v", tc.expectedFilesystemType, fsType)
			}
		})
	}
}
