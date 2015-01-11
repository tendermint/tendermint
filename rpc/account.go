package rpc

import (
	"net/http"

	"github.com/tendermint/tendermint/account"
	"github.com/tendermint/tendermint/binary"
	"github.com/tendermint/tendermint/block"
	. "github.com/tendermint/tendermint/common"
)

type GenPrivAccountResponse struct {
	PrivAccount *account.PrivAccount
}

func GenPrivAccountHandler(w http.ResponseWriter, r *http.Request) {
	privAccount := account.GenPrivAccount()

	res := GenPrivAccountResponse{
		PrivAccount: privAccount,
	}
	WriteAPIResponse(w, API_OK, res)
}

//-----------------------------------------------------------------------------

type SignSendTxResponse struct {
	SendTx *block.SendTx
}

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

	res := SignSendTxResponse{
		SendTx: sendTx,
	}
	WriteAPIResponse(w, API_OK, res)
}

//-----------------------------------------------------------------------------

type ListAccountsResponse struct {
	Accounts []*account.Account
}

func ListAccountsHandler(w http.ResponseWriter, r *http.Request) {
	state := consensusState.GetState()
	state.GetAccounts().Iterate(func(key interface{}, value interface{}) bool {
		log.Warn(">>", "key", key, "value", value)
		return false
	})
	WriteAPIResponse(w, API_OK, state)
}
