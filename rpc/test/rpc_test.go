package rpc

import (
	"encoding/json"
	"fmt"
	"github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/daemon"
	"github.com/tendermint/tendermint/p2p"
	"github.com/tendermint/tendermint/rpc"
	"io/ioutil"
	"net/http"
	"testing"
	"time"
)

var (
	rpcAddr     = "127.0.0.1:8089"
	requestAddr = "http://" + rpcAddr + "/"
	chainId     string
	node        *daemon.Node
)

func newNode() {
	// Create & start node
	node = daemon.NewNode()
	l := p2p.NewDefaultListener("tcp", config.App().GetString("ListenAddr"), false)
	node.AddListener(l)
	node.Start()

	// Run the RPC server.
	node.StartRpc()

	// Sleep forever
	ch := make(chan struct{})
	<-ch
}

func init() {
	app := config.App()
	app.Set("SeedNode", "")
	app.Set("DB.Backend", "memdb")
	app.Set("RPC.HTTP.ListenAddr", rpcAddr)
	config.SetApp(app)
	// start a node
	go newNode()
	time.Sleep(2 * time.Second)
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
	if status.Data.ChainId != node.Switch().GetChainId() {
		t.Fatal(fmt.Errorf("ChainId mismatch: got %s expected %s", status.Data.ChainId, node.Switch().GetChainId()))
	}
}
