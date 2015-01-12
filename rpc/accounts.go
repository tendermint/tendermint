package rpc

import (
	"net/http"

	"github.com/tendermint/tendermint/account"
	"github.com/tendermint/tendermint/binary"
	"github.com/tendermint/tendermint/block"
	. "github.com/tendermint/tendermint/common"
)

func GenPrivAccountHandler(w http.ResponseWriter, r *http.Request) {
	privAccount := account.GenPrivAccount()

	WriteAPIResponse(w, API_OK, struct {
		PrivAccount *account.PrivAccount
	}{privAccount})
}

//-----------------------------------------------------------------------------

func SignSendTxHandler(w http.ResponseWriter, r *http.Request) {
	sendTxStr := GetParam(r, "sendTx")
	privAccountsStr := GetParam(r, "privAccounts")

	var err error
	sendTx := binary.ReadJSON(&block.SendTx{}, []byte(sendTxStr), &err).(*block.SendTx)
	if err != nil {
		WriteAPIResponse(w, API_INVALID_PARAM, Fmt("Invalid sendTx: %v", err))
		return
	}
	privAccounts := binary.ReadJSON([]*account.PrivAccount{}, []byte(privAccountsStr), &err).([]*account.PrivAccount)
	if err != nil {
		WriteAPIResponse(w, API_INVALID_PARAM, Fmt("Invalid privAccounts: %v", err))
		return
	}

	for i, input := range sendTx.Inputs {
		input.PubKey = privAccounts[i].PubKey
		input.Signature = privAccounts[i].Sign(sendTx)
	}

	WriteAPIResponse(w, API_OK, struct {
		SendTx *block.SendTx
	}{sendTx})
}

//-----------------------------------------------------------------------------

func ListAccountsHandler(w http.ResponseWriter, r *http.Request) {
	var blockHeight uint
	var accounts []*account.Account
	state := consensusState.GetState()
	blockHeight = state.LastBlockHeight
	state.GetAccounts().Iterate(func(key interface{}, value interface{}) bool {
		accounts = append(accounts, value.(*account.Account))
		return false
	})

	WriteAPIResponse(w, API_OK, struct {
		BlockHeight uint
		Accounts    []*account.Account
	}{blockHeight, accounts})
}
