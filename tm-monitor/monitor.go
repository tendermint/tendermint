package main

import (
	"math/rand"
	"time"

	tmtypes "github.com/tendermint/tendermint/types"
)

// waiting more than this many seconds for a block means we're unhealthy
const nodeLivenessTimeout = 5 * time.Second

// Monitor keeps track of the nodes and updates common statistics upon
// receiving new events from nodes.
//
// Common statistics is stored in Network struct.
type Monitor struct {
	Nodes   map[string]*Node
	Network *Network

	monitorQuit chan struct{}            // monitor exitting
	nodeQuit    map[string]chan struct{} // node is being stopped and removed from under the monitor

	recalculateNetworkUptimeEvery time.Duration
	numValidatorsUpdateInterval   time.Duration
}

// NewMonitor creates new instance of a Monitor. You can provide options to
// change some default values.
//
// Example:
//   NewMonitor(monitor.SetNumValidatorsUpdateInterval(1 * time.Second))
func NewMonitor(options ...func(*Monitor)) *Monitor {
	m := &Monitor{
		Nodes:                         make(map[string]*Node),
		Network:                       NewNetwork(),
		monitorQuit:                   make(chan struct{}),
		nodeQuit:                      make(map[string]chan struct{}),
		recalculateNetworkUptimeEvery: 10 * time.Second,
		numValidatorsUpdateInterval:   5 * time.Second,
	}

	for _, option := range options {
		option(m)
	}

	return m
}

// RecalculateNetworkUptimeEvery lets you change network uptime update interval.
func RecalculateNetworkUptimeEvery(d time.Duration) func(m *Monitor) {
	return func(m *Monitor) {
		m.recalculateNetworkUptimeEvery = d
	}
}

// SetNumValidatorsUpdateInterval lets you change num validators update interval.
func SetNumValidatorsUpdateInterval(d time.Duration) func(m *Monitor) {
	return func(m *Monitor) {
		m.numValidatorsUpdateInterval = d
	}
}

// Monitor begins to monitor the node `n`. The node will be started and added
// to the monitor.
func (m *Monitor) Monitor(n *Node) error {
	m.Nodes[n.Name] = n

	blockCh := make(chan tmtypes.Header, 10)
	n.SendBlocksTo(blockCh)
	blockLatencyCh := make(chan float64, 10)
	n.SendBlockLatenciesTo(blockLatencyCh)
	disconnectCh := make(chan bool, 10)
	n.NotifyAboutDisconnects(disconnectCh)

	if err := n.Start(); err != nil {
		return err
	}

	m.Network.NewNode(n.Name)

	m.nodeQuit[n.Name] = make(chan struct{})
	go m.listen(n.Name, blockCh, blockLatencyCh, disconnectCh, m.nodeQuit[n.Name])

	return nil
}

// Unmonitor stops monitoring node `n`. The node will be stopped and removed
// from the monitor.
func (m *Monitor) Unmonitor(n *Node) {
	m.Network.NodeDeleted(n.Name)

	n.Stop()
	close(m.nodeQuit[n.Name])
	delete(m.nodeQuit, n.Name)
	delete(m.Nodes, n.Name)
}

// Start starts the monitor's routines: recalculating network uptime and
// updating number of validators.
func (m *Monitor) Start() error {
	go m.recalculateNetworkUptimeLoop()
	go m.updateNumValidatorLoop()

	return nil
}

// Stop stops the monitor's routines.
func (m *Monitor) Stop() {
	close(m.monitorQuit)

	for _, n := range m.Nodes {
		m.Unmonitor(n)
	}
}

// main loop where we listen for events from the node
func (m *Monitor) listen(nodeName string, blockCh <-chan tmtypes.Header, blockLatencyCh <-chan float64, disconnectCh <-chan bool, quit <-chan struct{}) {
	for {
		select {
		case <-quit:
			return
		case b := <-blockCh:
			m.Network.NewBlock(b)
			m.Network.NodeIsOnline(nodeName)
		case l := <-blockLatencyCh:
			m.Network.NewBlockLatency(l)
			m.Network.NodeIsOnline(nodeName)
		case disconnected := <-disconnectCh:
			if disconnected {
				m.Network.NodeIsDown(nodeName)
			} else {
				m.Network.NodeIsOnline(nodeName)
			}
		case <-time.After(nodeLivenessTimeout):
			m.Network.NodeIsDown(nodeName)
		}
	}
}

// recalculateNetworkUptimeLoop every N seconds.
func (m *Monitor) recalculateNetworkUptimeLoop() {
	for {
		select {
		case <-m.monitorQuit:
			return
		case <-time.After(m.recalculateNetworkUptimeEvery):
			m.Network.RecalculateUptime()
		}
	}
}

// updateNumValidatorLoop sends a request to a random node once every N seconds,
// which in turn makes an RPC call to get the latest validators.
func (m *Monitor) updateNumValidatorLoop() {
	rand.Seed(time.Now().Unix())

	var height uint64
	var num int
	var err error

	for {
		if 0 == len(m.Nodes) {
			time.Sleep(m.numValidatorsUpdateInterval)
			continue
		}

		randomNodeIndex := rand.Intn(len(m.Nodes))

		select {
		case <-m.monitorQuit:
			return
		case <-time.After(m.numValidatorsUpdateInterval):
			i := 0
			for _, n := range m.Nodes {
				if i == randomNodeIndex {
					height, num, err = n.NumValidators()
					if err != nil {
						log.Debug(err.Error())
					}
					break
				}
				i++
			}

			if m.Network.Height <= height {
				m.Network.NumValidators = num
			}
		}
	}
}
