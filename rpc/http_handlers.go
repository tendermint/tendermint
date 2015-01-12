package rpc

import (
	"net/http"
)

func initHandlers() {
	http.HandleFunc("/blockchain", BlockchainInfoHandler)
	http.HandleFunc("/block", BlockHandler)
	http.HandleFunc("/broadcast_tx", BroadcastTxHandler)
	http.HandleFunc("/gen_priv_account", GenPrivAccountHandler)
	http.HandleFunc("/sign_send_tx", SignSendTxHandler)
	http.HandleFunc("/accounts", ListAccountsHandler)
}
