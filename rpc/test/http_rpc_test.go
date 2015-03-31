package rpc

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/tendermint/tendermint2/binary"
	"github.com/tendermint/tendermint2/config"
	"github.com/tendermint/tendermint2/merkle"
	"github.com/tendermint/tendermint2/rpc/core"
	"github.com/tendermint/tendermint2/types"
	"io/ioutil"
	"net/http"
	"net/url"
	"testing"
)

func TestHTTPStatus(t *testing.T) {
	resp, err := http.Get(requestAddr + "status")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	var status struct {
		Status string
		Data   core.ResponseStatus
		Error  string
	}
	err = json.Unmarshal(body, &status)
	if err != nil {
		t.Fatal(err)
	}
	data := status.Data
	if data.Network != config.App().GetString("Network") {
		t.Fatal(fmt.Errorf("Network mismatch: got %s expected %s", data.Network, config.App().Get("Network")))
	}
}

func TestHTTPGenPriv(t *testing.T) {
	resp, err := http.Get(requestAddr + "unsafe/gen_priv_account")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 {
		t.Fatal(resp)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	var status struct {
		Status string
		Data   core.ResponseGenPrivAccount
		Error  string
	}
	binary.ReadJSON(&status, body, &err)
	if err != nil {
		t.Fatal(err)
	}
	if len(status.Data.PrivAccount.Address) == 0 {
		t.Fatal("Failed to generate an address")
	}
}

func TestHTTPGetAccount(t *testing.T) {
	byteAddr, _ := hex.DecodeString(userAddr)
	acc := getAccount(t, "HTTP", byteAddr)
	if acc == nil {
		t.Fatalf("Account was nil")
	}
	if bytes.Compare(acc.Address, byteAddr) != 0 {
		t.Fatalf("Failed to get correct account. Got %x, expected %x", acc.Address, byteAddr)
	}

}

func TestHTTPSignedTx(t *testing.T) {
	byteAddr, _ := hex.DecodeString(userAddr)
	var byteKey [64]byte
	oh, _ := hex.DecodeString(userPriv)
	copy(byteKey[:], oh)

	amt := uint64(100)
	toAddr := []byte{20, 143, 25, 63, 16, 177, 83, 29, 91, 91, 54, 23, 233, 46, 190, 121, 122, 34, 86, 54}
	tx, priv := signTx(t, "HTTP", byteAddr, toAddr, byteKey, amt)
	checkTx(t, byteAddr, priv, tx)

	toAddr = []byte{20, 143, 24, 63, 16, 17, 83, 29, 90, 91, 52, 2, 0, 41, 190, 121, 122, 34, 86, 54}
	tx, priv = signTx(t, "HTTP", byteAddr, toAddr, byteKey, amt)
	checkTx(t, byteAddr, priv, tx)

	toAddr = []byte{0, 0, 4, 0, 0, 4, 0, 0, 4, 91, 52, 2, 0, 41, 190, 121, 122, 34, 86, 54}
	tx, priv = signTx(t, "HTTP", byteAddr, toAddr, byteKey, amt)
	checkTx(t, byteAddr, priv, tx)
}

func TestHTTPBroadcastTx(t *testing.T) {
	byteAddr, _ := hex.DecodeString(userAddr)
	var byteKey [64]byte
	oh, _ := hex.DecodeString(userPriv)
	copy(byteKey[:], oh)

	amt := uint64(100)
	toAddr := []byte{20, 143, 25, 63, 16, 177, 83, 29, 91, 91, 54, 23, 233, 46, 190, 121, 122, 34, 86, 54}
	tx, priv := signTx(t, "HTTP", byteAddr, toAddr, byteKey, amt)
	checkTx(t, byteAddr, priv, tx)

	n, w := new(int64), new(bytes.Buffer)
	var err error
	binary.WriteJSON(tx, w, n, &err)
	if err != nil {
		t.Fatal(err)
	}
	b := w.Bytes()

	var status struct {
		Status string
		Data   core.ResponseBroadcastTx
		Error  string
	}
	requestResponse(t, "broadcast_tx", url.Values{"tx": {string(b)}}, &status)
	if status.Status == "ERROR" {
		t.Fatal(status.Error)
	}
	receipt := status.Data.Receipt
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
	if bytes.Compare(merkle.HashFromBinary(tx), merkle.HashFromBinary(tx2)) != 0 {
		t.Fatal("inconsistent hashes for mempool tx and sent tx")
	}

}

/*tx.Inputs[0].Signature = mint.priv.PrivKey.Sign(account.SignBytes(tx))
err = mint.MempoolReactor.BroadcastTx(tx)
return hex.EncodeToString(merkle.HashFromBinary(tx)), err*/
