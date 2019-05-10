package monitor_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	amino "github.com/tendermint/go-amino"
	"github.com/tendermint/tendermint/crypto/ed25519"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
	em "github.com/tendermint/tendermint/tools/tm-monitor/eventmeter"
	mock "github.com/tendermint/tendermint/tools/tm-monitor/mock"
	monitor "github.com/tendermint/tendermint/tools/tm-monitor/monitor"
	tmtypes "github.com/tendermint/tendermint/types"
)

const (
	blockHeight = int64(1)
)

func TestNodeStartStop(t *testing.T) {
	n, _ := startValidatorNode(t)
	defer n.Stop()

	assert.Equal(t, true, n.Online)
	assert.Equal(t, true, n.IsValidator)
}

func TestNodeNewBlockReceived(t *testing.T) {
	blockCh := make(chan tmtypes.Header, 100)
	n, emMock := startValidatorNode(t)
	defer n.Stop()
	n.SendBlocksTo(blockCh)

	blockHeader := tmtypes.Header{Height: 5}
	emMock.Call("eventCallback", &em.EventMetric{}, tmtypes.EventDataNewBlockHeader{Header: blockHeader})

	assert.Equal(t, int64(5), n.Height)
	assert.Equal(t, blockHeader, <-blockCh)
}

func TestNodeNewBlockLatencyReceived(t *testing.T) {
	blockLatencyCh := make(chan float64, 100)
	n, emMock := startValidatorNode(t)
	defer n.Stop()
	n.SendBlockLatenciesTo(blockLatencyCh)

	emMock.Call("latencyCallback", 1000000.0)

	assert.Equal(t, 1.0, n.BlockLatency)
	assert.Equal(t, 1000000.0, <-blockLatencyCh)
}

func TestNodeConnectionLost(t *testing.T) {
	disconnectCh := make(chan bool, 100)
	n, emMock := startValidatorNode(t)
	defer n.Stop()
	n.NotifyAboutDisconnects(disconnectCh)

	emMock.Call("disconnectCallback")

	assert.Equal(t, true, <-disconnectCh)
	assert.Equal(t, false, n.Online)
}

func TestNumValidators(t *testing.T) {
	n, _ := startValidatorNode(t)
	defer n.Stop()

	height, num, err := n.NumValidators()
	assert.Nil(t, err)
	assert.Equal(t, blockHeight, height)
	assert.Equal(t, 1, num)
}

func startValidatorNode(t *testing.T) (n *monitor.Node, emMock *mock.EventMeter) {
	emMock = &mock.EventMeter{}

	stubs := make(map[string]interface{})
	pubKey := ed25519.GenPrivKey().PubKey()
	stubs["validators"] = ctypes.ResultValidators{BlockHeight: blockHeight, Validators: []*tmtypes.Validator{tmtypes.NewValidator(pubKey, 0)}}
	stubs["status"] = ctypes.ResultStatus{ValidatorInfo: ctypes.ValidatorInfo{PubKey: pubKey}}
	cdc := amino.NewCodec()
	rpcClientMock := &mock.RpcClient{Stubs: stubs}
	rpcClientMock.SetCodec(cdc)

	n = monitor.NewNodeWithEventMeterAndRpcClient("tcp://127.0.0.1:26657", emMock, rpcClientMock)

	err := n.Start()
	require.Nil(t, err)
	return
}
