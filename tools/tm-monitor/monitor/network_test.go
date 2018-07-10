package monitor_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	monitor "github.com/tendermint/tendermint/tools/tm-monitor/monitor"
	tmtypes "github.com/tendermint/tendermint/types"
)

func TestNetworkNewBlock(t *testing.T) {
	n := monitor.NewNetwork()

	n.NewBlock(tmtypes.Header{Height: 5, NumTxs: 100})
	assert.Equal(t, int64(5), n.Height)
	assert.Equal(t, 0.0, n.AvgBlockTime)
	assert.Equal(t, 0.0, n.AvgTxThroughput)
}

func TestNetworkNewBlockLatency(t *testing.T) {
	n := monitor.NewNetwork()

	n.NewBlockLatency(9000000.0) // nanoseconds
	assert.Equal(t, 0.0, n.AvgBlockLatency)
}

func TestNetworkNodeIsDownThenOnline(t *testing.T) {
	n := monitor.NewNetwork()
	n.NewNode("test")

	n.NodeIsDown("test")
	assert.Equal(t, 0, n.NumNodesMonitoredOnline)
	assert.Equal(t, monitor.Dead, n.Health)
	n.NodeIsDown("test")
	assert.Equal(t, 0, n.NumNodesMonitoredOnline)

	n.NodeIsOnline("test")
	assert.Equal(t, 1, n.NumNodesMonitoredOnline)
	assert.Equal(t, monitor.ModerateHealth, n.Health)
	n.NodeIsOnline("test")
	assert.Equal(t, 1, n.NumNodesMonitoredOnline)
}

func TestNetworkNewNode(t *testing.T) {
	n := monitor.NewNetwork()
	assert.Equal(t, 0, n.NumNodesMonitored)
	assert.Equal(t, 0, n.NumNodesMonitoredOnline)
	n.NewNode("test")
	assert.Equal(t, 1, n.NumNodesMonitored)
	assert.Equal(t, 1, n.NumNodesMonitoredOnline)
}

func TestNetworkNodeDeleted(t *testing.T) {
	n := monitor.NewNetwork()
	n.NewNode("test")
	n.NodeDeleted("test")
	assert.Equal(t, 0, n.NumNodesMonitored)
	assert.Equal(t, 0, n.NumNodesMonitoredOnline)
}

func TestNetworkGetHealthString(t *testing.T) {
	n := monitor.NewNetwork()
	assert.Equal(t, "full", n.GetHealthString())
	n.Health = monitor.ModerateHealth
	assert.Equal(t, "moderate", n.GetHealthString())
	n.Health = monitor.Dead
	assert.Equal(t, "dead", n.GetHealthString())
}

func TestNetworkUptime(t *testing.T) {
	n := monitor.NewNetwork()
	assert.Equal(t, 100.0, n.Uptime())
}

func TestNetworkStartTime(t *testing.T) {
	n := monitor.NewNetwork()
	assert.True(t, n.StartTime().Before(time.Now()))
}
