package main_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	em "github.com/tendermint/go-event-meter"
	monitor "github.com/tendermint/netmon/tm-monitor"
	mock "github.com/tendermint/netmon/tm-monitor/mock"
	tmtypes "github.com/tendermint/tendermint/types"
)

func TestNodeStartStop(t *testing.T) {
	assert := assert.New(t)

	n, _ := setupNode(t)
	assert.Equal(true, n.Online)

	n.Stop()
}

func TestNodeNewBlockReceived(t *testing.T) {
	assert := assert.New(t)

	blockCh := make(chan tmtypes.Header, 100)
	n, emMock := setupNode(t)
	n.SendBlocksTo(blockCh)

	blockHeader := &tmtypes.Header{Height: 5}
	emMock.Call("eventCallback", &em.EventMetric{}, tmtypes.EventDataNewBlockHeader{blockHeader})

	assert.Equal(uint64(5), n.Height)
	assert.Equal(*blockHeader, <-blockCh)
}

func TestNodeNewBlockLatencyReceived(t *testing.T) {
	assert := assert.New(t)

	blockLatencyCh := make(chan float64, 100)
	n, emMock := setupNode(t)
	n.SendBlockLatenciesTo(blockLatencyCh)

	emMock.Call("latencyCallback", 1000000.0)

	assert.Equal(1.0, n.BlockLatency)
	assert.Equal(1000000.0, <-blockLatencyCh)
}

func TestNodeConnectionLost(t *testing.T) {
	assert := assert.New(t)

	disconnectCh := make(chan bool, 100)
	n, emMock := setupNode(t)
	n.NotifyAboutDisconnects(disconnectCh)

	emMock.Call("disconnectCallback")

	assert.Equal(true, <-disconnectCh)
	assert.Equal(false, <-disconnectCh)

	// we're back in a race
	assert.Equal(true, n.Online)
}

func setupNode(t *testing.T) (n *monitor.Node, emMock *mock.EventMeter) {
	emMock = &mock.EventMeter{}
	n = monitor.NewNodeWithEventMeter("tcp://127.0.0.1:46657", emMock)

	err := n.Start()
	require.Nil(t, err)
	return
}
