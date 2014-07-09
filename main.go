package main

import (
	"os"
	"os/signal"

	"github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/p2p"
)

func main() {

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

	// Create the switch
	sw := p2p.NewSwitch(chDescs)

	// Create a listener for incoming connections
	l := p2p.NewDefaultListener("tcp", ":8001")
	go func() {
		for {
			inConn, ok := <-l.Connections()
			if !ok {
				break
			}
			peer, err := sw.AddPeerWithConnection(inConn, false)
			if err != nil {
				log.Infof("Ignoring error from incoming connection: %v\n%v",
					peer, err)
				continue
			}
			initPeer(peer)
		}
	}()

	// Open our address book
	book := p2p.NewAddrBook(config.AppDir + "/addrbook.json")

	// Start PEX
	go p2p.PexHandler(sw, book)

	// Sleep forever
	trapSignal()
	select {}
}

func initPeer(peer *p2p.Peer) {
	// TODO: ask for more peers if we need them.
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
