package rpc

import (
	"bytes"
	"encoding/hex"
	"github.com/tendermint/tendermint/account"
	"github.com/tendermint/tendermint/binary"
	"github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/daemon"
	"github.com/tendermint/tendermint/logger"
	"github.com/tendermint/tendermint/p2p"
	"github.com/tendermint/tendermint/rpc"
	"github.com/tendermint/tendermint/rpc/core"
	"github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/types"
	"io/ioutil"
	"net/http"
	"net/url"
	"testing"
)

var (
	rpcAddr     = "127.0.0.1:8089"
	requestAddr = "http://" + rpcAddr + "/"

	node *daemon.Node

	mempoolCount = 0

	userAddr = "D7DFF9806078899C8DA3FE3633CC0BF3C6C2B1BB"
	userPriv = "FDE3BD94CB327D19464027BA668194C5EFA46AE83E8419D7542CFF41F00C81972239C21C81EA7173A6C489145490C015E05D4B97448933B708A7EC5B7B4921E3"
	userPub  = "2239C21C81EA7173A6C489145490C015E05D4B97448933B708A7EC5B7B4921E3"

	clients  = map[string]rpc.Client{
		"JSONRPC": rpc.NewClient(requestAddr, "JSONRPC"),
		"HTTP":    rpc.NewClient(requestAddr, "HTTP"),
	}
)

func decodeHex(hexStr string) []byte {
	bytes, err := hex.DecodeString(hexStr)
	if err != nil {
		panic(err)
	}
	return bytes
}


// create a new node and sleep forever
func newNode(ready chan struct{}) {
	// Create & start node
	node = daemon.NewNode()
	l := p2p.NewDefaultListener("tcp", config.App().GetString("ListenAddr"), false)
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
	rootDir := ".tendermint"
	config.Init(rootDir)
	app := config.App()
	app.Set("SeedNode", "")
	app.Set("DB.Backend", "memdb")
	app.Set("RPC.HTTP.ListenAddr", rpcAddr)
	app.Set("GenesisFile", rootDir+"/genesis.json")
	app.Set("PrivValidatorFile", rootDir+"/priv_validator.json")
	app.Set("Log.Stdout.Level", "debug")
	config.SetApp(app)
	logger.Reset()

	// Save new priv_validator file.
	priv := &state.PrivValidator{
		Address: decodeHex(userAddr),
		PubKey:  account.PubKeyEd25519(decodeHex(userPub)),
		PrivKey: account.PrivKeyEd25519(decodeHex(userPriv)),
	}
	priv.SetFile(rootDir + "/priv_validator.json")
	priv.Save()

	// start a node
	ready := make(chan struct{})
	go newNode(ready)
	<-ready
}

//-------------------------------------------------------------------------------
// make transactions

// make a send tx (uses get account to figure out the nonce)
func makeSendTx(t *testing.T, typ string, from, to []byte, amt uint64) *types.SendTx {
	acc := getAccount(t, typ, from)
	nonce := 0
	if acc != nil {
		nonce = int(acc.Sequence) + 1
	}
	bytePub, err := hex.DecodeString(userPub)
	if err != nil {
		t.Fatal(err)
	}
	tx := &types.SendTx{
		Inputs: []*types.TxInput{
			&types.TxInput{
				Address:   from,
				Amount:    amt,
				Sequence:  uint(nonce),
				Signature: account.SignatureEd25519{},
				PubKey:    account.PubKeyEd25519(bytePub),
			},
		},
		Outputs: []*types.TxOutput{
			&types.TxOutput{
				Address: to,
				Amount:  amt,
			},
		},
	}
	return tx
}

// make a call tx (uses get account to figure out the nonce)
func makeCallTx(t *testing.T, typ string, from, to, data []byte, amt, gaslim, fee uint64) *types.CallTx {
	acc := getAccount(t, typ, from)
	nonce := 0
	if acc != nil {
		nonce = int(acc.Sequence) + 1
	}

	bytePub, err := hex.DecodeString(userPub)
	if err != nil {
		t.Fatal(err)
	}
	tx := &types.CallTx{
		Input: &types.TxInput{
			Address:   from,
			Amount:    amt,
			Sequence:  uint(nonce),
			Signature: account.SignatureEd25519{},
			PubKey:    account.PubKeyEd25519(bytePub),
		},
		Address:  to,
		GasLimit: gaslim,
		Fee:      fee,
		Data:     data,
	}
	return tx
}

// make transactions
//-------------------------------------------------------------------------------
// rpc call wrappers

// get the account
func getAccount(t *testing.T, typ string, addr []byte) *account.Account {
	client := clients[typ]
	ac, err := client.GetAccount(addr)
	if err != nil {
		t.Fatal(err)
	}
	return ac.Account
}

// make and sign transaction
func signTx(t *testing.T, typ string, fromAddr, toAddr, data []byte, key [64]byte, amt, gaslim, fee uint64) (types.Tx, *account.PrivAccount) {
	var tx types.Tx
	if data == nil {
		tx = makeSendTx(t, typ, fromAddr, toAddr, amt)
	} else {
		tx = makeCallTx(t, typ, fromAddr, toAddr, data, amt, gaslim, fee)
	}

	privAcc := account.GenPrivAccountFromKey(key)
	if bytes.Compare(privAcc.PubKey.Address(), fromAddr) != 0 {
		t.Fatal("Faield to generate correct priv acc")
	}

	client := clients[typ]
	resp, err := client.SignTx(tx, []*account.PrivAccount{privAcc})
	if err != nil {
		t.Fatal(err)
	}
	return resp.Tx, privAcc
}

// create, sign, and broadcast a transaction
func broadcastTx(t *testing.T, typ string, fromAddr, toAddr, data []byte, key [64]byte, amt, gaslim, fee uint64) (types.Tx, core.Receipt) {
	tx, _ := signTx(t, typ, fromAddr, toAddr, data, key, amt, gaslim, fee)
	client := clients[typ]
	resp, err := client.BroadcastTx(tx)
	if err != nil {
		t.Fatal(err)
	}
	return tx, resp.Receipt
}

// dump all storage for an account. currently unused
func dumpStorage(t *testing.T, addr []byte) core.ResponseDumpStorage {
	client := clients["HTTP"]
	resp, err := client.DumpStorage(addr)
	if err != nil {
		t.Fatal(err)
	}
	return *resp
	/*addrString := "\"" + hex.EncodeToString(addr) + "\""
	var response struct {
		Result  core.ResponseDumpStorage `json:"result"`
		Error   string                   `json:"error"`
		Id      string                   `json:"id"`
		JSONRPC string                   `json:"jsonrpc"`
	}
	requestResponse(t, "dump_storage", url.Values{"address": {addrString}}, &response)
	if response.Error != "" {
		t.Fatal(response.Error)
	}*/
}

func getStorage(t *testing.T, typ string, addr, slot []byte) []byte {
	client := clients[typ]
	resp, err := client.GetStorage(addr, slot)
	if err != nil {
		t.Fatal(err)
	}
	return resp.Value
}

//--------------------------------------------------------------------------------
// utility verification function

func checkTx(t *testing.T, fromAddr []byte, priv *account.PrivAccount, tx *types.SendTx) {
	if bytes.Compare(tx.Inputs[0].Address, fromAddr) != 0 {
		t.Fatal("Tx input addresses don't match!")
	}

	signBytes := account.SignBytes(tx)
	in := tx.Inputs[0] //(*types.SendTx).Inputs[0]

	if err := in.ValidateBasic(); err != nil {
		t.Fatal(err)
	}
	// Check signatures
	// acc := getAccount(t, byteAddr)
	// NOTE: using the acc here instead of the in fails; its PubKeyNil ... ?
	if !in.PubKey.VerifyBytes(signBytes, in.Signature) {
		t.Fatal(types.ErrTxInvalidSignature)
	}
}

//--------------------------------------------------------------------------------

func requestResponse(t *testing.T, method string, values url.Values, response interface{}) {
	resp, err := http.PostForm(requestAddr+method, values)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	binary.ReadJSON(response, body, &err)
	if err != nil {
		t.Fatal(err)
	}
}
