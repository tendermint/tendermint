package node

import (
	"testing"
	"time"

	"github.com/tendermint/tendermint/config/tendermint_test"
)

func TestNodeStartStop(t *testing.T) {
	config := tendermint_test.ResetConfig("node_node_test")

	// Create & start node
	n := NewNodeDefault(config)
	n.Start()
	log.Notice("Started node", "nodeInfo", n.sw.NodeInfo())

	// Wait a bit to initialize
	// TODO remove time.Sleep(), make asynchronous.
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
