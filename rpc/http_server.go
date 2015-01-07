package rpc

import (
	"net/http"

	. "github.com/tendermint/tendermint/common"
	. "github.com/tendermint/tendermint/config"
)

func StartHTTPServer() {

	http.HandleFunc("/blockchain", BlockchainInfoHandler)
	http.HandleFunc("/block", BlockHandler)
	http.HandleFunc("/broadcast_tx", BroadcastTxHandler)

	log.Info(Fmt("Starting RPC HTTP server on %s", Config.RPC.HTTPLAddr))

	go func() {
		log.Crit("RPC HTTPServer stopped", "result", http.ListenAndServe(Config.RPC.HTTPLAddr, RecoverAndLogHandler(http.DefaultServeMux)))
	}()
}
