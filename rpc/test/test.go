package rpc

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/tendermint/tendermint/account"
	"github.com/tendermint/tendermint/binary"
	"github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/daemon"
	"github.com/tendermint/tendermint/logger"
	"github.com/tendermint/tendermint/p2p"
	"github.com/tendermint/tendermint/rpc"
	"github.com/tendermint/tendermint/types"
	"io/ioutil"
	"net/http"
	"net/url"
	"testing"
)

var (
	rpcAddr     = "127.0.0.1:8089"
	requestAddr = "http://" + rpcAddr + "/"

	chainId string

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
	node.StartRpc()
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
	logger.InitLog()
	// start a node
	ready := make(chan struct{})
	go newNode(ready)
	<-ready
}

func getAccount(t *testing.T, typ string, addr []byte) *account.Account {
	var resp *http.Response
	var err error
	switch typ {
	case "JSONRPC":
		s := rpc.JsonRpc{
			JsonRpc: "2.0",
			Method:  "get_account",
			Params:  []string{"0x" + hex.EncodeToString(addr)},
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
			url.Values{"address": {string(addr)}})
	}
	fmt.Println("RESPONSE:", resp)
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
		Data   rpc.ResponseGetAccount
		Error  string
	}
	fmt.Println(string(body))
	binary.ReadJSON(&status, body, &err)
	if err != nil {
		t.Fatal(err)
	}
	return status.Data.Account
}

func makeTx(t *testing.T, typ string, from, to []byte, amt uint64) *types.SendTx {
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

func requestResponse(t *testing.T, method string, values url.Values, status interface{}) {
	resp, err := http.PostForm(requestAddr+method, values)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println(string(body))
	binary.ReadJSON(status, body, &err)
	if err != nil {
		t.Fatal(err)
	}
}

func signTx(t *testing.T, typ string, fromAddr, toAddr []byte, key [64]byte, amt uint64) (*types.SendTx, *account.PrivAccount) {
	tx := makeTx(t, typ, fromAddr, toAddr, amt)

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

	var status struct {
		Status string
		Data   rpc.ResponseSignTx
		Error  string
	}
	requestResponse(t, "unsafe/sign_tx", url.Values{"tx": {string(b)}, "privAccounts": {string(w.Bytes())}}, &status)
	if status.Status == "ERROR" {
		t.Fatal(status.Error)
	}
	response := status.Data
	tx = response.Tx.(*types.SendTx)
	return tx, privAcc
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
	fmt.Println(priv.PubKey, in.PubKey)
	// Check signatures
	// acc := getAccount(t, byteAddr)
	// NOTE: using the acc here instead of the in fails; its PubKeyNil ... ?
	if !in.PubKey.VerifyBytes(signBytes, in.Signature) {
		t.Fatal(types.ErrTxInvalidSignature)
	}
}
