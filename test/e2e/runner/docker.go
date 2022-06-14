// nolint: gosec
package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"text/template"

	"github.com/tendermint/tendermint/libs/log"
	e2e "github.com/tendermint/tendermint/test/e2e/pkg"
)

// DockerInfraAPI provides an API for provisioning and manipulating
// infrastructure for Docker-based testnets.
type DockerInfraAPI struct {
	logger  log.Logger
	testnet *e2e.Testnet
}

var _ InfraAPI = &DockerInfraAPI{}

// NewDockerInfraAPI constructs an infrastructure provider that allows for
// Docker-based testnet infrastructure.
func NewDockerInfraAPI(logger log.Logger, testnet *e2e.Testnet) *DockerInfraAPI {
	return &DockerInfraAPI{
		logger:  logger,
		testnet: testnet,
	}
}

func (api *DockerInfraAPI) GenerateConfig() error {
	compose, err := makeDockerCompose(api.testnet)
	if err != nil {
		return err
	}
	err = os.WriteFile(filepath.Join(api.testnet.Dir, "docker-compose.yml"), compose, 0644)
	if err != nil {
		return err
	}
	return nil
}

func (api *DockerInfraAPI) ProvisionNode(ctx context.Context, node *e2e.Node) error {
	if err := execCompose(api.testnet.Dir, "up", "-d", node.Name); err != nil {
		return err
	}
	return nil
}

func (api *DockerInfraAPI) DisconnectNode(ctx context.Context, node *e2e.Node) error {
	if err := execDocker("network", "disconnect", api.testnet.Name+"_"+api.testnet.Name, node.Name); err != nil {
		return err
	}
	return nil
}

func (api *DockerInfraAPI) ConnectNode(ctx context.Context, node *e2e.Node) error {
	if err := execDocker("network", "connect", api.testnet.Name+"_"+api.testnet.Name, node.Name); err != nil {
		return err
	}
	return nil
}

func (api *DockerInfraAPI) KillNode(ctx context.Context, node *e2e.Node) error {
	if err := execCompose(api.testnet.Dir, "kill", "-s", "SIGKILL", node.Name); err != nil {
		return err
	}
	return nil
}

func (api *DockerInfraAPI) StartNode(ctx context.Context, node *e2e.Node) error {
	if err := execCompose(api.testnet.Dir, "start", node.Name); err != nil {
		return err
	}
	return nil
}

func (api *DockerInfraAPI) PauseNode(ctx context.Context, node *e2e.Node) error {
	if err := execCompose(api.testnet.Dir, "pause", node.Name); err != nil {
		return err
	}
	return nil
}

func (api *DockerInfraAPI) UnpauseNode(ctx context.Context, node *e2e.Node) error {
	if err := execCompose(api.testnet.Dir, "unpause", node.Name); err != nil {
		return err
	}
	return nil
}

func (api *DockerInfraAPI) TerminateNode(ctx context.Context, node *e2e.Node) error {
	if err := execCompose(api.testnet.Dir, "kill", "-s", "SIGTERM", node.Name); err != nil {
		return err
	}
	return nil
}

func (api *DockerInfraAPI) Stop(ctx context.Context) error {
	return execCompose(api.testnet.Dir, "down")
}

func (api *DockerInfraAPI) Pause(ctx context.Context) error {
	return execCompose(api.testnet.Dir, "pause")
}

func (api *DockerInfraAPI) Unpause(ctx context.Context) error {
	return execCompose(api.testnet.Dir, "unpause")
}

func (api *DockerInfraAPI) ShowLogs() error {
	return execComposeVerbose(api.testnet.Dir, "logs", "--no-color")
}

func (api *DockerInfraAPI) ShowNodeLogs(nodeID string) error {
	return execComposeVerbose(api.testnet.Dir, "logs", "--no-color", nodeID)
}

func (api *DockerInfraAPI) TailLogs() error {
	return execComposeVerbose(api.testnet.Dir, "logs", "--follow")
}

func (api *DockerInfraAPI) TailNodeLogs(nodeID string) error {
	return execComposeVerbose(api.testnet.Dir, "logs", "--follow", nodeID)
}

func (api *DockerInfraAPI) Cleanup() error {
	api.logger.Info("Removing Docker containers and networks")

	// GNU xargs requires the -r flag to not run when input is empty, macOS
	// does this by default. Ugly, but works.
	xargsR := `$(if [[ $OSTYPE == "linux-gnu"* ]]; then echo -n "-r"; fi)`

	err := exec("bash", "-c", fmt.Sprintf(
		"docker container ls -qa --filter label=e2e | xargs %v docker container rm -f", xargsR))
	if err != nil {
		return err
	}

	err = exec("bash", "-c", fmt.Sprintf(
		"docker network ls -q --filter label=e2e | xargs %v docker network rm", xargsR))
	if err != nil {
		return err
	}

	// On Linux, some local files in the volume will be owned by root since Tendermint
	// runs as root inside the container, so we need to clean them up from within a
	// container running as root too.
	absDir, err := filepath.Abs(api.testnet.Dir)
	if err != nil {
		return err
	}
	err = execDocker("run", "--rm", "--entrypoint", "", "-v", fmt.Sprintf("%v:/network", absDir),
		"tendermint/e2e-node", "sh", "-c", "rm -rf /network/*/")
	if err != nil {
		return err
	}

	return nil
}

// makeDockerCompose generates a Docker Compose config for a testnet.
func makeDockerCompose(testnet *e2e.Testnet) ([]byte, error) {
	// Must use version 2 Docker Compose format, to support IPv6.
	tmpl, err := template.New("docker-compose").Funcs(template.FuncMap{
		"addUint32": func(x, y uint32) uint32 {
			return x + y
		},
		"isBuiltin": func(protocol e2e.Protocol, mode e2e.Mode) bool {
			return mode == e2e.ModeLight || protocol == e2e.ProtocolBuiltin
		},
	}).Parse(`version: '2.4'

networks:
  {{ .Name }}:
    labels:
      e2e: true
    driver: bridge
{{- if .IPv6 }}
    enable_ipv6: true
{{- end }}
    ipam:
      driver: default
      config:
      - subnet: {{ .IP }}

services:
{{- range .Nodes }}
  {{ .Name }}:
    labels:
      e2e: true
    container_name: {{ .Name }}
    image: tendermint/e2e-node
{{- if isBuiltin $.ABCIProtocol .Mode }}
    entrypoint: /usr/bin/entrypoint-builtin
{{- else if .LogLevel }}
    command: start --log-level {{ .LogLevel }}
{{- end }}
    init: true
    ports:
    - 26656
    - {{ if .ProxyPort }}{{ addUint32 .ProxyPort 1000 }}:{{ end }}26660
    - {{ if .ProxyPort }}{{ .ProxyPort }}:{{ end }}26657
    - 6060
    volumes:
    - ./{{ .Name }}:/tendermint
    networks:
      {{ $.Name }}:
        ipv{{ if $.IPv6 }}6{{ else }}4{{ end}}_address: {{ .IP }}

{{end}}`)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	err = tmpl.Execute(&buf, testnet)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// execCompose runs a Docker Compose command for a testnet.
func execCompose(dir string, args ...string) error {
	return exec(append(
		[]string{"docker-compose", "--ansi=never", "-f", filepath.Join(dir, "docker-compose.yml")},
		args...)...)
}

// execComposeVerbose runs a Docker Compose command for a testnet and displays its output.
func execComposeVerbose(dir string, args ...string) error {
	return execVerbose(append(
		[]string{"docker-compose", "--ansi=never", "-f", filepath.Join(dir, "docker-compose.yml")},
		args...)...)
}

// execDocker runs a Docker command.
func execDocker(args ...string) error {
	return exec(append([]string{"docker"}, args...)...)
}
