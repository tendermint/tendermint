package main

import (
	"context"

	e2e "github.com/tendermint/tendermint/test/e2e/pkg"
)

// TestnetInfra provides an API for interacting with the infrastructure for an
// entire testnet.
type TestnetInfra interface {
	// Setup generates any necessary configuration for the
	// infrastructure provider during testnet setup.
	Setup(ctx context.Context) error

	// Stop attempts to stop the entire testnet.
	Stop(ctx context.Context) error

	// Pause attempts to pause the entire testnet.
	Pause(ctx context.Context) error

	// Unpause attempts to resume a paused testnet.
	Unpause(ctx context.Context) error

	// ShowLogs will print all logs for the whole testnet to stdout.
	ShowLogs(ctx context.Context) error

	// TailLogs tails the logs for all nodes in the testnet.
	TailLogs(ctx context.Context) error

	// Cleanup stops and destroys all running infrastructure and deletes any
	// generated files.
	Cleanup(ctx context.Context) error
}

// NodeInfra provides an API for interacting with specific nodes'
// infrastructure.
type NodeInfra interface {
	// ProvisionNode attempts to provision infrastructure for the given node
	// and starts it.
	ProvisionNode(ctx context.Context, node *e2e.Node) error

	// DisconnectNode attempts to ensure that the given node is disconnected
	// from the network.
	DisconnectNode(ctx context.Context, node *e2e.Node) error

	// ConnectNode attempts to ensure that the given node is connected to the
	// network.
	ConnectNode(ctx context.Context, node *e2e.Node) error

	// KillNode attempts to ensure that the given node's process is killed
	// immediately using SIGKILL.
	KillNode(ctx context.Context, node *e2e.Node) error

	// StartNode attempts to start a node's process. Assumes that the node's
	// infrastructure has previously been provisioned using ProvisionNode.
	StartNode(ctx context.Context, node *e2e.Node) error

	// PauseNode attempts to pause a node's process.
	PauseNode(ctx context.Context, node *e2e.Node) error

	// UnpauseNode attempts to resume a paused node's process.
	UnpauseNode(ctx context.Context, node *e2e.Node) error

	// TerminateNode attempts to ensure that the given node's process is
	// terminated using SIGTERM.
	TerminateNode(ctx context.Context, node *e2e.Node) error

	// ShowNodeLogs will print all logs for the node with the give ID to
	// stdout.
	ShowNodeLogs(ctx context.Context, nodeID string) error

	// TailNodeLogs tails the logs for a single node.
	TailNodeLogs(ctx context.Context, nodeID string) error
}

// Infra provides an API for interacting with testnet- and node-level
// infrastructure.
type Infra interface {
	TestnetInfra
	NodeInfra
}
