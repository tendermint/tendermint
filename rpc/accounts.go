package rpc

import (
	"net/http"

	"github.com/tendermint/tendermint/account"
	"github.com/tendermint/tendermint/binary"
	. "github.com/tendermint/tendermint/common"
)

func GenPrivAccountHandler(w http.ResponseWriter, r *http.Request) {
	privAccount := account.GenPrivAccount()

	WriteAPIResponse(w, API_OK, struct {
		PrivAccount *account.PrivAccount
	}{privAccount})
}

//-----------------------------------------------------------------------------

func GetAccountHandler(w http.ResponseWriter, r *http.Request) {
	addressStr := GetParam(r, "address")

	var address []byte
	var err error
	binary.ReadJSON(&address, []byte(addressStr), &err)
	if err != nil {
		WriteAPIResponse(w, API_INVALID_PARAM, Fmt("Invalid address: %v", err))
		return
	}

	state := consensusState.GetState()
	account_ := state.GetAccount(address)

	if account_ == nil {
		WriteAPIResponse(w, API_OK, struct{}{})
		return
	}

	WriteAPIResponse(w, API_OK, struct {
		Account *account.Account
	}{account_})
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
