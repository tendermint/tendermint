package rpc

import (
	"net/http"

	"github.com/tendermint/tendermint/account"
	"github.com/tendermint/tendermint/binary"
	blk "github.com/tendermint/tendermint/block"
	. "github.com/tendermint/tendermint/common"
)

func SignTxHandler(w http.ResponseWriter, r *http.Request) {
	txStr := GetParam(r, "tx")
	privAccountsStr := GetParam(r, "privAccounts")

	var err error
	var tx blk.Tx
	binary.ReadJSON(&tx, []byte(txStr), &err)
	if err != nil {
		WriteAPIResponse(w, API_INVALID_PARAM, Fmt("Invalid tx: %v", err))
		return
	}
	privAccounts := binary.ReadJSON([]*account.PrivAccount{}, []byte(privAccountsStr), &err).([]*account.PrivAccount)
	if err != nil {
		WriteAPIResponse(w, API_INVALID_PARAM, Fmt("Invalid privAccounts: %v", err))
		return
	}
	for i, privAccount := range privAccounts {
		if privAccount == nil || privAccount.PrivKey == nil {
			WriteAPIResponse(w, API_INVALID_PARAM, Fmt("Invalid (empty) privAccount @%v", i))
			return
		}
	}

	switch tx.(type) {
	case *blk.SendTx:
		sendTx := tx.(*blk.SendTx)
		for i, input := range sendTx.Inputs {
			input.PubKey = privAccounts[i].PubKey
			input.Signature = privAccounts[i].Sign(sendTx)
		}
	case *blk.BondTx:
		bondTx := tx.(*blk.BondTx)
		for i, input := range bondTx.Inputs {
			input.PubKey = privAccounts[i].PubKey
			input.Signature = privAccounts[i].Sign(bondTx)
		}
	case *blk.UnbondTx:
		unbondTx := tx.(*blk.UnbondTx)
		unbondTx.Signature = privAccounts[0].Sign(unbondTx).(account.SignatureEd25519)
	case *blk.RebondTx:
		rebondTx := tx.(*blk.RebondTx)
		rebondTx.Signature = privAccounts[0].Sign(rebondTx).(account.SignatureEd25519)
	}

	WriteAPIResponse(w, API_OK, struct{ blk.Tx }{tx})
}
