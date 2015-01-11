package rpc

import (
	"net/http"

	. "github.com/tendermint/tendermint/common"
	"github.com/tendermint/tendermint/config"
)

func StartHTTPServer() {

	http.HandleFunc("/blockchain", BlockchainInfoHandler)
	http.HandleFunc("/block", BlockHandler)
	http.HandleFunc("/broadcast_tx", BroadcastTxHandler)
	http.HandleFunc("/gen_priv_account", GenPrivAccountHandler)
	http.HandleFunc("/sign_send_tx", SignSendTxHandler)
	http.HandleFunc("/accounts", ListAccountsHandler)

	log.Info(Fmt("Starting RPC HTTP server on %s", config.App.GetString("RPC.HTTP.ListenAddr")))

	go func() {
		log.Crit("RPC HTTPServer stopped", "result", http.ListenAndServe(config.App.GetString("RPC.HTTP.ListenAddr"), RecoverAndLogHandler(http.DefaultServeMux)))
	}()
}
