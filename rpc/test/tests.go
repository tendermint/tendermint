package rpctest

import (
	"bytes"
	"fmt"
	. "github.com/tendermint/tendermint/common"
	"github.com/tendermint/tendermint/types"
	"testing"
)

var doNothing = func(eid string, b []byte) error { return nil }

func testStatus(t *testing.T, typ string) {
	client := clients[typ]
	resp, err := client.Status()
	if err != nil {
		t.Fatal(err)
	}
	if resp.NodeInfo.ChainID != chainID {
		t.Fatal(fmt.Errorf("ChainID mismatch: got %s expected %s",
			resp.NodeInfo.ChainID, chainID))
	}
}

func testGenPriv(t *testing.T, typ string) {
	client := clients[typ]
	privAcc, err := client.GenPrivAccount()
	if err != nil {
		t.Fatal(err)
	}
	if len(privAcc.PrivAccount.Address) == 0 {
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
	amt := int64(100)
	toAddr := user[1].Address
	testOneSignTx(t, typ, toAddr, amt)

	toAddr = user[2].Address
	testOneSignTx(t, typ, toAddr, amt)

	toAddr = user[3].Address
	testOneSignTx(t, typ, toAddr, amt)
}

func testOneSignTx(t *testing.T, typ string, addr []byte, amt int64) {
	tx := makeDefaultSendTx(t, typ, addr, amt)
	tx2 := signTx(t, typ, tx, user[0])
	tx2hash := types.TxID(chainID, tx2)
	tx.SignInput(chainID, 0, user[0])
	txhash := types.TxID(chainID, tx)
	if bytes.Compare(txhash, tx2hash) != 0 {
		t.Fatal("Got different signatures for signing via rpc vs tx_utils")
	}

	tx_ := signTx(t, typ, tx, user[0])
	tx = tx_.(*types.SendTx)
	checkTx(t, user[0].Address, user[0], tx)
}

func testBroadcastTx(t *testing.T, typ string) {
	amt := int64(100)
	toAddr := user[1].Address
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

	amt, gasLim, fee := int64(1100), int64(1000), int64(1000)
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
	waitForEvent(t, con, eid, true, func() {}, doNothing)
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
	callCode(t, client, user[0].PubKey.Address(), code, data, expected)

	// pass two ints as calldata, add, and return the result
	code = []byte{0x60, 0x0, 0x35, 0x60, 0x20, 0x35, 0x1, 0x60, 0x0, 0x52, 0x60, 0x20, 0x60, 0x0, 0xf3}
	data = append(LeftPadWord256([]byte{0x5}).Bytes(), LeftPadWord256([]byte{0x6}).Bytes()...)
	expected = []byte{0xb}
	callCode(t, client, user[0].PubKey.Address(), code, data, expected)
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
	amt, gasLim, fee := int64(6969), int64(1000), int64(1000)
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
	waitForEvent(t, con, eid, true, func() {}, doNothing)
	mempoolCount = 0

	// run a call through the contract
	data := []byte{}
	expected := []byte{0xb}
	callContract(t, client, user[0].PubKey.Address(), contractAddr, data, expected)
}

func testNameReg(t *testing.T, typ string) {
	client := clients[typ]
	con := newWSCon(t)

	types.MinNameRegistrationPeriod = 1

	// register a new name, check if its there
	// since entries ought to be unique and these run against different clients, we append the typ
	name := "ye_old_domain_name_" + typ
	data := "if not now, when"
	fee := int64(1000)
	numDesiredBlocks := int64(2)
	amt := fee + numDesiredBlocks*types.NameByteCostMultiplier*types.NameBlockCostMultiplier*types.NameBaseCost(name, data)

	eid := types.EventStringNameReg(name)
	subscribe(t, con, eid)

	tx := makeDefaultNameTx(t, typ, name, data, amt, fee)
	broadcastTx(t, typ, tx)
	// verify the name by both using the event and by checking get_name
	waitForEvent(t, con, eid, true, func() {}, func(eid string, b []byte) error {
		// TODO: unmarshal the response
		tx, err := unmarshalResponseNameReg(b)
		if err != nil {
			return err
		}
		if tx.Name != name {
			t.Fatal(fmt.Sprintf("Err on received event tx.Name: Got %s, expected %s", tx.Name, name))
		}
		if tx.Data != data {
			t.Fatal(fmt.Sprintf("Err on received event tx.Data: Got %s, expected %s", tx.Data, data))
		}
		return nil
	})
	mempoolCount = 0
	entry := getNameRegEntry(t, typ, name)
	if entry.Data != data {
		t.Fatal(fmt.Sprintf("Err on entry.Data: Got %s, expected %s", entry.Data, data))
	}
	if bytes.Compare(entry.Owner, user[0].Address) != 0 {
		t.Fatal(fmt.Sprintf("Err on entry.Owner: Got %s, expected %s", entry.Owner, user[0].Address))
	}

	unsubscribe(t, con, eid)

	// for the rest we just use new block event
	// since we already tested the namereg event
	eid = types.EventStringNewBlock()
	subscribe(t, con, eid)
	defer func() {
		unsubscribe(t, con, eid)
		con.Close()
	}()

	// update the data as the owner, make sure still there
	numDesiredBlocks = int64(2)
	data = "these are amongst the things I wish to bestow upon the youth of generations come: a safe supply of honey, and a better money. For what else shall they need"
	amt = fee + numDesiredBlocks*types.NameByteCostMultiplier*types.NameBlockCostMultiplier*types.NameBaseCost(name, data)
	tx = makeDefaultNameTx(t, typ, name, data, amt, fee)
	broadcastTx(t, typ, tx)
	// commit block
	waitForEvent(t, con, eid, true, func() {}, doNothing)
	mempoolCount = 0
	entry = getNameRegEntry(t, typ, name)
	if entry.Data != data {
		t.Fatal(fmt.Sprintf("Err on entry.Data: Got %s, expected %s", entry.Data, data))
	}

	// try to update as non owner, should fail
	nonce := getNonce(t, typ, user[1].Address)
	data2 := "this is not my beautiful house"
	tx = types.NewNameTxWithNonce(user[1].PubKey, name, data2, amt, fee, nonce+1)
	tx.Sign(chainID, user[1])
	_, err := client.BroadcastTx(tx)
	if err == nil {
		t.Fatal("Expected error on NameTx")
	}

	// commit block
	waitForEvent(t, con, eid, true, func() {}, doNothing)

	// now the entry should be expired, so we can update as non owner
	_, err = client.BroadcastTx(tx)
	waitForEvent(t, con, eid, true, func() {}, doNothing)
	mempoolCount = 0
	entry = getNameRegEntry(t, typ, name)
	if entry.Data != data2 {
		t.Fatal(fmt.Sprintf("Error on entry.Data: Got %s, expected %s", entry.Data, data2))
	}
	if bytes.Compare(entry.Owner, user[1].Address) != 0 {
		t.Fatal(fmt.Sprintf("Err on entry.Owner: Got %s, expected %s", entry.Owner, user[1].Address))
	}
}
