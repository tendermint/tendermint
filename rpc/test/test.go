package rpc

import (
	"bytes"
	"encoding/hex"
	"github.com/tendermint/tendermint2/account"
	"github.com/tendermint/tendermint2/binary"
	"github.com/tendermint/tendermint2/config"
	"github.com/tendermint/tendermint2/daemon"
	"github.com/tendermint/tendermint2/logger"
	"github.com/tendermint/tendermint2/p2p"
	"github.com/tendermint/tendermint2/rpc"
	"github.com/tendermint/tendermint2/rpc/core"
	"github.com/tendermint/tendermint2/state"
	"github.com/tendermint/tendermint2/types"
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

	userPub = "2239C21C81EA7173A6C489145490C015E05D4B97448933B708A7EC5B7B4921E3"
)

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

	priv := state.LoadPrivValidator(rootDir + "/priv_validator.json")
	priv.LastHeight = 0
	priv.LastRound = 0
	priv.LastStep = 0
	priv.Save()

	// start a node
	ready := make(chan struct{})
	go newNode(ready)
	<-ready
}

func getAccount(t *testing.T, typ string, addr []byte) *account.Account {
	var client rpc.Client
	switch typ {
	case "JSONRPC":
		client = rpc.NewClient(requestAddr, "JSONRPC")
	case "HTTP":
		client = rpc.NewClient(requestAddr, "HTTP")
	}
	ac, err := client.GetAccount(addr)
	if err != nil {
		t.Fatal(err)
	}
	return ac.Account
}

/*
func getAccount(t *testing.T, typ string, addr []byte) *account.Account {
	var resp *http.Response
	var err error
	switch typ {
	case "JSONRPC":
		s := rpc.JSONRPC{
			JSONRPC: "2.0",
			Method:  "get_account",
			Params:  []interface{}{hex.EncodeToString(addr)},
			Id:      0,
		}
		b, err := json.Marshal(s)
		if err != nil {
			t.Fatal(err)
		}
		buf := bytes.NewBuffer(b)
		resp, err = http.Post(requestAddr, "text/json", buf)
	case "HTTP":
		resp, err = http.PostForm(requestAddr+"get_account",
			url.Values{"address": {"\"" + (hex.EncodeToString(addr)) + "\""}})
	}
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	var response struct {
		Result  core.ResponseGetAccount `json:"result"`
		Error   string                  `json:"error"`
		Id      string                  `json:"id"`
		JSONRPC string                  `json:"jsonrpc"`
	}
	binary.ReadJSON(&response, body, &err)
	if err != nil {
		t.Fatal(err)
	}
	return response.Result.Account
}*/

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

func signTx(t *testing.T, typ string, fromAddr, toAddr, data []byte, key [64]byte, amt, gaslim, fee uint64) (types.Tx, *account.PrivAccount) {
	var tx types.Tx
	if data == nil {
		tx = makeSendTx(t, typ, fromAddr, toAddr, amt)
	} else {
		tx = makeCallTx(t, typ, fromAddr, toAddr, data, amt, gaslim, fee)
	}

	n, w := new(int64), new(bytes.Buffer)
	var err error
	binary.WriteJSON(tx, w, n, &err)
	if err != nil {
		t.Fatal(err)
	}
	b := w.Bytes()

	privAcc := account.GenPrivAccountFromKey(key)
	if bytes.Compare(privAcc.PubKey.Address(), fromAddr) != 0 {
		t.Fatal("Faield to generate correct priv acc")
	}
	w = new(bytes.Buffer)
	binary.WriteJSON([]*account.PrivAccount{privAcc}, w, n, &err)
	if err != nil {
		t.Fatal(err)
	}

	var response struct {
		Result  core.ResponseSignTx `json:"result"`
		Error   string              `json:"error"`
		Id      string              `json:"id"`
		JSONRPC string              `json:"jsonrpc"`
	}
	requestResponse(t, "unsafe/sign_tx", url.Values{"tx": {string(b)}, "privAccounts": {string(w.Bytes())}}, &response)
	if response.Error != "" {
		t.Fatal(response.Error)
	}
	result := response.Result
	return result.Tx, privAcc
}

func broadcastTx(t *testing.T, typ string, fromAddr, toAddr, data []byte, key [64]byte, amt, gaslim, fee uint64) (types.Tx, core.Receipt) {
	tx, _ := signTx(t, typ, fromAddr, toAddr, data, key, amt, gaslim, fee)

	n, w := new(int64), new(bytes.Buffer)
	var err error
	binary.WriteJSON(tx, w, n, &err)
	if err != nil {
		t.Fatal(err)
	}
	b := w.Bytes()

	var response struct {
		Result  core.ResponseBroadcastTx `json:"result"`
		Error   string                   `json:"error"`
		Id      string                   `json:"id"`
		JSONRPC string                   `json:"jsonrpc"`
	}
	requestResponse(t, "broadcast_tx", url.Values{"tx": {string(b)}}, &response)
	if response.Error != "" {
		t.Fatal(response.Error)
	}
	return tx, response.Result.Receipt
}

func dumpStorage(t *testing.T, addr []byte) core.ResponseDumpStorage {
	addrString := "\"" + hex.EncodeToString(addr) + "\""
	var response struct {
		Result  core.ResponseDumpStorage `json:"result"`
		Error   string                   `json:"error"`
		Id      string                   `json:"id"`
		JSONRPC string                   `json:"jsonrpc"`
	}
	requestResponse(t, "dump_storage", url.Values{"address": {addrString}}, &response)
	if response.Error != "" {
		t.Fatal(response.Error)
	}
	return response.Result
}

func getStorage(t *testing.T, addr, slot []byte) []byte {
	addrString := "\"" + hex.EncodeToString(addr) + "\""
	slotString := "\"" + hex.EncodeToString(slot) + "\""
	var response struct {
		Result  core.ResponseGetStorage `json:"result"`
		Error   string                  `json:"error"`
		Id      string                  `json:"id"`
		JSONRPC string                  `json:"jsonrpc"`
	}
	requestResponse(t, "get_storage", url.Values{"address": {addrString}, "storage": {slotString}}, &response)
	if response.Error != "" {
		t.Fatal(response.Error)
	}
	return response.Result.Value
}

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
