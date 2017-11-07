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

	ticker := time.NewTicker(10 * time.Millisecond)
	select {
	case <-ticker.C:
		if n.IsRunning() {
			return
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for start")
	}

	go func() {
		n.Stop()
	}()

	select {
	case <-n.Quit:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for shutdown")
	}
}
