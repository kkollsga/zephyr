package git

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"time"
)

const defaultTimeout = 10 * time.Second

// Run executes a git command in the given directory and returns stdout.
func Run(dir string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	cmd.Env = append(cmd.Environ(),
		"GIT_OPTIONAL_LOCKS=0",
		"LC_ALL=C",
	)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("git %v: %w: %s", args, err, stderr.String())
	}
	return stdout.Bytes(), nil
}

// RunSilent executes a git command where stdout is not needed.
func RunSilent(dir string, args ...string) error {
	_, err := Run(dir, args...)
	return err
}
