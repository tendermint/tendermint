package rpctest

import (
	"bytes"
	"fmt"
	"github.com/tendermint/tendermint/account"
	. "github.com/tendermint/tendermint/common"
	"github.com/tendermint/tendermint/types"
	"testing"
)

func testStatus(t *testing.T, typ string) {
	client := clients[typ]
	resp, err := client.Status()
	if err != nil {
		t.Fatal(err)
	}
	if resp.ChainID != chainID {
		t.Fatal(fmt.Errorf("ChainID mismatch: got %s expected %s",
			resp.ChainID, chainID))
	}
}

func testGenPriv(t *testing.T, typ string) {
	client := clients[typ]
	resp, err := client.GenPrivAccount()
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.PrivAccount.Address) == 0 {
		t.Fatal("Failed to generate an address")
	}
}

func testGetAccount(t *testing.T, typ string) {
	acc := getAccount(t, typ, user[0].Address)
	if acc == nil {
		t.Fatalf("Account was nil")
	}
	if bytes.Compare(acc.Address, user[0].Address) != 0 {
		t.Fatalf("Failed to get correct account. Got %x, expected %x", acc.Address, user[0].Address)
	}
}

func testSignedTx(t *testing.T, typ string) {
	amt := uint64(100)
	toAddr := []byte{20, 143, 25, 63, 16, 177, 83, 29, 91, 91, 54, 23, 233, 46, 190, 121, 122, 34, 86, 54}
	testOneSignTx(t, typ, toAddr, amt)

	toAddr = []byte{20, 143, 24, 63, 16, 17, 83, 29, 90, 91, 52, 2, 0, 41, 190, 121, 122, 34, 86, 54}
	testOneSignTx(t, typ, toAddr, amt)

	toAddr = []byte{0, 0, 4, 0, 0, 4, 0, 0, 4, 91, 52, 2, 0, 41, 190, 121, 122, 34, 86, 54}
	testOneSignTx(t, typ, toAddr, amt)
}

func testOneSignTx(t *testing.T, typ string, addr []byte, amt uint64) {
	tx := makeDefaultSendTx(t, typ, addr, amt)
	tx2 := signTx(t, typ, tx, user[0])
	tx2hash := account.HashSignBytes(chainID, tx2)
	tx.SignInput(chainID, 0, user[0])
	txhash := account.HashSignBytes(chainID, tx)
	if bytes.Compare(txhash, tx2hash) != 0 {
		t.Fatal("Got different signatures for signing via rpc vs tx_utils")
	}

	tx_ := signTx(t, typ, tx, user[0])
	tx = tx_.(*types.SendTx)
	checkTx(t, user[0].Address, user[0], tx)
}

func testBroadcastTx(t *testing.T, typ string) {
	amt := uint64(100)
	toAddr := []byte{20, 143, 25, 63, 16, 177, 83, 29, 91, 91, 54, 23, 233, 46, 190, 121, 122, 34, 86, 54}
	tx := makeDefaultSendTxSigned(t, typ, toAddr, amt)
	receipt := broadcastTx(t, typ, tx)
	if receipt.CreatesContract > 0 {
		t.Fatal("This tx does not create a contract")
	}
	if len(receipt.TxHash) == 0 {
		t.Fatal("Failed to compute tx hash")
	}
	pool := node.MempoolReactor().Mempool
	txs := pool.GetProposalTxs()
	if len(txs) != mempoolCount {
		t.Fatalf("The mem pool has %d txs. Expected %d", len(txs), mempoolCount)
	}
	tx2 := txs[mempoolCount-1].(*types.SendTx)
	n, err := new(int64), new(error)
	buf1, buf2 := new(bytes.Buffer), new(bytes.Buffer)
	tx.WriteSignBytes(chainID, buf1, n, err)
	tx2.WriteSignBytes(chainID, buf2, n, err)
	if bytes.Compare(buf1.Bytes(), buf2.Bytes()) != 0 {
		t.Fatal("inconsistent hashes for mempool tx and sent tx")
	}
}

func testGetStorage(t *testing.T, typ string) {
	con := newWSCon(t)
	eid := types.EventStringNewBlock()
	subscribe(t, con, eid)
	defer func() {
		unsubscribe(t, con, eid)
		con.Close()
	}()

	amt, gasLim, fee := uint64(1100), uint64(1000), uint64(1000)
	code := []byte{0x60, 0x5, 0x60, 0x1, 0x55}
	tx := makeDefaultCallTx(t, typ, nil, code, amt, gasLim, fee)
	receipt := broadcastTx(t, typ, tx)
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

	// allow it to get mined
	waitForEvent(t, con, eid, true, func() {
	}, func(eid string, b []byte) error {
		return nil
	})
	mempoolCount = 0

	v := getStorage(t, typ, contractAddr, []byte{0x1})
	got := LeftPadWord256(v)
	expected := LeftPadWord256([]byte{0x5})
	if got.Compare(expected) != 0 {
		t.Fatalf("Wrong storage value. Got %x, expected %x", got.Bytes(), expected.Bytes())
	}
}

func testCallCode(t *testing.T, typ string) {
	client := clients[typ]

	// add two integers and return the result
	code := []byte{0x60, 0x5, 0x60, 0x6, 0x1, 0x60, 0x0, 0x52, 0x60, 0x20, 0x60, 0x0, 0xf3}
	data := []byte{}
	expected := []byte{0xb}
	callCode(t, client, code, data, expected)

	// pass two ints as calldata, add, and return the result
	code = []byte{0x60, 0x0, 0x35, 0x60, 0x20, 0x35, 0x1, 0x60, 0x0, 0x52, 0x60, 0x20, 0x60, 0x0, 0xf3}
	data = append(LeftPadWord256([]byte{0x5}).Bytes(), LeftPadWord256([]byte{0x6}).Bytes()...)
	expected = []byte{0xb}
	callCode(t, client, code, data, expected)
}

func testCall(t *testing.T, typ string) {
	con := newWSCon(t)
	eid := types.EventStringNewBlock()
	subscribe(t, con, eid)
	defer func() {
		unsubscribe(t, con, eid)
		con.Close()
	}()

	client := clients[typ]

	// create the contract
	amt, gasLim, fee := uint64(6969), uint64(1000), uint64(1000)
	code, _, _ := simpleContract()
	tx := makeDefaultCallTx(t, typ, nil, code, amt, gasLim, fee)
	receipt := broadcastTx(t, typ, tx)

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

	// allow it to get mined
	waitForEvent(t, con, eid, true, func() {
	}, func(eid string, b []byte) error {
		return nil
	})
	mempoolCount = 0

	// run a call through the contract
	data := []byte{}
	expected := []byte{0xb}
	callContract(t, client, contractAddr, data, expected)
}
