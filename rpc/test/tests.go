package rpc

import (
	"bytes"
	"encoding/hex"
	"fmt"
	. "github.com/tendermint/tendermint/common"
	"github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/types"
	"testing"
	"time"
)

func testStatus(t *testing.T, typ string) {
	client := clients[typ]
	resp, err := client.Status()
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println(">>>", resp)
	if resp.Network != config.App().GetString("Network") {
		t.Fatal(fmt.Errorf("Network mismatch: got %s expected %s",
			resp.Network, config.App().Get("Network")))
	}
}

func testGenPriv(t *testing.T, typ string) {
	client := clients[typ]
	resp, err := client.GenPrivAccount()
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println(">>>", resp)
	if len(resp.PrivAccount.Address) == 0 {
		t.Fatal("Failed to generate an address")
	}
}

func testGetAccount(t *testing.T, typ string) {
	byteAddr, _ := hex.DecodeString(userAddr)
	acc := getAccount(t, typ, byteAddr)
	if acc == nil {
		t.Fatalf("Account was nil")
	}
	if bytes.Compare(acc.Address, byteAddr) != 0 {
		t.Fatalf("Failed to get correct account. Got %x, expected %x", acc.Address, byteAddr)
	}
}

func testSignedTx(t *testing.T, typ string) {
	byteAddr, _ := hex.DecodeString(userAddr)
	var byteKey [64]byte
	oh, _ := hex.DecodeString(userPriv)
	copy(byteKey[:], oh)

	amt := uint64(100)
	toAddr := []byte{20, 143, 25, 63, 16, 177, 83, 29, 91, 91, 54, 23, 233, 46, 190, 121, 122, 34, 86, 54}
	tx, priv := signTx(t, typ, byteAddr, toAddr, nil, byteKey, amt, 0, 0)
	checkTx(t, byteAddr, priv, tx.(*types.SendTx))

	toAddr = []byte{20, 143, 24, 63, 16, 17, 83, 29, 90, 91, 52, 2, 0, 41, 190, 121, 122, 34, 86, 54}
	tx, priv = signTx(t, typ, byteAddr, toAddr, nil, byteKey, amt, 0, 0)
	checkTx(t, byteAddr, priv, tx.(*types.SendTx))

	toAddr = []byte{0, 0, 4, 0, 0, 4, 0, 0, 4, 91, 52, 2, 0, 41, 190, 121, 122, 34, 86, 54}
	tx, priv = signTx(t, typ, byteAddr, toAddr, nil, byteKey, amt, 0, 0)
	checkTx(t, byteAddr, priv, tx.(*types.SendTx))
}

func testBroadcastTx(t *testing.T, typ string) {
	byteAddr, _ := hex.DecodeString(userAddr)
	var byteKey [64]byte
	oh, _ := hex.DecodeString(userPriv)
	copy(byteKey[:], oh)

	amt := uint64(100)
	toAddr := []byte{20, 143, 25, 63, 16, 177, 83, 29, 91, 91, 54, 23, 233, 46, 190, 121, 122, 34, 86, 54}
	tx, receipt := broadcastTx(t, typ, byteAddr, toAddr, nil, byteKey, amt, 0, 0)
	if receipt.CreatesContract > 0 {
		t.Fatal("This tx does not create a contract")
	}
	if len(receipt.TxHash) == 0 {
		t.Fatal("Failed to compute tx hash")
	}
	pool := node.MempoolReactor().Mempool
	txs := pool.GetProposalTxs()
	if len(txs) != mempoolCount+1 {
		t.Fatalf("The mem pool has %d txs. Expected %d", len(txs), mempoolCount+1)
	}
	tx2 := txs[mempoolCount].(*types.SendTx)
	mempoolCount += 1
	n, err := new(int64), new(error)
	buf1, buf2 := new(bytes.Buffer), new(bytes.Buffer)
	tx.WriteSignBytes(buf1, n, err)
	tx2.WriteSignBytes(buf2, n, err)
	if bytes.Compare(buf1.Bytes(), buf2.Bytes()) != 0 {
		t.Fatal("inconsistent hashes for mempool tx and sent tx")
	}
}

func testGetStorage(t *testing.T, typ string) {
	priv := state.LoadPrivValidator(".tendermint/priv_validator.json")
	_ = priv
	//core.SetPrivValidator(priv)

	byteAddr, _ := hex.DecodeString(userAddr)
	var byteKey [64]byte
	oh, _ := hex.DecodeString(userPriv)
	copy(byteKey[:], oh)

	amt := uint64(1100)
	code := []byte{0x60, 0x5, 0x60, 0x1, 0x55}
	_, receipt := broadcastTx(t, typ, byteAddr, nil, code, byteKey, amt, 1000, 1000)
	if receipt.CreatesContract == 0 {
		t.Fatal("This tx creates a contract")
	}
	if len(receipt.TxHash) == 0 {
		t.Fatal("Failed to compute tx hash")
	}
	contractAddr := receipt.ContractAddr
	if len(contractAddr) == 0 {
		t.Fatal("Creates contract but resulting address is empty")
	}
	time.Sleep(time.Second * 20)
	mempoolCount -= 1

	v := getStorage(t, typ, contractAddr, []byte{0x1})
	got := RightPadWord256(v)
	expected := RightPadWord256([]byte{0x5})
	if got.Compare(expected) != 0 {
		t.Fatalf("Wrong storage value. Got %x, expected %x", got.Bytes(), expected.Bytes())
	}
}
