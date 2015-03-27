package core

import (
	"fmt"
	"github.com/tendermint/tendermint/account"
	"github.com/tendermint/tendermint/types"
)

//-----------------------------------------------------------------------------

func SignTx(tx types.Tx, privAccounts []*account.PrivAccount) (types.Tx, error) {
	// more checks?

	for i, privAccount := range privAccounts {
		if privAccount == nil || privAccount.PrivKey == nil {
			return nil, fmt.Errorf("Invalid (empty) privAccount @%v", i)
		}
	}

	switch tx.(type) {
	case *types.SendTx:
		sendTx := tx.(*types.SendTx)
		for i, input := range sendTx.Inputs {
			input.PubKey = privAccounts[i].PubKey
			input.Signature = privAccounts[i].Sign(sendTx)
		}
	case *types.CallTx:
		callTx := tx.(*types.CallTx)
		callTx.Input.PubKey = privAccounts[0].PubKey
		callTx.Input.Signature = privAccounts[0].Sign(callTx)
	case *types.BondTx:
		bondTx := tx.(*types.BondTx)
		for i, input := range bondTx.Inputs {
			input.PubKey = privAccounts[i].PubKey
			input.Signature = privAccounts[i].Sign(bondTx)
		}
	case *types.UnbondTx:
		unbondTx := tx.(*types.UnbondTx)
		unbondTx.Signature = privAccounts[0].Sign(unbondTx).(account.SignatureEd25519)
	case *types.RebondTx:
		rebondTx := tx.(*types.RebondTx)
		rebondTx.Signature = privAccounts[0].Sign(rebondTx).(account.SignatureEd25519)
	}
	return tx, nil
}
