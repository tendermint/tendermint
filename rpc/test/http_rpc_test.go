package rpc

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"github.com/tendermint/tendermint/binary"
	. "github.com/tendermint/tendermint/common"
	"github.com/tendermint/tendermint/merkle"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
	"github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/types"
	"io/ioutil"
	"net/http"
	"testing"
	"time"
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
	var response struct {
		Result  ctypes.ResponseStatus `json:"result"`
		Error   string                `json:"error"`
		Id      string                `json:"id"`
		JSONRPC string                `json:"jsonrpc"`
	}
	binary.ReadJSON(&response, body, &err)
	if err != nil {
		t.Fatal(err)
	}
	result := response.Result
	fmt.Println(">>>", result)
	return
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
	var response struct {
		Result  ctypes.ResponseGenPrivAccount `json:"result"`
		Error   string                        `json:"error"`
		Id      string                        `json:"id"`
		JSONRPC string                        `json:"jsonrpc"`
	}
	binary.ReadJSON(&response, body, &err)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println(">>>", response)
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
	tx, priv := signTx(t, "HTTP", byteAddr, toAddr, nil, byteKey, amt, 0, 0)
	checkTx(t, byteAddr, priv, tx.(*types.SendTx))

	toAddr = []byte{20, 143, 24, 63, 16, 17, 83, 29, 90, 91, 52, 2, 0, 41, 190, 121, 122, 34, 86, 54}
	tx, priv = signTx(t, "HTTP", byteAddr, toAddr, nil, byteKey, amt, 0, 0)
	checkTx(t, byteAddr, priv, tx.(*types.SendTx))

	toAddr = []byte{0, 0, 4, 0, 0, 4, 0, 0, 4, 91, 52, 2, 0, 41, 190, 121, 122, 34, 86, 54}
	tx, priv = signTx(t, "HTTP", byteAddr, toAddr, nil, byteKey, amt, 0, 0)
	checkTx(t, byteAddr, priv, tx.(*types.SendTx))
}

func TestHTTPBroadcastTx(t *testing.T) {
	byteAddr, _ := hex.DecodeString(userAddr)
	var byteKey [64]byte
	oh, _ := hex.DecodeString(userPriv)
	copy(byteKey[:], oh)

	amt := uint64(100)
	toAddr := []byte{20, 143, 25, 63, 16, 177, 83, 29, 91, 91, 54, 23, 233, 46, 190, 121, 122, 34, 86, 54}
	tx, receipt := broadcastTx(t, "HTTP", byteAddr, toAddr, nil, byteKey, amt, 0, 0)
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

func TestHTTPGetStorage(t *testing.T) {
	priv := state.LoadPrivValidator(".tendermint/priv_validator.json")
	_ = priv
	//ctypes.SetPrivValidator(priv)

	byteAddr, _ := hex.DecodeString(userAddr)
	var byteKey [64]byte
	oh, _ := hex.DecodeString(userPriv)
	copy(byteKey[:], oh)

	amt := uint64(1100)
	code := []byte{0x60, 0x5, 0x60, 0x1, 0x55}
	_, receipt := broadcastTx(t, "HTTP", byteAddr, nil, code, byteKey, amt, 1000, 1000)
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

	v := getStorage(t, contractAddr, []byte{0x1})
	got := RightPadWord256(v)
	expected := RightPadWord256([]byte{0x5})
	if got.Compare(expected) != 0 {
		t.Fatalf("Wrong storage value. Got %x, expected %x", got.Bytes(), expected.Bytes())
	}
}

/*tx.Inputs[0].Signature = mint.priv.PrivKey.Sign(account.SignBytes(tx))
err = mint.MempoolReactor.BroadcastTx(tx)
return hex.EncodeToString(merkle.HashFromBinary(tx)), err*/
