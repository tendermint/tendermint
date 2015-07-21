package node

import (
	"testing"
	"time"

	cfg "github.com/tendermint/tendermint/config"
	tmcfg "github.com/tendermint/tendermint/config/tendermint"
	"github.com/tendermint/tendermint/p2p"
)

func init() {
	config := tmcfg.GetConfig("")
	cfg.ApplyConfig(config)
}

func TestNodeStartStop(t *testing.T) {
	// Create & start node
	n := NewNode()
	l := p2p.NewDefaultListener("tcp", config.GetString("node_laddr"), false)
	n.AddListener(l)
	n.Start()
	log.Notice("Started node", "nodeInfo", n.sw.NodeInfo())
	time.Sleep(time.Second * 2)
	ch := make(chan struct{}, 1)
	go func() {
		n.Stop()
		ch <- struct{}{}
	}()
	ticker := time.NewTicker(time.Second * 5)
	select {
	case <-ch:
	case <-ticker.C:
		t.Fatal("timed out waiting for shutdown")
	}
}
