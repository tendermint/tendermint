package infra

import (
	"context"

	e2e "github.com/tendermint/tendermint/test/e2e/pkg"
)

// TestnetInfra provides an API for manipulating the infrastructure of a
// specific testnet.
type TestnetInfra interface {
	//
	// Overarching testnet infrastructure management.
	//

	// Setup generates any necessary configuration for the infrastructure
	// provider during testnet setup.
	Setup(ctx context.Context) error

	// Stop will stop all running processes throughout the testnet without
	// destroying any infrastructure.
	Stop(ctx context.Context) error

	// Pause will pause all processes in the testnet.
	Pause(ctx context.Context) error

	// Unpause will resume a paused testnet.
	Unpause(ctx context.Context) error

	// ShowLogs prints all logs for the whole testnet to stdout.
	ShowLogs(ctx context.Context) error

	// TailLogs tails the logs for all nodes in the testnet, if this is
	// supported by the infrastructure provider.
	TailLogs(ctx context.Context) error

	// Cleanup stops and destroys all running testnet infrastructure and
	// deletes any generated files.
	Cleanup(ctx context.Context) error

	//
	// Node management, including node infrastructure.
	//

	// StartNode provisions infrastructure for the given node and starts it.
	StartNode(ctx context.Context, node *e2e.Node) error

	// DisconnectNode modifies the specified node's network configuration such
	// that it becomes bidirectionally disconnected from the network (it cannot
	// see other nodes, and other nodes cannot see it).
	DisconnectNode(ctx context.Context, node *e2e.Node) error

	// ConnectNode modifies the specified node's network configuration such
	// that it can become bidirectionally connected.
	ConnectNode(ctx context.Context, node *e2e.Node) error

	// ShowNodeLogs prints all logs for the node with the give ID to stdout.
	ShowNodeLogs(ctx context.Context, node *e2e.Node) error

	// TailNodeLogs tails the logs for a single node, if this is supported by
	// the infrastructure provider.
	TailNodeLogs(ctx context.Context, node *e2e.Node) error

	//
	// Node process management.
	//

	// KillNodeProcess sends SIGKILL to a node's process.
	KillNodeProcess(ctx context.Context, node *e2e.Node) error

	// StartNodeProcess will start a stopped node's process. Assumes that the
	// node's infrastructure has previously been provisioned using
	// ProvisionNode.
	StartNodeProcess(ctx context.Context, node *e2e.Node) error

	// PauseNodeProcess sends a signal to the node's process to pause it.
	PauseNodeProcess(ctx context.Context, node *e2e.Node) error

	// UnpauseNodeProcess resumes a paused node's process.
	UnpauseNodeProcess(ctx context.Context, node *e2e.Node) error

	// TerminateNodeProcess sends SIGTERM to a node's process.
	TerminateNodeProcess(ctx context.Context, node *e2e.Node) error
}
