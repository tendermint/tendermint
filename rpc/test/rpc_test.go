package rpc

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/tendermint/tendermint/binary"
	"github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/daemon"
	"github.com/tendermint/tendermint/p2p"
	"github.com/tendermint/tendermint/rpc"
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
	config.SetApp(app)
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

func TestGetAccount(t *testing.T) {
	byteAddr, _ := hex.DecodeString(userAddr)
	resp, err := http.PostForm(requestAddr+"get_account",
		url.Values{"address": {string(byteAddr)}})
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
	}
	fmt.Println(string(body))
	binary.ReadJSON(&status, body, &err)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Compare(status.Data.Account.Address, byteAddr) != 0 {
		t.Fatalf("Failed to get correct account. Got %x, expected %x", status.Data.Account.Address, byteAddr)
	}

}

func TestSignedTx(t *testing.T) {

}

/*
	acc := mint.MempoolReactor.Mempool.GetState().GetAccount(mint.priv.Address)
	nonce := 0
	if acc != nil {
		nonce = int(acc.Sequence) + 1
	}

	amtInt, err := strconv.Atoi(amt)
	if err != nil {
		return "", err
	}
	amtUint64 := uint64(amtInt)

	tx := &blk.SendTx{
		Inputs: []*blk.TxInput{
			&blk.TxInput{
				Address:   mint.priv.Address,
				Amount:    amtUint64,
				Sequence:  uint(nonce),
				Signature: account.SignatureEd25519{},
				PubKey:    mint.priv.PubKey,
			},
		},
		Outputs: []*blk.TxOutput{
			&blk.TxOutput{
				Address: addrB,
				Amount:  amtUint64,
			},
		},
	}
	tx.Inputs[0].Signature = mint.priv.PrivKey.Sign(account.SignBytes(tx))
	err = mint.MempoolReactor.BroadcastTx(tx)
	return hex.EncodeToString(merkle.HashFromBinary(tx)), err

*/
