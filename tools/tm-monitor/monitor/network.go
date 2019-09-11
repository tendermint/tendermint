package monitor

import (
	"sync"
	"time"

	metrics "github.com/rcrowley/go-metrics"
	tmtypes "github.com/tendermint/tendermint/types"
)

// UptimeData stores data for how long network has been running.
type UptimeData struct {
	StartTime time.Time `json:"start_time"`
	Uptime    float64   `json:"uptime" amino:"unsafe"` // percentage of time we've been healthy, ever

	totalDownTime time.Duration // total downtime (only updated when we come back online)
	wentDown      time.Time
}

// Health describes the health of the network. Note that this applies only to
// the observed nodes, and not to the entire cluster, which may consist of
// thousands of machines. It may change in the future.
type Health int

const (
	// FullHealth means all nodes online, synced, validators making blocks
	FullHealth = Health(0)
	// ModerateHealth means we're making blocks
	ModerateHealth = Health(1)
	// Dead means we're not making blocks due to all validators freezing or crashing
	Dead = Health(2)
)

// Common statistics for network of nodes
type Network struct {
	Height int64 `json:"height"`

	AvgBlockTime      float64 `json:"avg_block_time" amino:"unsafe"` // ms (avg over last minute)
	blockTimeMeter    metrics.Meter
	AvgTxThroughput   float64 `json:"avg_tx_throughput" amino:"unsafe"` // tx/s (avg over last minute)
	txThroughputMeter metrics.Meter
	AvgBlockLatency   float64 `json:"avg_block_latency" amino:"unsafe"` // ms (avg over last minute)
	blockLatencyMeter metrics.Meter

	NumValidators           int `json:"num_validators"`
	NumNodesMonitored       int `json:"num_nodes_monitored"`
	NumNodesMonitoredOnline int `json:"num_nodes_monitored_online"`

	Health Health `json:"health"`

	UptimeData *UptimeData `json:"uptime_data"`

	nodeStatusMap map[string]bool

	mu sync.Mutex
}

func NewNetwork() *Network {
	return &Network{
		blockTimeMeter:    metrics.NewMeter(),
		txThroughputMeter: metrics.NewMeter(),
		blockLatencyMeter: metrics.NewMeter(),
		Health:            FullHealth,
		UptimeData: &UptimeData{
			StartTime: time.Now(),
			Uptime:    100.0,
		},
		nodeStatusMap: make(map[string]bool),
	}
}

func (n *Network) NewBlock(b tmtypes.Header) {
	n.mu.Lock()
	defer n.mu.Unlock()

	if n.Height >= b.Height {
		return
	}

	n.Height = b.Height

	n.blockTimeMeter.Mark(1)
	if n.blockTimeMeter.Rate1() > 0.0 {
		n.AvgBlockTime = (1.0 / n.blockTimeMeter.Rate1()) * 1000 // 1/s to ms
	} else {
		n.AvgBlockTime = 0.0
	}
	n.txThroughputMeter.Mark(b.NumTxs)
	n.AvgTxThroughput = n.txThroughputMeter.Rate1()
}

func (n *Network) NewBlockLatency(l float64) {
	n.mu.Lock()
	defer n.mu.Unlock()

	n.blockLatencyMeter.Mark(int64(l))
	n.AvgBlockLatency = n.blockLatencyMeter.Rate1() / 1000000.0 // ns to ms
}

// RecalculateUptime calculates uptime on demand.
func (n *Network) RecalculateUptime() {
	n.mu.Lock()
	defer n.mu.Unlock()

	since := time.Since(n.UptimeData.StartTime)
	uptime := since - n.UptimeData.totalDownTime
	if n.Health != FullHealth {
		uptime -= time.Since(n.UptimeData.wentDown)
	}
	n.UptimeData.Uptime = (float64(uptime) / float64(since)) * 100.0
}

// NodeIsDown is called when the node disconnects for whatever reason.
// Must be safe to call multiple times.
func (n *Network) NodeIsDown(name string) {
	n.mu.Lock()
	defer n.mu.Unlock()

	if online, ok := n.nodeStatusMap[name]; !ok || online {
		n.nodeStatusMap[name] = false
		n.NumNodesMonitoredOnline--
		n.UptimeData.wentDown = time.Now()
		n.updateHealth()
	}
}

// NodeIsOnline is called when connection to the node is restored.
// Must be safe to call multiple times.
func (n *Network) NodeIsOnline(name string) {
	n.mu.Lock()
	defer n.mu.Unlock()

	if online, ok := n.nodeStatusMap[name]; ok && !online {
		n.nodeStatusMap[name] = true
		n.NumNodesMonitoredOnline++
		n.UptimeData.totalDownTime += time.Since(n.UptimeData.wentDown)
		n.updateHealth()
	}
}

// NewNode is called when the new node is added to the monitor.
func (n *Network) NewNode(name string) {
	n.mu.Lock()
	defer n.mu.Unlock()

	n.NumNodesMonitored++
	n.NumNodesMonitoredOnline++
	n.updateHealth()
}

// NodeDeleted is called when the node is deleted from under the monitor.
func (n *Network) NodeDeleted(name string) {
	n.mu.Lock()
	defer n.mu.Unlock()

	n.NumNodesMonitored--
	n.NumNodesMonitoredOnline--
	n.updateHealth()
}

func (n *Network) updateHealth() {
	// if we are connected to all validators, we're at full health
	// TODO: make sure they're all at the same height (within a block)
	// and all proposing (and possibly validating ) Alternatively, just
	// check there hasn't been a new round in numValidators rounds
	switch {
	case n.NumValidators != 0 && n.NumNodesMonitoredOnline == n.NumValidators:
		n.Health = FullHealth
	case n.NumNodesMonitoredOnline > 0 && n.NumNodesMonitoredOnline <= n.NumNodesMonitored:
		n.Health = ModerateHealth
	default:
		n.Health = Dead
	}
}

func (n *Network) UpdateNumValidatorsForHeight(num int, height int64) {
	n.mu.Lock()
	defer n.mu.Unlock()

	if n.Height <= height {
		n.NumValidators = num
	}
}

func (n *Network) GetHealthString() string {
	switch n.Health {
	case FullHealth:
		return "full"
	case ModerateHealth:
		return "moderate"
	case Dead:
		return "dead"
	default:
		return "undefined"
	}
}

// Uptime returns network's uptime in percentages.
func (n *Network) Uptime() float64 {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.UptimeData.Uptime
}

// StartTime returns time we started monitoring.
func (n *Network) StartTime() time.Time {
	return n.UptimeData.StartTime
}
