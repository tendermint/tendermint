package rpctest

import (
	"bytes"
	"strconv"
	"testing"

	acm "github.com/tendermint/tendermint/account"
	. "github.com/tendermint/tendermint/common"
	nm "github.com/tendermint/tendermint/node"
	"github.com/tendermint/tendermint/p2p"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
	cclient "github.com/tendermint/tendermint/rpc/core_client"
	"github.com/tendermint/tendermint/types"
)

// global variables for use across all tests
var (
	rpcAddr       = "127.0.0.1:36657" // Not 46657
	requestAddr   = "http://" + rpcAddr + "/"
	websocketAddr = "ws://" + rpcAddr + "/websocket"

	node *nm.Node

	mempoolCount = 0

	// make keys
	user = makeUsers(5)

	chainID string

	clients = map[string]cclient.Client{
		"JSONRPC": cclient.NewClient(requestAddr, "JSONRPC"),
		"HTTP":    cclient.NewClient(requestAddr, "HTTP"),
	}
)

// deterministic account generation, synced with genesis file in config/tendermint_test/config.go
func makeUsers(n int) []*acm.PrivAccount {
	accounts := []*acm.PrivAccount{}
	for i := 0; i < n; i++ {
		secret := ("mysecret" + strconv.Itoa(i))
		user := acm.GenPrivAccountFromSecret(secret)
		accounts = append(accounts, user)
	}
	return accounts
}

// create a new node and sleep forever
func newNode(ready chan struct{}) {
	// Create & start node
	node = nm.NewNode()
	l := p2p.NewDefaultListener("tcp", config.GetString("node_laddr"))
	node.AddListener(l)
	node.Start()

	// Run the RPC server.
	node.StartRPC()
	ready <- struct{}{}

	// Sleep forever
	ch := make(chan struct{})
	<-ch
}

// initialize config and create new node
func init() {
	chainID = config.GetString("chain_id")

	// Save new priv_validator file.
	priv := &types.PrivValidator{
		Address: user[0].Address,
		PubKey:  acm.PubKeyEd25519(user[0].PubKey.(acm.PubKeyEd25519)),
		PrivKey: acm.PrivKeyEd25519(user[0].PrivKey.(acm.PrivKeyEd25519)),
	}
	priv.SetFile(config.GetString("priv_validator_file"))
	priv.Save()

	// TODO: change consensus/state.go timeouts to be shorter

	// start a node
	ready := make(chan struct{})
	go newNode(ready)
	<-ready
}

//-------------------------------------------------------------------------------
// some default transaction functions

func makeDefaultSendTx(t *testing.T, typ string, addr []byte, amt int64) *types.SendTx {
	nonce := getNonce(t, typ, user[0].Address)
	tx := types.NewSendTx()
	tx.AddInputWithNonce(user[0].PubKey, amt, nonce+1)
	tx.AddOutput(addr, amt)
	return tx
}

func makeDefaultSendTxSigned(t *testing.T, typ string, addr []byte, amt int64) *types.SendTx {
	tx := makeDefaultSendTx(t, typ, addr, amt)
	tx.SignInput(chainID, 0, user[0])
	return tx
}

func makeDefaultCallTx(t *testing.T, typ string, addr, code []byte, amt, gasLim, fee int64) *types.CallTx {
	nonce := getNonce(t, typ, user[0].Address)
	tx := types.NewCallTxWithNonce(user[0].PubKey, addr, code, amt, gasLim, fee, nonce+1)
	tx.Sign(chainID, user[0])
	return tx
}

func makeDefaultNameTx(t *testing.T, typ string, name, value string, amt, fee int64) *types.NameTx {
	nonce := getNonce(t, typ, user[0].Address)
	tx := types.NewNameTxWithNonce(user[0].PubKey, name, value, amt, fee, nonce+1)
	tx.Sign(chainID, user[0])
	return tx
}

//-------------------------------------------------------------------------------
// rpc call wrappers (fail on err)

// get an account's nonce
func getNonce(t *testing.T, typ string, addr []byte) int {
	client := clients[typ]
	ac, err := client.GetAccount(addr)
	if err != nil {
		t.Fatal(err)
	}
	if ac.Account == nil {
		return 0
	}
	return ac.Account.Sequence
}

// get the account
func getAccount(t *testing.T, typ string, addr []byte) *acm.Account {
	client := clients[typ]
	ac, err := client.GetAccount(addr)
	if err != nil {
		t.Fatal(err)
	}
	return ac.Account
}

// sign transaction
func signTx(t *testing.T, typ string, tx types.Tx, privAcc *acm.PrivAccount) types.Tx {
	client := clients[typ]
	signedTx, err := client.SignTx(tx, []*acm.PrivAccount{privAcc})
	if err != nil {
		t.Fatal(err)
	}
	return signedTx.Tx
}

// broadcast transaction
func broadcastTx(t *testing.T, typ string, tx types.Tx) ctypes.Receipt {
	client := clients[typ]
	rec, err := client.BroadcastTx(tx)
	if err != nil {
		t.Fatal(err)
	}
	mempoolCount += 1
	return rec.Receipt
}

// dump all storage for an account. currently unused
func dumpStorage(t *testing.T, addr []byte) ctypes.ResultDumpStorage {
	client := clients["HTTP"]
	resp, err := client.DumpStorage(addr)
	if err != nil {
		t.Fatal(err)
	}
	return *resp
}

func getStorage(t *testing.T, typ string, addr, key []byte) []byte {
	client := clients[typ]
	resp, err := client.GetStorage(addr, key)
	if err != nil {
		t.Fatal(err)
	}
	return resp.Value
}

func callCode(t *testing.T, client cclient.Client, fromAddress, code, data, expected []byte) {
	resp, err := client.CallCode(fromAddress, code, data)
	if err != nil {
		t.Fatal(err)
	}
	ret := resp.Return
	// NOTE: we don't flip memory when it comes out of RETURN (?!)
	if bytes.Compare(ret, LeftPadWord256(expected).Bytes()) != 0 {
		t.Fatalf("Conflicting return value. Got %x, expected %x", ret, expected)
	}
}

func callContract(t *testing.T, client cclient.Client, fromAddress, toAddress, data, expected []byte) {
	resp, err := client.Call(fromAddress, toAddress, data)
	if err != nil {
		t.Fatal(err)
	}
	ret := resp.Return
	// NOTE: we don't flip memory when it comes out of RETURN (?!)
	if bytes.Compare(ret, LeftPadWord256(expected).Bytes()) != 0 {
		t.Fatalf("Conflicting return value. Got %x, expected %x", ret, expected)
	}
}

// get the namereg entry
func getNameRegEntry(t *testing.T, typ string, name string) *types.NameRegEntry {
	client := clients[typ]
	entry, err := client.GetName(name)
	if err != nil {
		t.Fatal(err)
	}
	return entry.Entry
}

//--------------------------------------------------------------------------------
// utility verification function

func checkTx(t *testing.T, fromAddr []byte, priv *acm.PrivAccount, tx *types.SendTx) {
	if bytes.Compare(tx.Inputs[0].Address, fromAddr) != 0 {
		t.Fatal("Tx input addresses don't match!")
	}

	signBytes := acm.SignBytes(chainID, tx)
	in := tx.Inputs[0] //(*types.SendTx).Inputs[0]

	if err := in.ValidateBasic(); err != nil {
		t.Fatal(err)
	}
	// Check signatures
	// acc := getAccount(t, byteAddr)
	// NOTE: using the acc here instead of the in fails; it is nil.
	if !in.PubKey.VerifyBytes(signBytes, in.Signature) {
		t.Fatal(types.ErrTxInvalidSignature)
	}
}

// simple contract returns 5 + 6 = 0xb
func simpleContract() ([]byte, []byte, []byte) {
	// this is the code we want to run when the contract is called
	contractCode := []byte{0x60, 0x5, 0x60, 0x6, 0x1, 0x60, 0x0, 0x52, 0x60, 0x20, 0x60, 0x0, 0xf3}
	// the is the code we need to return the contractCode when the contract is initialized
	lenCode := len(contractCode)
	// push code to the stack
	//code := append([]byte{byte(0x60 + lenCode - 1)}, RightPadWord256(contractCode).Bytes()...)
	code := append([]byte{0x7f}, RightPadWord256(contractCode).Bytes()...)
	// store it in memory
	code = append(code, []byte{0x60, 0x0, 0x52}...)
	// return whats in memory
	//code = append(code, []byte{0x60, byte(32 - lenCode), 0x60, byte(lenCode), 0xf3}...)
	code = append(code, []byte{0x60, byte(lenCode), 0x60, 0x0, 0xf3}...)
	// return init code, contract code, expected return
	return code, contractCode, LeftPadBytes([]byte{0xb}, 32)
}

// simple call contract calls another contract
func simpleCallContract(addr []byte) ([]byte, []byte, []byte) {
	gas1, gas2 := byte(0x1), byte(0x1)
	value := byte(0x1)
	inOff, inSize := byte(0x0), byte(0x0) // no call data
	retOff, retSize := byte(0x0), byte(0x20)
	// this is the code we want to run (call a contract and return)
	contractCode := []byte{0x60, retSize, 0x60, retOff, 0x60, inSize, 0x60, inOff, 0x60, value, 0x73}
	contractCode = append(contractCode, addr...)
	contractCode = append(contractCode, []byte{0x61, gas1, gas2, 0xf1, 0x60, 0x20, 0x60, 0x0, 0xf3}...)

	// the is the code we need to return; the contractCode when the contract is initialized
	// it should copy the code from the input into memory
	lenCode := len(contractCode)
	memOff := byte(0x0)
	inOff = byte(0xc) // length of code before codeContract
	length := byte(lenCode)

	code := []byte{0x60, length, 0x60, inOff, 0x60, memOff, 0x37}
	// return whats in memory
	code = append(code, []byte{0x60, byte(lenCode), 0x60, 0x0, 0xf3}...)
	code = append(code, contractCode...)
	// return init code, contract code, expected return
	return code, contractCode, LeftPadBytes([]byte{0xb}, 32)
}
