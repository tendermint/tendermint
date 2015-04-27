package types

import (
	"testing"

	"github.com/tendermint/tendermint/account"
	. "github.com/tendermint/tendermint/common"
	"github.com/tendermint/tendermint/config"
)

func TestSendTxSignable(t *testing.T) {
	sendTx := &SendTx{
		Inputs: []*TxInput{
			&TxInput{
				Address:  []byte("input1"),
				Amount:   12345,
				Sequence: 67890,
			},
			&TxInput{
				Address:  []byte("input2"),
				Amount:   111,
				Sequence: 222,
			},
		},
		Outputs: []*TxOutput{
			&TxOutput{
				Address: []byte("output1"),
				Amount:  333,
			},
			&TxOutput{
				Address: []byte("output2"),
				Amount:  444,
			},
		},
	}
	signBytes := account.SignBytes(sendTx)
	signStr := string(signBytes)
	expected := Fmt(`{"Network":"%X","Tx":[1,{"Inputs":[{"Address":"696E70757431","Amount":12345,"Sequence":67890},{"Address":"696E70757432","Amount":111,"Sequence":222}],"Outputs":[{"Address":"6F757470757431","Amount":333},{"Address":"6F757470757432","Amount":444}]}]}`,
		config.App().GetString("Network"))
	if signStr != expected {
		t.Errorf("Got unexpected sign string for SendTx. Expected:\n%v\nGot:\n%v", expected, signStr)
	}
}

func TestCallTxSignable(t *testing.T) {
	callTx := &CallTx{
		Input: &TxInput{
			Address:  []byte("input1"),
			Amount:   12345,
			Sequence: 67890,
		},
		Address:  []byte("contract1"),
		GasLimit: 111,
		Fee:      222,
		Data:     []byte("data1"),
	}
	signBytes := account.SignBytes(callTx)
	signStr := string(signBytes)
	expected := Fmt(`{"Network":"%X","Tx":[2,{"Address":"636F6E747261637431","Data":"6461746131","Fee":222,"GasLimit":111,"Input":{"Address":"696E70757431","Amount":12345,"Sequence":67890}}]}`,
		config.App().GetString("Network"))
	if signStr != expected {
		t.Errorf("Got unexpected sign string for CallTx. Expected:\n%v\nGot:\n%v", expected, signStr)
	}
}

func TestBondTxSignable(t *testing.T) {
	privAccount := account.GenPrivAccountFromKey([64]byte{})
	bondTx := &BondTx{
		PubKey: privAccount.PubKey.(account.PubKeyEd25519),
		Inputs: []*TxInput{
			&TxInput{
				Address:  []byte("input1"),
				Amount:   12345,
				Sequence: 67890,
			},
			&TxInput{
				Address:  []byte("input2"),
				Amount:   111,
				Sequence: 222,
			},
		},
		UnbondTo: []*TxOutput{
			&TxOutput{
				Address: []byte("output1"),
				Amount:  333,
			},
			&TxOutput{
				Address: []byte("output2"),
				Amount:  444,
			},
		},
	}
	signBytes := account.SignBytes(bondTx)
	signStr := string(signBytes)
	expected := Fmt(`{"Network":"%X","Tx":[17,{"Inputs":[{"Address":"696E70757431","Amount":12345,"Sequence":67890},{"Address":"696E70757432","Amount":111,"Sequence":222}],"PubKey":[1,"3B6A27BCCEB6A42D62A3A8D02A6F0D73653215771DE243A63AC048A18B59DA29"],"UnbondTo":[{"Address":"6F757470757431","Amount":333},{"Address":"6F757470757432","Amount":444}]}]}`,
		config.App().GetString("Network"))
	if signStr != expected {
		t.Errorf("Got unexpected sign string for BondTx")
	}
}

func TestUnbondTxSignable(t *testing.T) {
	unbondTx := &UnbondTx{
		Address: []byte("address1"),
		Height:  111,
	}
	signBytes := account.SignBytes(unbondTx)
	signStr := string(signBytes)
	expected := Fmt(`{"Network":"%X","Tx":[18,{"Address":"6164647265737331","Height":111}]}`,
		config.App().GetString("Network"))
	if signStr != expected {
		t.Errorf("Got unexpected sign string for UnbondTx")
	}
}

func TestRebondTxSignable(t *testing.T) {
	rebondTx := &RebondTx{
		Address: []byte("address1"),
		Height:  111,
	}
	signBytes := account.SignBytes(rebondTx)
	signStr := string(signBytes)
	expected := Fmt(`{"Network":"%X","Tx":[19,{"Address":"6164647265737331","Height":111}]}`,
		config.App().GetString("Network"))
	if signStr != expected {
		t.Errorf("Got unexpected sign string for RebondTx")
	}
}
