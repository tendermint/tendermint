/*
Package inspect provides a tool for investigating the state of a
failed Tendermint node.

This package provides the Inspect type. The Inspect type runs a subset of the Tendermint
RPC endpoints that are useful for debugging issues with Tendermint consensus.

When a node running the Tendermint consensus engine detects an inconsistent consensus state,
the entire node will crash. The Tendermint consensus engine cannot run in this
inconsistent state so the node will not be able to start up again.

The RPC endpoints provided by the Inspect type allow for a node operator to inspect
the block store and state store to better understand what may have caused the inconsistent state.


The Inspect type's lifecycle is controlled by a context.Context
  ins := inspect.NewFromConfig(rpcConfig)
  ctx, cancelFunc:= context.WithCancel(context.Background())

  // Run blocks until the Inspect server is shut down.
  go ins.Run(ctx)
  ...

  // calling the cancel function will stop the running inspect server
  cancelFunc()

Inspect serves its RPC endpoints on the address configured in the RPC configuration

  rpcConfig.ListenAddress = "tcp://127.0.0.1:26657"
  ins := inspect.NewFromConfig(rpcConfig)
  go ins.Run(ctx)

The list of available RPC endpoints can then be viewed by navigating to
http://127.0.0.1:26657/ in the web browser.
*/
package inspect
