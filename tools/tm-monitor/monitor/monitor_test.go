package monitor_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	amino "github.com/tendermint/go-amino"
	"github.com/tendermint/tendermint/crypto/ed25519"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
	mock "github.com/tendermint/tendermint/tools/tm-monitor/mock"
	monitor "github.com/tendermint/tendermint/tools/tm-monitor/monitor"
	tmtypes "github.com/tendermint/tendermint/types"
)

func TestMonitorUpdatesNumberOfValidators(t *testing.T) {
	m := startMonitor(t)
	defer m.Stop()

	n, _ := createValidatorNode(t)
	m.Monitor(n)
	assert.Equal(t, 1, m.Network.NumNodesMonitored)
	assert.Equal(t, 1, m.Network.NumNodesMonitoredOnline)

	time.Sleep(1 * time.Second)

	// DATA RACE
	// assert.Equal(t, 1, m.Network.NumValidators())
}

func TestMonitorRecalculatesNetworkUptime(t *testing.T) {
	m := startMonitor(t)
	defer m.Stop()
	assert.Equal(t, 100.0, m.Network.Uptime())

	n, _ := createValidatorNode(t)
	m.Monitor(n)

	m.Network.NodeIsDown(n.Name) // simulate node failure
	time.Sleep(200 * time.Millisecond)
	m.Network.NodeIsOnline(n.Name)
	time.Sleep(1 * time.Second)

	assert.True(t, m.Network.Uptime() < 100.0, "Uptime should be less than 100%")
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
	pubKey := ed25519.GenPrivKey().PubKey()
	stubs["validators"] = ctypes.ResultValidators{BlockHeight: blockHeight, Validators: []*tmtypes.Validator{tmtypes.NewValidator(pubKey, 0)}}
	stubs["status"] = ctypes.ResultStatus{ValidatorInfo: ctypes.ValidatorInfo{PubKey: pubKey}}
	cdc := amino.NewCodec()
	rpcClientMock := &mock.RpcClient{Stubs: stubs}
	rpcClientMock.SetCodec(cdc)

	n = monitor.NewNodeWithEventMeterAndRpcClient("tcp://127.0.0.1:26657", emMock, rpcClientMock)
	return
}
