package monitor_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	tmtypes "github.com/tendermint/tendermint/types"
	monitor "github.com/tendermint/tools/tm-monitor/monitor"
)

func TestNetworkNewBlock(t *testing.T) {
	assert := assert.New(t)

	n := monitor.NewNetwork()

	n.NewBlock(tmtypes.Header{Height: 5, NumTxs: 100})
	assert.Equal(uint64(5), n.Height)
	assert.Equal(0.0, n.AvgBlockTime)
	assert.Equal(0.0, n.AvgTxThroughput)
}

func TestNetworkNewBlockLatency(t *testing.T) {
	assert := assert.New(t)

	n := monitor.NewNetwork()

	n.NewBlockLatency(9000000.0) // nanoseconds
	assert.Equal(0.0, n.AvgBlockLatency)
}

func TestNetworkNodeIsDownThenOnline(t *testing.T) {
	assert := assert.New(t)

	n := monitor.NewNetwork()
	n.NewNode("test")

	n.NodeIsDown("test")
	assert.Equal(0, n.NumNodesMonitoredOnline)
	assert.Equal(monitor.Dead, n.Health)
	n.NodeIsDown("test")
	assert.Equal(0, n.NumNodesMonitoredOnline)

	n.NodeIsOnline("test")
	assert.Equal(1, n.NumNodesMonitoredOnline)
	assert.Equal(monitor.ModerateHealth, n.Health)
	n.NodeIsOnline("test")
	assert.Equal(1, n.NumNodesMonitoredOnline)
}

func TestNetworkNewNode(t *testing.T) {
	assert := assert.New(t)

	n := monitor.NewNetwork()
	assert.Equal(0, n.NumNodesMonitored)
	assert.Equal(0, n.NumNodesMonitoredOnline)
	n.NewNode("test")
	assert.Equal(1, n.NumNodesMonitored)
	assert.Equal(1, n.NumNodesMonitoredOnline)
}

func TestNetworkNodeDeleted(t *testing.T) {
	assert := assert.New(t)

	n := monitor.NewNetwork()
	n.NewNode("test")
	n.NodeDeleted("test")
	assert.Equal(0, n.NumNodesMonitored)
	assert.Equal(0, n.NumNodesMonitoredOnline)
}

func TestNetworkGetHealthString(t *testing.T) {
	assert := assert.New(t)

	n := monitor.NewNetwork()
	assert.Equal("full", n.GetHealthString())
	n.Health = monitor.ModerateHealth
	assert.Equal("moderate", n.GetHealthString())
	n.Health = monitor.Dead
	assert.Equal("dead", n.GetHealthString())
}

func TestNetworkUptime(t *testing.T) {
	assert := assert.New(t)

	n := monitor.NewNetwork()
	assert.Equal(100.0, n.Uptime())
}

func TestNetworkStartTime(t *testing.T) {
	assert := assert.New(t)

	n := monitor.NewNetwork()
	assert.True(n.StartTime().Before(time.Now()))
}
