package main

import (
	"github.com/tendermint/tendermint/p2p"
)

func initPeer(peer *p2p.Peer) {
	//
}

func main() {

	// Define channels for our app
	chDescs := []ChannelDescriptor{
		ChannelDescriptor{
			Name:           "PEX",
			SendBufferSize: 2,
			RecvBuffersize: 2,
		},
		ChannelDescriptor{
			Name:           "block",
			SendBufferSize: 10,
			RecvBufferSize: 10,
		},
		ChannelDescriptor{
			Name:           "mempool",
			SendBufferSize: 100,
			RecvBufferSize: 100,
		},
		ChannelDescriptor{
			Name:           "consensus",
			SendBufferSize: 1000,
			RecvBufferSize: 1000,
		},
	}

	// Create the switch
	sw := NewSwitch(chDescs)

	// Create a listener for incoming connections
	l := NewDefaultListener("tcp", ":8001")
	go func() {
		for {
			inConn, ok := <-l.Connections()
			if !ok {
				break
			}
			sw.AddPeerWithConnection(inConn, false)
		}
	}()

	// TODO
}
