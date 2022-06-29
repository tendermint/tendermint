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

// testnetInfra provides an API for provisioning and manipulating
// infrastructure for a Docker-based testnet.
type testnetInfra struct {
	logger  log.Logger
	testnet *e2e.Testnet
}

var _ infra.TestnetInfra = &testnetInfra{}

// NewTestnetInfra constructs an infrastructure provider that allows for Docker-based
// testnet infrastructure.
func NewTestnetInfra(logger log.Logger, testnet *e2e.Testnet) infra.TestnetInfra {
	return &testnetInfra{
		logger:  logger,
		testnet: testnet,
	}
}

func (ti *testnetInfra) Setup(ctx context.Context) error {
	compose, err := makeDockerCompose(ti.testnet)
	if err != nil {
		return err
	}
	// nolint: gosec
	// G306: Expect WriteFile permissions to be 0600 or less
	err = os.WriteFile(filepath.Join(ti.testnet.Dir, "docker-compose.yml"), compose, 0644)
	if err != nil {
		return err
	}
	return nil
}

func (ti *testnetInfra) StartNode(ctx context.Context, node *e2e.Node) error {
	return execCompose(ctx, ti.testnet.Dir, "up", "-d", node.Name)
}

func (ti *testnetInfra) DisconnectNode(ctx context.Context, node *e2e.Node) error {
	return execDocker(ctx, "network", "disconnect", ti.testnet.Name+"_"+ti.testnet.Name, node.Name)
}

func (ti *testnetInfra) ConnectNode(ctx context.Context, node *e2e.Node) error {
	return execDocker(ctx, "network", "connect", ti.testnet.Name+"_"+ti.testnet.Name, node.Name)
}

func (ti *testnetInfra) KillNodeProcess(ctx context.Context, node *e2e.Node) error {
	return execCompose(ctx, ti.testnet.Dir, "kill", "-s", "SIGKILL", node.Name)
}

func (ti *testnetInfra) StartNodeProcess(ctx context.Context, node *e2e.Node) error {
	return execCompose(ctx, ti.testnet.Dir, "start", node.Name)
}

func (ti *testnetInfra) PauseNodeProcess(ctx context.Context, node *e2e.Node) error {
	return execCompose(ctx, ti.testnet.Dir, "pause", node.Name)
}

func (ti *testnetInfra) UnpauseNodeProcess(ctx context.Context, node *e2e.Node) error {
	return execCompose(ctx, ti.testnet.Dir, "unpause", node.Name)
}

func (ti *testnetInfra) TerminateNodeProcess(ctx context.Context, node *e2e.Node) error {
	return execCompose(ctx, ti.testnet.Dir, "kill", "-s", "SIGTERM", node.Name)
}

func (ti *testnetInfra) Stop(ctx context.Context) error {
	return execCompose(ctx, ti.testnet.Dir, "down")
}

func (ti *testnetInfra) Pause(ctx context.Context) error {
	return execCompose(ctx, ti.testnet.Dir, "pause")
}

func (ti *testnetInfra) Unpause(ctx context.Context) error {
	return execCompose(ctx, ti.testnet.Dir, "unpause")
}

func (ti *testnetInfra) ShowLogs(ctx context.Context) error {
	return execComposeVerbose(ctx, ti.testnet.Dir, "logs", "--no-color")
}

func (ti *testnetInfra) ShowNodeLogs(ctx context.Context, node *e2e.Node) error {
	return execComposeVerbose(ctx, ti.testnet.Dir, "logs", "--no-color", node.Name)
}

func (ti *testnetInfra) TailLogs(ctx context.Context) error {
	return execComposeVerbose(ctx, ti.testnet.Dir, "logs", "--follow")
}

func (ti *testnetInfra) TailNodeLogs(ctx context.Context, node *e2e.Node) error {
	return execComposeVerbose(ctx, ti.testnet.Dir, "logs", "--follow", node.Name)
}

func (ti *testnetInfra) Cleanup(ctx context.Context) error {
	ti.logger.Info("Removing Docker containers and networks")

	// GNU xargs requires the -r flag to not run when input is empty, macOS
	// does this by default. Ugly, but works.
	xargsR := `$(if [[ $OSTYPE == "linux-gnu"* ]]; then echo -n "-r"; fi)`

	err := exec.Command(ctx, "bash", "-c", fmt.Sprintf(
		"docker container ls -qa --filter label=e2e | xargs %v docker container rm -f", xargsR))
	if err != nil {
		return err
	}

	err = exec.Command(ctx, "bash", "-c", fmt.Sprintf(
		"docker network ls -q --filter label=e2e | xargs %v docker network rm", xargsR))
	if err != nil {
		return err
	}

	// On Linux, some local files in the volume will be owned by root since Tendermint
	// runs as root inside the container, so we need to clean them up from within a
	// container running as root too.
	absDir, err := filepath.Abs(ti.testnet.Dir)
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
