package monitor_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	crypto "github.com/tendermint/go-crypto"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
	tmtypes "github.com/tendermint/tendermint/types"
	em "github.com/tendermint/tools/tm-monitor/eventmeter"
	mock "github.com/tendermint/tools/tm-monitor/mock"
	monitor "github.com/tendermint/tools/tm-monitor/monitor"
)

const (
	blockHeight = 1
)

func TestNodeStartStop(t *testing.T) {
	assert := assert.New(t)

	n, _ := startValidatorNode(t)
	defer n.Stop()

	assert.Equal(true, n.Online)
	assert.Equal(true, n.IsValidator)
}

func TestNodeNewBlockReceived(t *testing.T) {
	assert := assert.New(t)

	blockCh := make(chan tmtypes.Header, 100)
	n, emMock := startValidatorNode(t)
	defer n.Stop()
	n.SendBlocksTo(blockCh)

	blockHeader := &tmtypes.Header{Height: 5}
	emMock.Call("eventCallback", &em.EventMetric{}, tmtypes.TMEventData{tmtypes.EventDataNewBlockHeader{blockHeader}})

	assert.Equal(uint64(5), n.Height)
	assert.Equal(*blockHeader, <-blockCh)
}

func TestNodeNewBlockLatencyReceived(t *testing.T) {
	assert := assert.New(t)

	blockLatencyCh := make(chan float64, 100)
	n, emMock := startValidatorNode(t)
	defer n.Stop()
	n.SendBlockLatenciesTo(blockLatencyCh)

	emMock.Call("latencyCallback", 1000000.0)

	assert.Equal(1.0, n.BlockLatency)
	assert.Equal(1000000.0, <-blockLatencyCh)
}

func TestNodeConnectionLost(t *testing.T) {
	assert := assert.New(t)

	disconnectCh := make(chan bool, 100)
	n, emMock := startValidatorNode(t)
	defer n.Stop()
	n.NotifyAboutDisconnects(disconnectCh)

	emMock.Call("disconnectCallback")

	assert.Equal(true, <-disconnectCh)
	assert.Equal(false, n.Online)
}

func TestNumValidators(t *testing.T) {
	assert := assert.New(t)

	n, _ := startValidatorNode(t)
	defer n.Stop()

	height, num, err := n.NumValidators()
	assert.Nil(err)
	assert.Equal(uint64(blockHeight), height)
	assert.Equal(1, num)
}

func startValidatorNode(t *testing.T) (n *monitor.Node, emMock *mock.EventMeter) {
	emMock = &mock.EventMeter{}

	stubs := make(map[string]interface{})
	pubKey := crypto.GenPrivKeyEd25519().PubKey()
	stubs["validators"] = ctypes.ResultValidators{BlockHeight: blockHeight, Validators: []*tmtypes.Validator{tmtypes.NewValidator(pubKey, 0)}}
	stubs["status"] = ctypes.ResultStatus{PubKey: pubKey}
	rpcClientMock := &mock.RpcClient{stubs}

	n = monitor.NewNodeWithEventMeterAndRpcClient("tcp://127.0.0.1:46657", emMock, rpcClientMock)

	err := n.Start()
	require.Nil(t, err)
	return
}
