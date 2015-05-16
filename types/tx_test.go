package types

import (
	"testing"

	"github.com/tendermint/tendermint/account"
	. "github.com/tendermint/tendermint/common"
	_ "github.com/tendermint/tendermint/test"
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
	expected := Fmt(`{"network":"%X","tx":[1,{"inputs":[{"address":"696E70757431","amount":12345,"sequence":67890},{"address":"696E70757432","amount":111,"sequence":222}],"outputs":[{"address":"6F757470757431","amount":333},{"address":"6F757470757432","amount":444}]}]}`,
		config.GetString("network"))
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
	expected := Fmt(`{"network":"%X","tx":[2,{"address":"636F6E747261637431","data":"6461746131","fee":222,"gas_limit":111,"input":{"address":"696E70757431","amount":12345,"sequence":67890}}]}`,
		config.GetString("network"))
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
	expected := Fmt(`{"network":"%X","tx":[17,{"inputs":[{"address":"696E70757431","amount":12345,"sequence":67890},{"address":"696E70757432","amount":111,"sequence":222}],"pub_key":[1,"3B6A27BCCEB6A42D62A3A8D02A6F0D73653215771DE243A63AC048A18B59DA29"],"unbond_to":[{"address":"6F757470757431","amount":333},{"address":"6F757470757432","amount":444}]}]}`,
		config.GetString("network"))
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
	expected := Fmt(`{"network":"%X","tx":[18,{"address":"6164647265737331","height":111}]}`,
		config.GetString("network"))
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
	expected := Fmt(`{"network":"%X","tx":[19,{"address":"6164647265737331","height":111}]}`,
		config.GetString("network"))
	if signStr != expected {
		t.Errorf("Got unexpected sign string for RebondTx")
	}
}
