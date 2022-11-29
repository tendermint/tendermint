package main

import (
	"path/filepath"

	"github.com/tendermint/tendermint/test/e2e/pkg/exec"
	"golang.org/x/net/context"
)

// execCompose runs a Docker Compose command for a testnet.
func execCompose(dir string, args ...string) error {
	return exec.Command(context.Background(), append(
		[]string{"docker-compose", "-f", filepath.Join(dir, "docker-compose.yml")},
		args...)...)
}

// execComposeVerbose runs a Docker Compose command for a testnet and displays its output.
func execComposeVerbose(dir string, args ...string) error {
	return exec.CommandVerbose(context.Background(), append(
		[]string{"docker-compose", "-f", filepath.Join(dir, "docker-compose.yml")},
		args...)...)
}

// execDocker runs a Docker command.
func execDocker(args ...string) error {
	return exec.Command(context.Background(), append([]string{"docker"}, args...)...)
}
