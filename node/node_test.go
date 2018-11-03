package node

import (
	"context"
	"fmt"
	"os"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/tendermint/tendermint/abci/example/kvstore"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/p2p"
	sm "github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/version"

	cfg "github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/types"

	tmtime "github.com/tendermint/tendermint/types/time"
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

func TestSplitAndTrimEmpty(t *testing.T) {
	testCases := []struct {
		s        string
		sep      string
		cutset   string
		expected []string
	}{
		{"a,b,c", ",", " ", []string{"a", "b", "c"}},
		{" a , b , c ", ",", " ", []string{"a", "b", "c"}},
		{" a, b, c ", ",", " ", []string{"a", "b", "c"}},
		{" a, ", ",", " ", []string{"a"}},
		{"   ", ",", " ", []string{}},
	}

	for _, tc := range testCases {
		assert.Equal(t, tc.expected, splitAndTrimEmpty(tc.s, tc.sep, tc.cutset), "%s", tc.s)
	}
}

func TestNodeDelayedStop(t *testing.T) {
	config := cfg.ResetTestRoot("node_delayed_node_test")
	now := tmtime.Now()

	// create & start node
	n, err := DefaultNewNode(config, log.TestingLogger())
	n.GenesisDoc().GenesisTime = now.Add(5 * time.Second)
	assert.NoError(t, err)

	n.Start()
	startTime := tmtime.Now()
	assert.Equal(t, true, startTime.After(n.GenesisDoc().GenesisTime))
}

func TestNodeSetAppVersion(t *testing.T) {
	config := cfg.ResetTestRoot("node_app_version_test")

	// create & start node
	n, err := DefaultNewNode(config, log.TestingLogger())
	assert.NoError(t, err, "expected no err on DefaultNewNode")

	// default config uses the kvstore app
	var appVersion version.Protocol = kvstore.ProtocolVersion

	// check version is set in state
	state := sm.LoadState(n.stateDB)
	assert.Equal(t, state.Version.Consensus.App, appVersion)

	// check version is set in node info
	assert.Equal(t, n.nodeInfo.(p2p.DefaultNodeInfo).ProtocolVersion.App, appVersion)
}
