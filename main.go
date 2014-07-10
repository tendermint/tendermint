package main

import (
	"os"
	"os/signal"

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
	book    *p2p.AddressBook
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

	return &New{
		sw:      sw,
		book:    book,
		quit:    make(chan struct{}, 0),
		dialing: NewCMap(),
	}
}

func (n *Node) Start() {
	n.sw.Start()
	n.book.Start()
	go p2p.PexHandler(sw, book)
	go n.ensurePeersHandler(sw, book)
}

func (n *Node) initPeer(peer *Peer) {
	if peer.IsOutgoing() {
		// TODO: initiate PEX
	}
}

// Add a Listener to accept incoming peer connections.
func (n *Node) AddListener(l Listener) {
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

// Ensures that sufficient peers are connected.
func (n *Node) ensurePeers() {
	numPeers := len(n.sw.Peers())
	numDialing := n.dialing.Size()
	numToDial = minNumPeers - (numPeers + numDialing)
	if numToDial <= 0 {
		return
	}
	for i := 0; i < numToDial; i++ {
		// XXX
	}
}

func (n *Node) ensurePeersHandler() {
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

	n := NewNode()
	l := p2p.NewDefaultListener("tcp", ":8001")
	n.AddListener()

	if false {
		// TODO remove
		// let's connect to 66.175.218.199
		conn, err := p2p.NewNetAddressString("66.175.218.199:8001").Dial()
		if err != nil {
			log.Infof("Error connecting to it: %v", err)
			return
		}
		peer, err := sw.AddPeerWithConnection(conn, true)
		if err != nil {
			log.Infof("Error adding peer with connection: %v", err)
			return
		}
		log.Infof("Connected to peer: %v", peer)
		// TODO remove
	}

	// Sleep forever
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
