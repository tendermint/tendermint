package docker

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/tendermint/tendermint/libs/log"
	e2e "github.com/tendermint/tendermint/test/e2e/pkg"
	"github.com/tendermint/tendermint/test/e2e/pkg/exec"
	"github.com/tendermint/tendermint/test/e2e/pkg/infra"
)

// Provider provides an API for provisioning and manipulating infrastructure
// for Docker-based testnets.
type Provider struct {
	logger  log.Logger
	testnet *e2e.Testnet
}

var _ infra.Provider = &Provider{}

// NewProvider constructs an infrastructure provider that allows for Docker-based
// testnet infrastructure.
func NewProvider(logger log.Logger, testnet *e2e.Testnet) *Provider {
	return &Provider{
		logger:  logger,
		testnet: testnet,
	}
}

func (p *Provider) Setup(ctx context.Context) error {
	compose, err := makeDockerCompose(p.testnet)
	if err != nil {
		return err
	}
	// nolint: gosec
	// G306: Expect WriteFile permissions to be 0600 or less
	err = os.WriteFile(filepath.Join(p.testnet.Dir, "docker-compose.yml"), compose, 0644)
	if err != nil {
		return err
	}
	return nil
}

func (p *Provider) StartNode(ctx context.Context, node *e2e.Node) error {
	return execCompose(ctx, p.testnet.Dir, "up", "-d", node.Name)
}

func (p *Provider) DisconnectNode(ctx context.Context, node *e2e.Node) error {
	return execDocker(ctx, "network", "disconnect", p.testnet.Name+"_"+p.testnet.Name, node.Name)
}

func (p *Provider) ConnectNode(ctx context.Context, node *e2e.Node) error {
	return execDocker(ctx, "network", "connect", p.testnet.Name+"_"+p.testnet.Name, node.Name)
}

func (p *Provider) KillNodeProcess(ctx context.Context, node *e2e.Node) error {
	return execCompose(ctx, p.testnet.Dir, "kill", "-s", "SIGKILL", node.Name)
}

func (p *Provider) StartNodeProcess(ctx context.Context, node *e2e.Node) error {
	return execCompose(ctx, p.testnet.Dir, "start", node.Name)
}

func (p *Provider) PauseNodeProcess(ctx context.Context, node *e2e.Node) error {
	return execCompose(ctx, p.testnet.Dir, "pause", node.Name)
}

func (p *Provider) UnpauseNodeProcess(ctx context.Context, node *e2e.Node) error {
	return execCompose(ctx, p.testnet.Dir, "unpause", node.Name)
}

func (p *Provider) TerminateNodeProcess(ctx context.Context, node *e2e.Node) error {
	return execCompose(ctx, p.testnet.Dir, "kill", "-s", "SIGTERM", node.Name)
}

func (p *Provider) Stop(ctx context.Context) error {
	return execCompose(ctx, p.testnet.Dir, "down")
}

func (p *Provider) Pause(ctx context.Context) error {
	return execCompose(ctx, p.testnet.Dir, "pause")
}

func (p *Provider) Unpause(ctx context.Context) error {
	return execCompose(ctx, p.testnet.Dir, "unpause")
}

func (p *Provider) ShowLogs(ctx context.Context) error {
	return execComposeVerbose(ctx, p.testnet.Dir, "logs", "--no-color")
}

func (p *Provider) ShowNodeLogs(ctx context.Context, node *e2e.Node) error {
	return execComposeVerbose(ctx, p.testnet.Dir, "logs", "--no-color", node.Name)
}

func (p *Provider) TailLogs(ctx context.Context) error {
	return execComposeVerbose(ctx, p.testnet.Dir, "logs", "--follow")
}

func (p *Provider) TailNodeLogs(ctx context.Context, node *e2e.Node) error {
	return execComposeVerbose(ctx, p.testnet.Dir, "logs", "--follow", node.Name)
}

func (p *Provider) Cleanup(ctx context.Context) error {
	p.logger.Info("Removing Docker containers and networks")

	// GNU xargs requires the -r flag to not run when input is empty, macOS
	// does this by default. Ugly, but works.
	xargsR := `$(if [[ $OSTYPE == "linux-gnu"* ]]; then echo -n "-r"; fi)`

	err := exec.Exec(ctx, "bash", "-c", fmt.Sprintf(
		"docker container ls -qa --filter label=e2e | xargs %v docker container rm -f", xargsR))
	if err != nil {
		return err
	}

	err = exec.Exec(ctx, "bash", "-c", fmt.Sprintf(
		"docker network ls -q --filter label=e2e | xargs %v docker network rm", xargsR))
	if err != nil {
		return err
	}

	// On Linux, some local files in the volume will be owned by root since Tendermint
	// runs as root inside the container, so we need to clean them up from within a
	// container running as root too.
	absDir, err := filepath.Abs(p.testnet.Dir)
	if err != nil {
		return err
	}
	err = execDocker(ctx, "run", "--rm", "--entrypoint", "", "-v", fmt.Sprintf("%v:/network", absDir),
		"tendermint/e2e-node", "sh", "-c", "rm -rf /network/*/")
	if err != nil {
		return err
	}

	return nil
}
