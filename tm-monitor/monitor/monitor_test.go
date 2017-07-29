package monitor_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	crypto "github.com/tendermint/go-crypto"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
	tmtypes "github.com/tendermint/tendermint/types"
	mock "github.com/tendermint/tools/tm-monitor/mock"
	monitor "github.com/tendermint/tools/tm-monitor/monitor"
)

func TestMonitorUpdatesNumberOfValidators(t *testing.T) {
	assert := assert.New(t)

	m := startMonitor(t)
	defer m.Stop()

	n, _ := createValidatorNode(t)
	m.Monitor(n)
	assert.Equal(1, m.Network.NumNodesMonitored)
	assert.Equal(1, m.Network.NumNodesMonitoredOnline)

	time.Sleep(1 * time.Second)

	assert.Equal(1, m.Network.NumValidators)
}

func TestMonitorRecalculatesNetworkUptime(t *testing.T) {
	assert := assert.New(t)

	m := startMonitor(t)
	defer m.Stop()
	assert.Equal(100.0, m.Network.Uptime())

	n, _ := createValidatorNode(t)
	m.Monitor(n)

	m.Network.NodeIsDown(n.Name) // simulate node failure
	time.Sleep(200 * time.Millisecond)
	m.Network.NodeIsOnline(n.Name)
	time.Sleep(1 * time.Second)

	assert.True(m.Network.Uptime() < 100.0, "Uptime should be less than 100%")
}

func startMonitor(t *testing.T) *monitor.Monitor {
	m := monitor.NewMonitor(
		monitor.SetNumValidatorsUpdateInterval(200*time.Millisecond),
		monitor.RecalculateNetworkUptimeEvery(200*time.Millisecond),
	)
	err := m.Start()
	require.Nil(t, err)
	return m
}

func createValidatorNode(t *testing.T) (n *monitor.Node, emMock *mock.EventMeter) {
	emMock = &mock.EventMeter{}

	stubs := make(map[string]interface{})
	pubKey := crypto.GenPrivKeyEd25519().PubKey()
	stubs["validators"] = ctypes.ResultValidators{BlockHeight: blockHeight, Validators: []*tmtypes.Validator{tmtypes.NewValidator(pubKey, 0)}}
	stubs["status"] = ctypes.ResultStatus{PubKey: pubKey}
	rpcClientMock := &mock.RpcClient{stubs}

	n = monitor.NewNodeWithEventMeterAndRpcClient("tcp://127.0.0.1:46657", emMock, rpcClientMock)
	return
}
