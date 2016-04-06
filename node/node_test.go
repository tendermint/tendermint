package node

import (
	"testing"
	"time"

	_ "github.com/eris-ltd/tendermint/config/tendermint_test"
	"github.com/eris-ltd/tendermint/p2p"
)

func TestNodeStartStop(t *testing.T) {
	// Create & start node
	n := NewNodeDefaultPrivVal()
	l := p2p.NewDefaultListener("tcp", config.GetString("node_laddr"))
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
