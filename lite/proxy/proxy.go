package proxy

import (
	"net/http"

	amino "github.com/tendermint/go-amino"
	"github.com/tendermint/tendermint/libs/log"

	rpcclient "github.com/tendermint/tendermint/rpc/client"
	"github.com/tendermint/tendermint/rpc/core"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
	rpcserver "github.com/tendermint/tendermint/rpc/lib/server"
)

const (
	wsEndpoint = "/websocket"
)

// StartProxy will start the websocket manager on the client,
// set up the rpc routes to proxy via the given client,
// and start up an http/rpc server on the location given by bind (eg. :1234)
// NOTE: This function blocks - you may want to call it in a go-routine.
func StartProxy(c rpcclient.Client, listenAddr string, logger log.Logger, maxOpenConnections int) error {
	err := c.Start()
	if err != nil {
		return err
	}

	cdc := amino.NewCodec()
	ctypes.RegisterAmino(cdc)
	r := RPCRoutes(c)

	// build the handler...
	mux := http.NewServeMux()
	rpcserver.RegisterRPCFuncs(mux, r, cdc, logger)

	wm := rpcserver.NewWebsocketManager(r, cdc, rpcserver.EventSubscriber(c))
	wm.SetLogger(logger)
	core.SetLogger(logger)
	mux.HandleFunc(wsEndpoint, wm.WebsocketHandler)

	l, err := rpcserver.Listen(listenAddr, rpcserver.Config{MaxOpenConnections: maxOpenConnections})
	if err != nil {
		return err
	}
	return rpcserver.StartHTTPServer(l, mux, logger)
}

// RPCRoutes just routes everything to the given client, as if it were
// a tendermint fullnode.
//
// if we want security, the client must implement it as a secure client
func RPCRoutes(c rpcclient.Client) map[string]*rpcserver.RPCFunc {

	return map[string]*rpcserver.RPCFunc{
		// Subscribe/unsubscribe are reserved for websocket events.
		// We can just use the core tendermint impl, which uses the
		// EventSwitch we registered in NewWebsocketManager above
		"subscribe":   rpcserver.NewWSRPCFunc(core.Subscribe, "query"),
		"unsubscribe": rpcserver.NewWSRPCFunc(core.Unsubscribe, "query"),

		// info API
		"status":     rpcserver.NewRPCFunc(c.Status, ""),
		"blockchain": rpcserver.NewRPCFunc(c.BlockchainInfo, "minHeight,maxHeight"),
		"genesis":    rpcserver.NewRPCFunc(c.Genesis, ""),
		"block":      rpcserver.NewRPCFunc(c.Block, "height"),
		"commit":     rpcserver.NewRPCFunc(c.Commit, "height"),
		"tx":         rpcserver.NewRPCFunc(c.Tx, "hash,prove"),
		"validators": rpcserver.NewRPCFunc(c.Validators, ""),

		// broadcast API
		"broadcast_tx_commit": rpcserver.NewRPCFunc(c.BroadcastTxCommit, "tx"),
		"broadcast_tx_sync":   rpcserver.NewRPCFunc(c.BroadcastTxSync, "tx"),
		"broadcast_tx_async":  rpcserver.NewRPCFunc(c.BroadcastTxAsync, "tx"),

		// abci API
		"abci_query": rpcserver.NewRPCFunc(c.ABCIQuery, "path,data,prove"),
		"abci_info":  rpcserver.NewRPCFunc(c.ABCIInfo, ""),
	}
}
