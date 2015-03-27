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
	chainId     string
	node        *daemon.Node
	userAddr    = "D7DFF9806078899C8DA3FE3633CC0BF3C6C2B1BB"
	userPriv    = "FDE3BD94CB327D19464027BA668194C5EFA46AE83E8419D7542CFF41F00C81972239C21C81EA7173A6C489145490C015E05D4B97448933B708A7EC5B7B4921E3"

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

func TestSayHello(t *testing.T) {
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
		Data   rpc.ResponseStatus
	}
	err = json.Unmarshal(body, &status)
	if err != nil {
		t.Fatal(err)
	}
	if status.Data.Network != config.App().GetString("Network") {
		t.Fatal(fmt.Errorf("Network mismatch: got %s expected %s", status.Data.Network, config.App().Get("Network")))
	}
}

func TestGenPriv(t *testing.T) {
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
		Data   rpc.ResponseGenPrivAccount
	}
	binary.ReadJSON(&status, body, &err)
	if err != nil {
		t.Fatal(err)
	}
	if len(status.Data.PrivAccount.Address) == 0 {
		t.Fatal("Failed to generate an address")
	}
}

func getAccount(t *testing.T, addr []byte) *account.Account {
	resp, err := http.PostForm(requestAddr+"get_account",
		url.Values{"address": {string(addr)}})
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

func TestGetAccount(t *testing.T) {
	byteAddr, _ := hex.DecodeString(userAddr)
	acc := getAccount(t, byteAddr)
	if bytes.Compare(acc.Address, byteAddr) != 0 {
		t.Fatalf("Failed to get correct account. Got %x, expected %x", acc.Address, byteAddr)
	}

}

func makeTx(t *testing.T, from, to []byte, amt uint64) *types.SendTx {
	acc := getAccount(t, from)
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

func TestSignedTx(t *testing.T) {
	byteAddr, _ := hex.DecodeString(userAddr)
	var byteKey [64]byte
	oh, _ := hex.DecodeString(userPriv)
	copy(byteKey[:], oh)

	amt := uint64(100)
	toAddr := []byte{20, 143, 25, 63, 16, 177, 83, 29, 91, 91, 54, 23, 233, 46, 190, 121, 122, 34, 86, 54}
	tx := makeTx(t, byteAddr, toAddr, amt)

	/*b, err := json.Marshal(rpc.InterfaceType{types.TxTypeSend, tx})
	if err != nil {
		t.Fatal(err)
	}*/

	n := new(int64)
	var err error
	w := new(bytes.Buffer)
	binary.WriteJSON(tx, w, n, &err)
	if err != nil {
		t.Fatal(err)
	}
	b := w.Bytes()

	privAcc := account.GenPrivAccountFromKey(byteKey)
	if bytes.Compare(privAcc.PubKey.Address(), byteAddr) != 0 {
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
	//tx = status.Data.(rpc.ResponseSignTx).Tx.(*types.SendTx)
	tx = response.Tx.(*types.SendTx)
	if bytes.Compare(tx.Inputs[0].Address, byteAddr) != 0 {
		t.Fatal("Tx input addresses don't match!")
	}

	signBytes := account.SignBytes(tx)
	in := tx.Inputs[0] //(*types.SendTx).Inputs[0]

	if err := in.ValidateBasic(); err != nil {
		t.Fatal(err)
	}
	fmt.Println(privAcc.PubKey, in.PubKey)
	// Check signatures
	// acc := getAccount(t, byteAddr)
	// NOTE: using the acc here instead of the in fails; its PubKeyNil ... ?
	if !in.PubKey.VerifyBytes(signBytes, in.Signature) {
		t.Fatal(types.ErrTxInvalidSignature)
	}
}

func TestBroadcastTx(t *testing.T) {
	/*
		byteAddr, _ := hex.DecodeString(userAddr)

		amt := uint64(100)
		toAddr := []byte{20, 143, 25, 63, 16, 177, 83, 29, 91, 91, 54, 23, 233, 46, 190, 121, 122, 34, 86, 54}
		tx := makeTx(t, byteAddr, toAddr, amt)

		b, err := json.Marshal([]interface{}{types.TxTypeSend, tx})
		if err != nil {
			t.Fatal(err)
		}

		var status struct {
			Status string
			Data   rpc.ResponseSignTx
		}
		// TODO ...
	*/
}

/*tx.Inputs[0].Signature = mint.priv.PrivKey.Sign(account.SignBytes(tx))
err = mint.MempoolReactor.BroadcastTx(tx)
return hex.EncodeToString(merkle.HashFromBinary(tx)), err*/
