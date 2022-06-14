package docker

import (
	"context"
	"path/filepath"

	"github.com/tendermint/tendermint/test/e2e/pkg/exec"
)

// execCompose runs a Docker Compose command for a testnet.
func execCompose(ctx context.Context, dir string, args ...string) error {
	return exec.Exec(ctx, append(
		[]string{"docker-compose", "--ansi=never", "-f", filepath.Join(dir, "docker-compose.yml")},
		args...)...)
}

// execComposeVerbose runs a Docker Compose command for a testnet and displays its output.
func execComposeVerbose(ctx context.Context, dir string, args ...string) error {
	return exec.ExecVerbose(ctx, append(
		[]string{"docker-compose", "--ansi=never", "-f", filepath.Join(dir, "docker-compose.yml")},
		args...)...)
}

// execDocker runs a Docker command.
func execDocker(ctx context.Context, args ...string) error {
	return exec.Exec(ctx, append([]string{"docker"}, args...)...)
}
