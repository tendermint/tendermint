package node

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/tendermint/tmlibs/log"

	cfg "github.com/tendermint/tendermint/config"
)

func TestNodeStartStop(t *testing.T) {
	config := cfg.ResetTestRoot("node_node_test")

	// Create & start node
	n, err := DefaultNewNode(config, log.TestingLogger())
	assert.NoError(t, err, "expected no err on DefaultNewNode")
	n.Start()
	t.Logf("Started node %v", n.sw.NodeInfo())

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
