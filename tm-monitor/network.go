package main

import (
	"sync"
	"time"

	metrics "github.com/rcrowley/go-metrics"
	tmtypes "github.com/tendermint/tendermint/types"
)

// UptimeData stores data for how long network has been running
type UptimeData struct {
	StartTime time.Time `json:"start_time"`
	Uptime    float64   `json:"uptime" wire:"unsafe"` // percentage of time we've been `ModerateHealth`y, ever

	totalDownTime time.Duration // total downtime (only updated when we come back online)
	wentDown      time.Time
}

type Health int

const (
	// FullHealth means all validators online, synced, making blocks
	FullHealth = iota
	// ModerateHealth means we're making blocks
	ModerateHealth
	// Dead means we're not making blocks due to all validators freezing or crashing
	Dead
)

// Common statistics for network of nodes
type Network struct {
	Height uint64 `json:"height"`

	AvgBlockTime      float64 `json:"avg_block_time" wire:"unsafe"` // ms (avg over last minute)
	blockTimeMeter    metrics.Meter
	AvgTxThroughput   float64 `json:"avg_tx_throughput" wire:"unsafe"` // tx/s (avg over last minute)
	txThroughputMeter metrics.Meter
	AvgBlockLatency   float64 `json:"avg_block_latency" wire:"unsafe"` // ms (avg over last minute)
	blockLatencyMeter metrics.Meter

	// Network Info
	NumValidators       int `json:"num_validators"`
	NumValidatorsOnline int `json:"num_validators_online"`

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

	if n.Height >= uint64(b.Height) {
		log.Debug("Received new block with height %v less or equal to recorded %v", b.Height, n.Height)
		return
	}

	log.Debug("Received new block", "height", b.Height, "ntxs", b.NumTxs)
	n.Height = uint64(b.Height)

	n.blockTimeMeter.Mark(1)
	n.AvgBlockTime = (1.0 / n.blockTimeMeter.Rate1()) * 1000 // 1/s to ms
	n.txThroughputMeter.Mark(int64(b.NumTxs))
	n.AvgTxThroughput = n.txThroughputMeter.Rate1()

	// if we're making blocks, we're healthy
	if n.Health == Dead {
		n.Health = ModerateHealth
		n.UptimeData.totalDownTime += time.Since(n.UptimeData.wentDown)
	}

	// if we are connected to all validators, we're at full health
	// TODO: make sure they're all at the same height (within a block)
	// and all proposing (and possibly validating ) Alternatively, just
	// check there hasn't been a new round in numValidators rounds
	if n.NumValidatorsOnline == n.NumValidators {
		n.Health = FullHealth
	}
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

func (n *Network) NodeIsDown(name string) {
	n.mu.Lock()
	defer n.mu.Unlock()

	if online := n.nodeStatusMap[name]; online {
		n.nodeStatusMap[name] = false
		n.NumValidatorsOnline--
		n.UptimeData.wentDown = time.Now()
		n.updateHealth()
	}
}

func (n *Network) NodeIsOnline(name string) {
	n.mu.Lock()
	defer n.mu.Unlock()

	if online, ok := n.nodeStatusMap[name]; !ok || !online {
		n.nodeStatusMap[name] = true
		n.NumValidatorsOnline++
		n.UptimeData.totalDownTime += time.Since(n.UptimeData.wentDown)
		n.updateHealth()
	}
}

func (n *Network) updateHealth() {
	if n.NumValidatorsOnline < n.NumValidators {
		n.Health = ModerateHealth
	}

	if n.NumValidatorsOnline == 0 {
		n.Health = Dead
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
