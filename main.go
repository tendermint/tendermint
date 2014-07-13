package main

import (
	"os"
	"os/signal"
	"time"

	. "github.com/tendermint/tendermint/common"
	"github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/p2p"
)

const (
	minNumPeers = 10
	maxNumPeers = 20

	ensurePeersPeriodSeconds = 30
	peerDialTimeoutSeconds   = 30
)

type Node struct {
	sw      *p2p.Switch
	book    *p2p.AddrBook
	quit    chan struct{}
	dialing *CMap
}

func NewNode() *Node {
	// Define channels for our app
	chDescs := []p2p.ChannelDescriptor{
		p2p.ChannelDescriptor{
			Name:           "PEX",
			SendBufferSize: 2,
			RecvBufferSize: 2,
		},
		p2p.ChannelDescriptor{
			Name:           "block",
			SendBufferSize: 10,
			RecvBufferSize: 10,
		},
		p2p.ChannelDescriptor{
			Name:           "mempool",
			SendBufferSize: 100,
			RecvBufferSize: 100,
		},
		p2p.ChannelDescriptor{
			Name:           "consensus",
			SendBufferSize: 1000,
			RecvBufferSize: 1000,
		},
	}
	sw := p2p.NewSwitch(chDescs)
	book := p2p.NewAddrBook(config.AppDir + "/addrbook.json")

	return &Node{
		sw:      sw,
		book:    book,
		quit:    make(chan struct{}, 0),
		dialing: NewCMap(),
	}
}

func (n *Node) Start() {
	log.Infof("Starting node")
	n.sw.Start()
	n.book.Start()
	go p2p.PexHandler(n.sw, n.book)
	go n.ensurePeersHandler()
}

func (n *Node) initPeer(peer *p2p.Peer) {
	if peer.IsOutbound() {
		// TODO: initiate PEX
	}
}

// Add a Listener to accept incoming peer connections.
func (n *Node) AddListener(l p2p.Listener) {
	log.Infof("Adding listener %v", l)
	go func() {
		for {
			inConn, ok := <-l.Connections()
			if !ok {
				break
			}
			peer, err := n.sw.AddPeerWithConnection(inConn, false)
			if err != nil {
				log.Infof("Ignoring error from incoming connection: %v\n%v",
					peer, err)
				continue
			}
			n.initPeer(peer)
		}
	}()
}

// threadsafe
func (n *Node) DialPeerWithAddress(addr *p2p.NetAddress) (*p2p.Peer, error) {
	log.Infof("Dialing peer @ %v", addr)
	n.dialing.Set(addr.String(), addr)
	n.book.MarkAttempt(addr)
	conn, err := addr.DialTimeout(peerDialTimeoutSeconds * time.Second)
	n.dialing.Delete(addr.String())
	if err != nil {
		return nil, err
	}
	peer, err := n.sw.AddPeerWithConnection(conn, true)
	if err != nil {
		return nil, err
	}
	n.initPeer(peer)
	return peer, nil
}

// Ensures that sufficient peers are connected.
func (n *Node) ensurePeers() {
	numPeers := n.sw.NumOutboundPeers()
	numDialing := n.dialing.Size()
	numToDial := minNumPeers - (numPeers + numDialing)
	if numToDial <= 0 {
		return
	}
	for i := 0; i < numToDial; i++ {
		newBias := MinInt(numPeers, 8)*10 + 10
		var picked *p2p.NetAddress
		// Try to fetch a new peer 3 times.
		// This caps the maximum number of tries to 3 * numToDial.
		for j := 0; i < 3; j++ {
			picked = n.book.PickAddress(newBias)
			if picked == nil {
				log.Debug("Empty addrbook.")
				return
			}
			if n.sw.Peers().Has(picked) {
				continue
			} else {
				break
			}
		}
		if picked == nil {
			continue
		}
		go n.DialPeerWithAddress(picked)
	}
}

func (n *Node) ensurePeersHandler() {
	// fire once immediately.
	n.ensurePeers()
	// fire periodically
	timer := NewRepeatTimer(ensurePeersPeriodSeconds * time.Second)
FOR_LOOP:
	for {
		select {
		case <-timer.Ch:
			n.ensurePeers()
		case <-n.quit:
			break FOR_LOOP
		}
	}

	// cleanup
	timer.Stop()
}

func (n *Node) Stop() {
	// TODO: gracefully disconnect from peers.
	n.sw.Stop()
	n.book.Stop()
}

//-----------------------------------------------------------------------------

func main() {

	// Create & start node
	n := NewNode()
	log.Warnf(">> %v", config.Config.LAddr)
	l := p2p.NewDefaultListener("tcp", config.Config.LAddr)
	n.AddListener(l)
	n.Start()

	// Seed?
	if config.Config.Seed != "" {
		peer, err := n.DialPeerWithAddress(p2p.NewNetAddressString(config.Config.Seed))
		if err != nil {
			log.Errorf("Error dialing seed: %v", err)
			return
		}
		log.Infof("Connected to seed: %v", peer)
	}

	// Sleep
	trapSignal()
	select {}
}

func trapSignal() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		for sig := range c {
			log.Infof("captured %v, exiting..", sig)
			os.Exit(1)
		}
	}()
}
