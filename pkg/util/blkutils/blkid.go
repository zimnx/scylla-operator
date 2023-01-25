package blkutils

import (
	"context"
	"fmt"
	"strings"

	oexec "github.com/scylladb/scylla-operator/pkg/util/exec"
	"k8s.io/utils/exec"
)

var (
	blkidFsTypeArgs = []string{
		"--output=value",
		"--match-tag=TYPE",
	}
)

func GetFilesystemType(ctx context.Context, executor exec.Interface, device string) (string, error) {
	const (
		BLKID_EXIT_NOTFOUND = 2
	)
	args := append(blkidFsTypeArgs, device)
	stdout, stderr, err := oexec.RunCommand(ctx, executor, "blkid", args...)
	if err != nil {
		exitErr, ok := err.(exec.ExitError)
		if ok && exitErr.ExitStatus() == BLKID_EXIT_NOTFOUND {
			return "", nil
		}
		return "", fmt.Errorf("failed to run blkid with args: %v: %w, stdout: %q, stderr: %q", args, err, stdout.String(), stderr.String())
	}

	return strings.TrimSpace(stdout.String()), nil
}
