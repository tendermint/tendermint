package main

// TODO: ensure Mark* gets called.

import (
	"os"
	"os/signal"

	. "github.com/tendermint/tendermint/binary"
	"github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/p2p"
)

type Node struct {
	lz   []p2p.Listener
	sw   *p2p.Switch
	book *p2p.AddrBook
	pmgr *p2p.PeerManager
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
	pmgr := p2p.NewPeerManager(sw, book)

	return &Node{
		sw:   sw,
		book: book,
		pmgr: pmgr,
	}
}

func (n *Node) Start() {
	log.Infof("Starting node")
	for _, l := range n.lz {
		go n.inboundConnectionHandler(l)
	}
	n.sw.Start()
	n.book.Start()
	n.pmgr.Start()
}

// Add a Listener to accept inbound peer connections.
func (n *Node) AddListener(l p2p.Listener) {
	log.Infof("Added %v", l)
	n.lz = append(n.lz, l)
}

func (n *Node) inboundConnectionHandler(l p2p.Listener) {
	for {
		inConn, ok := <-l.Connections()
		if !ok {
			break
		}
		// New inbound connection!
		peer, err := n.sw.AddPeerWithConnection(inConn, false)
		if err != nil {
			log.Infof("Ignoring error from inbound connection: %v\n%v",
				peer, err)
			continue
		}
		// NOTE: We don't yet have the external address of the
		// remote (if they have a listener at all).
		// PeerManager's pexHandler will handle that.
	}

	// cleanup
}

func (n *Node) SendOurExternalAddrs(peer *p2p.Peer) {
	// Send listener our external address(es)
	addrs := []*p2p.NetAddress{}
	for _, l := range n.lz {
		addrs = append(addrs, l.ExternalAddress())
	}
	pexAddrsMsg := &p2p.PexAddrsMessage{Addrs: addrs}
	peer.Send(p2p.NewPacket(
		p2p.PexCh,
		BinaryBytes(pexAddrsMsg),
	))
	// On the remote end, the pexHandler may choose
	// to add these to its book.
}

func (n *Node) newPeersHandler() {
	for {
		peer, ok := <-n.pmgr.NewPeers()
		if !ok {
			break
		}
		// New outbound peer!
		n.SendOurExternalAddrs(peer)
	}
}

func (n *Node) Stop() {
	log.Infof("Stopping node")
	// TODO: gracefully disconnect from peers.
	n.sw.Stop()
	n.book.Stop()
	n.pmgr.Stop()
}

//-----------------------------------------------------------------------------

func main() {

	// Create & start node
	n := NewNode()
	l := p2p.NewDefaultListener("tcp", config.Config.LAddr)
	n.AddListener(l)
	n.Start()

	// Seed?
	if config.Config.Seed != "" {
		peer, err := n.sw.DialPeerWithAddress(p2p.NewNetAddressString(config.Config.Seed))
		if err != nil {
			log.Errorf("Error dialing seed: %v", err)
			//n.book.MarkAttempt(addr)
			return
		} else {
			log.Infof("Connected to seed: %v", peer)
			n.SendOurExternalAddrs(peer)
		}
	}

	// Sleep forever and then...
	trapSignal(func() {
		n.Stop()
	})
}

func trapSignal(cb func()) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		for sig := range c {
			log.Infof("captured %v, exiting..", sig)
			cb()
			os.Exit(1)
		}
	}()
	select {}
}
