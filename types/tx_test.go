package types

import (
	"testing"

	acm "github.com/tendermint/tendermint/account"
	. "github.com/tendermint/tendermint/common"
	_ "github.com/tendermint/tendermint/config/tendermint_test"
	ptypes "github.com/tendermint/tendermint/permission/types"
)

var chainID string

func init() {
	chainID = config.GetString("chain_id")
}

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
	signBytes := acm.SignBytes(chainID, sendTx)
	signStr := string(signBytes)
	expected := Fmt(`{"chain_id":"%s","tx":[1,{"inputs":[{"address":"696E70757431","amount":12345,"sequence":67890},{"address":"696E70757432","amount":111,"sequence":222}],"outputs":[{"address":"6F757470757431","amount":333},{"address":"6F757470757432","amount":444}]}]}`,
		config.GetString("chain_id"))
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
	signBytes := acm.SignBytes(chainID, callTx)
	signStr := string(signBytes)
	expected := Fmt(`{"chain_id":"%s","tx":[2,{"address":"636F6E747261637431","data":"6461746131","fee":222,"gas_limit":111,"input":{"address":"696E70757431","amount":12345,"sequence":67890}}]}`,
		config.GetString("chain_id"))
	if signStr != expected {
		t.Errorf("Got unexpected sign string for CallTx. Expected:\n%v\nGot:\n%v", expected, signStr)
	}
}

func TestNameTxSignable(t *testing.T) {
	nameTx := &NameTx{
		Input: &TxInput{
			Address:  []byte("input1"),
			Amount:   12345,
			Sequence: 250,
		},
		Name: "google.com",
		Data: "secretly.not.google.com",
		Fee:  1000,
	}
	signBytes := acm.SignBytes(chainID, nameTx)
	signStr := string(signBytes)
	expected := Fmt(`{"chain_id":"%s","tx":[3,{"data":"secretly.not.google.com","fee":1000,"input":{"address":"696E70757431","amount":12345,"sequence":250},"name":"google.com"}]}`,
		config.GetString("chain_id"))
	if signStr != expected {
		t.Errorf("Got unexpected sign string for CallTx. Expected:\n%v\nGot:\n%v", expected, signStr)
	}
}

func TestBondTxSignable(t *testing.T) {
	privKeyBytes := make([]byte, 64)
	privAccount := acm.GenPrivAccountFromPrivKeyBytes(privKeyBytes)
	bondTx := &BondTx{
		PubKey: privAccount.PubKey.(acm.PubKeyEd25519),
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
	signBytes := acm.SignBytes(chainID, bondTx)
	signStr := string(signBytes)
	expected := Fmt(`{"chain_id":"%s","tx":[17,{"inputs":[{"address":"696E70757431","amount":12345,"sequence":67890},{"address":"696E70757432","amount":111,"sequence":222}],"pub_key":[1,"3B6A27BCCEB6A42D62A3A8D02A6F0D73653215771DE243A63AC048A18B59DA29"],"unbond_to":[{"address":"6F757470757431","amount":333},{"address":"6F757470757432","amount":444}]}]}`,
		config.GetString("chain_id"))
	if signStr != expected {
		t.Errorf("Unexpected sign string for BondTx. \nGot %s\nExpected %s", signStr, expected)
	}
}

func TestUnbondTxSignable(t *testing.T) {
	unbondTx := &UnbondTx{
		Address: []byte("address1"),
		Height:  111,
	}
	signBytes := acm.SignBytes(chainID, unbondTx)
	signStr := string(signBytes)
	expected := Fmt(`{"chain_id":"%s","tx":[18,{"address":"6164647265737331","height":111}]}`,
		config.GetString("chain_id"))
	if signStr != expected {
		t.Errorf("Got unexpected sign string for UnbondTx")
	}
}

func TestRebondTxSignable(t *testing.T) {
	rebondTx := &RebondTx{
		Address: []byte("address1"),
		Height:  111,
	}
	signBytes := acm.SignBytes(chainID, rebondTx)
	signStr := string(signBytes)
	expected := Fmt(`{"chain_id":"%s","tx":[19,{"address":"6164647265737331","height":111}]}`,
		config.GetString("chain_id"))
	if signStr != expected {
		t.Errorf("Got unexpected sign string for RebondTx")
	}
}

func TestPermissionsTxSignable(t *testing.T) {
	permsTx := &PermissionsTx{
		Input: &TxInput{
			Address:  []byte("input1"),
			Amount:   12345,
			Sequence: 250,
		},
		PermArgs: &ptypes.SetBaseArgs{
			Address:    []byte("address1"),
			Permission: 1,
			Value:      true,
		},
	}
	signBytes := acm.SignBytes(chainID, permsTx)
	signStr := string(signBytes)
	expected := Fmt(`{"chain_id":"%s","tx":[32,{"args":"[2,{"address":"6164647265737331","permission":1,"value":true}]","input":{"address":"696E70757431","amount":12345,"sequence":250}}]}`,
		config.GetString("chain_id"))
	if signStr != expected {
		t.Errorf("Got unexpected sign string for CallTx. Expected:\n%v\nGot:\n%v", expected, signStr)
	}
}
