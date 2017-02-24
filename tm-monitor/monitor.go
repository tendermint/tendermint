package main

import (
	"time"

	tmtypes "github.com/tendermint/tendermint/types"
)

// waiting more than this many seconds for a block means we're unhealthy
const nodeLivenessTimeout = 5 * time.Second

type Monitor struct {
	Nodes   map[string]*Node
	Network *Network

	monitorQuit chan struct{}            // monitor exitting
	nodeQuit    map[string]chan struct{} // node is being stopped and removed from under the monitor
}

func NewMonitor() *Monitor {
	return &Monitor{
		Nodes:       make(map[string]*Node),
		Network:     NewNetwork(),
		monitorQuit: make(chan struct{}),
		nodeQuit:    make(map[string]chan struct{}),
	}
}

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

	m.nodeQuit[n.Name] = make(chan struct{})
	go m.listen(n.Name, blockCh, blockLatencyCh, disconnectCh, m.nodeQuit[n.Name])
	return nil
}

func (m *Monitor) Unmonitor(n *Node) {
	n.Stop()
	close(m.nodeQuit[n.Name])
	delete(m.nodeQuit, n.Name)
	delete(m.Nodes, n.Name)
}

func (m *Monitor) Start() error {
	go m.recalculateNetworkUptime()

	return nil
}

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
		case l := <-blockLatencyCh:
			m.Network.NewBlockLatency(l)
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

// recalculateNetworkUptime every N seconds.
func (m *Monitor) recalculateNetworkUptime() {
	for {
		select {
		case <-m.monitorQuit:
			return
		case <-time.After(10 * time.Second):
			m.Network.RecalculateUptime()
		}
	}
}
