package main

import (
	"math/rand"
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

	m.Network.NumValidatorsOnline++

	m.nodeQuit[n.Name] = make(chan struct{})
	go m.listen(n.Name, blockCh, blockLatencyCh, disconnectCh, m.nodeQuit[n.Name])

	return nil
}

func (m *Monitor) Unmonitor(n *Node) {
	m.Network.NumValidatorsOnline--

	n.Stop()
	close(m.nodeQuit[n.Name])
	delete(m.nodeQuit, n.Name)
	delete(m.Nodes, n.Name)
}

func (m *Monitor) Start() error {
	go m.recalculateNetworkUptime()
	go m.updateNumValidators()

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

// updateNumValidators sends a request to a random node once every N seconds,
// which in turn makes an RPC call to get the latest validators.
func (m *Monitor) updateNumValidators() {
	rand.Seed(time.Now().Unix())

	var height uint64
	var num int
	var err error

	for {
		if 0 == len(m.Nodes) {
			m.Network.NumValidators = 0
			time.Sleep(5 * time.Second)
			continue
		}

		randomNodeIndex := rand.Intn(len(m.Nodes))

		select {
		case <-m.monitorQuit:
			return
		case <-time.After(5 * time.Second):
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
