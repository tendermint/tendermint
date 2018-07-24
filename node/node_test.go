package node

import (
	"context"
	"fmt"
	"os"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/tendermint/tendermint/libs/log"

	cfg "github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/types"
)

func TestNodeStartStop(t *testing.T) {
	config := cfg.ResetTestRoot("node_node_test")

	// create & start node
	n, err := DefaultNewNode(config, log.TestingLogger())
	assert.NoError(t, err, "expected no err on DefaultNewNode")
	err1 := n.Start()
	if err1 != nil {
		t.Error(err1)
	}
	t.Logf("Started node %v", n.sw.NodeInfo())

	// wait for the node to produce a block
	blockCh := make(chan interface{})
	err = n.EventBus().Subscribe(context.Background(), "node_test", types.EventQueryNewBlock, blockCh)
	assert.NoError(t, err)
	select {
	case <-blockCh:
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for the node to produce a block")
	}

	// stop the node
	go func() {
		n.Stop()
	}()

	select {
	case <-n.Quit():
	case <-time.After(5 * time.Second):
		pid := os.Getpid()
		p, err := os.FindProcess(pid)
		if err != nil {
			panic(err)
		}
		err = p.Signal(syscall.SIGABRT)
		fmt.Println(err)
		t.Fatal("timed out waiting for shutdown")
	}
}
