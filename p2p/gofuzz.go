// Copyright 2017 Tendermint. All rights reserved.
// Use of this source code is governed by Apache 2 LICENSE that can be found in the LICENSE file.

package p2p

import (
	"encoding/binary"
	"net"

	wire "github.com/tendermint/go-wire"
	cmn "github.com/tendermint/tmlibs/common"
	"github.com/tendermint/tmlibs/log"

	crypto "github.com/tendermint/go-crypto"
)

// randomAddr uses random bytes provided by the fuzzer to create a random IP address and port number.
func randomAddr(b []byte) *NetAddress {
	var x [4]uint8

	for i := 0; i < 4; i++ {
		x[i] = uint8(b[i])

		if x[i] == 0 {
			x[i] = 1
		}
	}

	port := binary.LittleEndian.Uint16(b[4:])
	if port <= 1024 {
		port += 1025
	}

	a := cmn.Fmt("%v.%v.%v.%v:%v", x[0], x[1], x[2], x[3], port)
	netAddr, _ := NewNetAddressString(a)
	return netAddr
}

// randomPeer uses random bytes provided by the fuzzer to create a random peer.
// TODO: the peer can be made more complete in order to support testing of other message types.
func randomPeer(b []byte) *Peer {
	privKey := crypto.GenPrivKeyEd25519()
	config := DefaultPeerConfig()
	config.AuthEnc = false
	netAddr := randomAddr(b)

	p, err := newInboundPeer(remoteConn, make(map[byte]Reactor),
		make([]*ChannelDescriptor, 0), func(p *Peer, r interface{}) {}, privKey, config)
	if err != nil {
		return nil
	}

	p.NodeInfo = &NodeInfo{ListenAddr: netAddr.String()}
	p.mconn.RemoteAddress = netAddr
	p.SetLogger(log.TestingLogger().With("peer", netAddr.String()))
	return p
}

// the PEXReactor and AddrBook are maintained across executions of the Fuzz function.
var (
	react      *PEXReactor
	book       *AddrBook
	remoteConn net.Conn
)

func createInboundPeer(conn net.Conn, config *PeerConfig) (*Peer, error) {
	chDescs := []*ChannelDescriptor{
		&ChannelDescriptor{ID: PexChannel, Priority: 1},
	}
	reactorsByCh := map[byte]Reactor{PexChannel: react}
	peerPK := crypto.GenPrivKeyEd25519()

	p, err := newInboundPeer(conn, reactorsByCh, chDescs, func(p *Peer, r interface{}) {}, peerPK, config)
	if err != nil {
		return nil, err
	}

	n, _ := NewNetAddressString("127.0.0.1:2345")
	p.NodeInfo.ListenAddr = n.String()
	p.mconn.RemoteAddress = n
	return p, nil
}

func init() {
	book = NewAddrBook("", true)
	book.SetLogger(log.TestingLogger())

	react = NewPEXReactor(book)
	react.SetLogger(log.TestingLogger())

	remoteConn, _ = net.Pipe()
}

// Each execution of the fuzzer calls this function with random data provided.
func Fuzz(data []byte) int {
	var i int
	datalen := len(data)

	if datalen < 12 {
		// small amounts of data will not due
		return -1
	}

	// simulate random remote peer
	peer := randomPeer(data)
	i += 6

	// send a PEXRequestMessage to the reactor
	request := wire.BinaryBytes(struct{ PexMessage }{&pexRequestMessage{}})
	react.Receive(PexChannel, peer, request)

	size := book.Size()

	// random number of messages that will be generated with the data
	// that is within maximum payload size
	maxAddrs := (binary.LittleEndian.Uint16(data[i:]) + 1) % (defaultMaxMsgPacketPayloadSize / 24)
	i += 2

	var addrs []*NetAddress

	// generate the addresses
	for n := uint16(0); (i+6 <= datalen) && n < maxAddrs; i, n = i+6, n+1 {
		netAddr := randomAddr(data[i:])
		if netAddr != nil {
			addrs = append(addrs, netAddr)
		}
	}

	// encode the message and send it to the reactor from our random peer
	msg := wire.BinaryBytes(struct{ PexMessage }{&pexAddrsMessage{Addrs: addrs}})
	react.Receive(PexChannel, peer, msg)

	if book.Size() > size {
		// let the fuzzer know that this data caused the AddrBook to grow
		return 1
	}
	// not a very interesting run
	return 0
}
