package rpc

import (
	"net/http"
)

func initHandlers() {
	http.HandleFunc("/blockchain", BlockchainInfoHandler)
	http.HandleFunc("/get_block", GetBlockHandler)
	http.HandleFunc("/broadcast_tx", BroadcastTxHandler)
	http.HandleFunc("/gen_priv_account", GenPrivAccountHandler)
	http.HandleFunc("/get_account", GetAccountHandler)
	http.HandleFunc("/list_accounts", ListAccountsHandler)
	http.HandleFunc("/sign_tx", SignTxHandler)
	http.HandleFunc("/list_validators", ListValidatorsHandler)
}
