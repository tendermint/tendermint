package rpc

import (
	"net/http"
)

func initHandlers() {
	http.HandleFunc("/status", StatusHandler)
	http.HandleFunc("/net_info", NetInfoHandler)
	http.HandleFunc("/blockchain", BlockchainInfoHandler)
	http.HandleFunc("/get_block", GetBlockHandler)
	http.HandleFunc("/get_account", GetAccountHandler)
	http.HandleFunc("/list_validators", ListValidatorsHandler)
	http.HandleFunc("/broadcast_tx", BroadcastTxHandler)
	//http.HandleFunc("/call", CallHandler)
	//http.HandleFunc("/get_storage", GetStorageHandler)

	http.HandleFunc("/develop/gen_priv_account", GenPrivAccountHandler)
	http.HandleFunc("/develop/list_accounts", ListAccountsHandler)
	http.HandleFunc("/develop/sign_tx", SignTxHandler)
}
