package client

import (
	data "github.com/tendermint/go-wire/data"
	nm "github.com/tendermint/tendermint/node"
	"github.com/tendermint/tendermint/rpc/core"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
	"github.com/tendermint/tendermint/types"
)

/*
Local is a Client implementation that directly executes the rpc
functions on a given node, without going through HTTP or GRPC

This implementation is useful for:

* Running tests against a node in-process without the overhead
of going through an http server
* Communication between an ABCI app and tendermin core when they
are compiled in process.

For real clients, you probably want to use client.HTTP.  For more
powerful control during testing, you probably want the "client/mock" package.
*/
type Local struct {
	node *nm.Node
	types.EventSwitch
}

// NewLocal configures a client that calls the Node directly.
//
// Note that given how rpc/core works with package singletons, that
// you can only have one node per process.  So make sure test cases
// don't run in parallel, or try to simulate an entire network in
// one process...
func NewLocal(node *nm.Node) Local {
	node.ConfigureRPC()
	return Local{
		node:        node,
		EventSwitch: node.EventSwitch(),
	}
}

func (c Local) _assertIsClient() Client {
	return c
}

func (c Local) _assertIsNetworkClient() NetworkClient {
	return c
}

func (c Local) Status() (*ctypes.ResultStatus, error) {
	return core.Status()
}

func (c Local) ABCIInfo() (*ctypes.ResultABCIInfo, error) {
	return core.ABCIInfo()
}

func (c Local) ABCIQuery(path string, data data.Bytes, prove bool) (*ctypes.ResultABCIQuery, error) {
	return core.ABCIQuery(path, data, prove)
}

func (c Local) BroadcastTxCommit(tx types.Tx) (*ctypes.ResultBroadcastTxCommit, error) {
	return core.BroadcastTxCommit(tx)
}

func (c Local) BroadcastTxAsync(tx types.Tx) (*ctypes.ResultBroadcastTx, error) {
	return core.BroadcastTxAsync(tx)
}

func (c Local) BroadcastTxSync(tx types.Tx) (*ctypes.ResultBroadcastTx, error) {
	return core.BroadcastTxSync(tx)
}

func (c Local) NetInfo() (*ctypes.ResultNetInfo, error) {
	return core.NetInfo()
}

func (c Local) DumpConsensusState() (*ctypes.ResultDumpConsensusState, error) {
	return core.DumpConsensusState()
}

func (c Local) DialSeeds(seeds []string) (*ctypes.ResultDialSeeds, error) {
	return core.UnsafeDialSeeds(seeds)
}

func (c Local) BlockchainInfo(minHeight, maxHeight int) (*ctypes.ResultBlockchainInfo, error) {
	return core.BlockchainInfo(minHeight, maxHeight)
}

func (c Local) Genesis() (*ctypes.ResultGenesis, error) {
	return core.Genesis()
}

func (c Local) Block(height int) (*ctypes.ResultBlock, error) {
	return core.Block(height)
}

func (c Local) Commit(height int) (*ctypes.ResultCommit, error) {
	return core.Commit(height)
}

func (c Local) Validators() (*ctypes.ResultValidators, error) {
	return core.Validators()
}

func (c Local) Tx(hash []byte, prove bool) (*ctypes.ResultTx, error) {
	return core.Tx(hash, prove)
}
