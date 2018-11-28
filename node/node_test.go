package node

import (
	"context"
	"fmt"
	"net"
	"os"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tendermint/tendermint/abci/example/kvstore"
	cfg "github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/crypto/ed25519"
	cmn "github.com/tendermint/tendermint/libs/common"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/p2p"
	"github.com/tendermint/tendermint/privval"
	sm "github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/types"
	tmtime "github.com/tendermint/tendermint/types/time"
	"github.com/tendermint/tendermint/version"
)

func TestNodeStartStop(t *testing.T) {
	config := cfg.ResetTestRoot("node_node_test")

	// create & start node
	n, err := DefaultNewNode(config, log.TestingLogger())
	require.NoError(t, err)
	err = n.Start()
	require.NoError(t, err)

	t.Logf("Started node %v", n.sw.NodeInfo())

	// wait for the node to produce a block
	blockCh := make(chan interface{})
	err = n.EventBus().Subscribe(context.Background(), "node_test", types.EventQueryNewBlock, blockCh)
	require.NoError(t, err)
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
	require.NoError(t, err)

	n.Start()
	startTime := tmtime.Now()
	assert.Equal(t, true, startTime.After(n.GenesisDoc().GenesisTime))
}

func TestNodeSetAppVersion(t *testing.T) {
	config := cfg.ResetTestRoot("node_app_version_test")

	// create & start node
	n, err := DefaultNewNode(config, log.TestingLogger())
	require.NoError(t, err)

	// default config uses the kvstore app
	var appVersion version.Protocol = kvstore.ProtocolVersion

	// check version is set in state
	state := sm.LoadState(n.stateDB)
	assert.Equal(t, state.Version.Consensus.App, appVersion)

	// check version is set in node info
	assert.Equal(t, n.nodeInfo.(p2p.DefaultNodeInfo).ProtocolVersion.App, appVersion)
}

func TestNodeSetPrivValTCP(t *testing.T) {
	addr := "tcp://" + testFreeAddr(t)

	config := cfg.ResetTestRoot("node_priv_val_tcp_test")
	config.BaseConfig.PrivValidatorListenAddr = addr

	rs := privval.NewRemoteSigner(
		log.TestingLogger(),
		config.ChainID(),
		addr,
		types.NewMockPV(),
		ed25519.GenPrivKey(),
	)
	privval.RemoteSignerConnDeadline(5 * time.Millisecond)(rs)
	go func() {
		err := rs.Start()
		if err != nil {
			panic(err)
		}
	}()
	defer rs.Stop()

	n, err := DefaultNewNode(config, log.TestingLogger())
	require.NoError(t, err)
	assert.IsType(t, &privval.TCPVal{}, n.PrivValidator())
}

// address without a protocol must result in error
func TestPrivValidatorListenAddrNoProtocol(t *testing.T) {
	addrNoPrefix := testFreeAddr(t)

	config := cfg.ResetTestRoot("node_priv_val_tcp_test")
	config.BaseConfig.PrivValidatorListenAddr = addrNoPrefix

	_, err := DefaultNewNode(config, log.TestingLogger())
	assert.Error(t, err)
}

func TestNodeSetPrivValIPC(t *testing.T) {
	tmpfile := "/tmp/kms." + cmn.RandStr(6) + ".sock"
	defer os.Remove(tmpfile) // clean up

	config := cfg.ResetTestRoot("node_priv_val_tcp_test")
	config.BaseConfig.PrivValidatorListenAddr = "unix://" + tmpfile

	rs := privval.NewIPCRemoteSigner(
		log.TestingLogger(),
		config.ChainID(),
		tmpfile,
		types.NewMockPV(),
	)
	privval.IPCRemoteSignerConnDeadline(3 * time.Second)(rs)

	done := make(chan struct{})
	go func() {
		defer close(done)
		n, err := DefaultNewNode(config, log.TestingLogger())
		require.NoError(t, err)
		assert.IsType(t, &privval.IPCVal{}, n.PrivValidator())
	}()

	err := rs.Start()
	require.NoError(t, err)
	defer rs.Stop()

	<-done
}

// testFreeAddr claims a free port so we don't block on listener being ready.
func testFreeAddr(t *testing.T) string {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer ln.Close()

	return fmt.Sprintf("127.0.0.1:%d", ln.Addr().(*net.TCPAddr).Port)
}
