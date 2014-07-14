package p2p

import (
	"bytes"
	"errors"
	"io"
	"sync/atomic"
	"time"

	. "github.com/tendermint/tendermint/binary"
	. "github.com/tendermint/tendermint/common"
)

var pexErrInvalidMessage = errors.New("Invalid PEX message")

const (
	PexCh                    = "PEX"
	ensurePeersPeriodSeconds = 30
	minNumPeers              = 10
	maxNumPeers              = 20
)

/*
PeerManager handles PEX (peer exchange) and ensures that an
adequate number of peers are connected to the switch.
User must pull from the .NewPeers() channel.
*/
type PeerManager struct {
	sw       *Switch
	book     *AddrBook
	quit     chan struct{}
	newPeers chan *Peer
	started  uint32
	stopped  uint32
}

func NewPeerManager(sw *Switch, book *AddrBook) *PeerManager {
	pm := &PeerManager{
		sw:       sw,
		book:     book,
		quit:     make(chan struct{}),
		newPeers: make(chan *Peer),
	}
	return pm
}

func (pm *PeerManager) Start() {
	if atomic.CompareAndSwapUint32(&pm.started, 0, 1) {
		log.Infof("Starting peerManager")
		go pm.ensurePeersHandler()
		go pm.pexHandler()
	}
}

func (pm *PeerManager) Stop() {
	if atomic.CompareAndSwapUint32(&pm.stopped, 0, 1) {
		log.Infof("Stopping peerManager")
		close(pm.newPeers)
		close(pm.quit)
	}
}

// Closes when PeerManager closes.
func (pm *PeerManager) NewPeers() <-chan *Peer {
	return pm.newPeers
}

func (pm *PeerManager) ensurePeersHandler() {
	// fire once immediately.
	pm.ensurePeers()
	// fire periodically
	timer := NewRepeatTimer(ensurePeersPeriodSeconds * time.Second)
FOR_LOOP:
	for {
		select {
		case <-timer.Ch:
			pm.ensurePeers()
		case <-pm.quit:
			break FOR_LOOP
		}
	}

	// cleanup
	timer.Stop()
}

// Ensures that sufficient peers are connected.
func (pm *PeerManager) ensurePeers() {
	numPeers := pm.sw.NumOutboundPeers()
	numDialing := pm.sw.dialing.Size()
	numToDial := minNumPeers - (numPeers + numDialing)
	if numToDial <= 0 {
		return
	}
	for i := 0; i < numToDial; i++ {
		newBias := MinInt(numPeers, 8)*10 + 10
		var picked *NetAddress
		// Try to fetch a new peer 3 times.
		// This caps the maximum number of tries to 3 * numToDial.
		for j := 0; i < 3; j++ {
			picked = pm.book.PickAddress(newBias)
			if picked == nil {
				log.Debug("Empty addrbook.")
				return
			}
			if pm.sw.Peers().Has(picked) {
				continue
			} else {
				break
			}
		}
		if picked == nil {
			continue
		}
		// Dial picked address
		go func() {
			peer, err := pm.sw.DialPeerWithAddress(picked)
			if err != nil {
				pm.book.MarkAttempt(picked)
			}
			// Connection established.
			pm.newPeers <- peer
		}()
	}
}

func (pm *PeerManager) pexHandler() {

	for {
		inPkt := pm.sw.Receive(PexCh) // {Peer, Time, Packet}
		if inPkt == nil {
			// Client has stopped
			break
		}

		// decode message
		msg := decodeMessage(inPkt.Bytes)
		log.Infof("pexHandler received %v", msg)

		switch msg.(type) {
		case *PexRequestMessage:
			// inPkt.Peer requested some peers.
			// TODO: prevent abuse.
			addrs := pm.book.GetSelection()
			response := &PexAddrsMessage{Addrs: addrs}
			pkt := NewPacket(PexCh, BinaryBytes(response))
			queued := inPkt.Peer.TrySend(pkt)
			if !queued {
				// ignore
			}
		case *PexAddrsMessage:
			// We received some peer addresses from inPkt.Peer.
			// TODO: prevent abuse.
			// (We don't want to get spammed with bad peers)
			srcAddr := inPkt.Peer.RemoteAddress()
			for _, addr := range msg.(*PexAddrsMessage).Addrs {
				pm.book.AddAddress(addr, srcAddr)
			}
		default:
			// Bad peer.
			pm.sw.StopPeerForError(inPkt.Peer, pexErrInvalidMessage)
		}
	}

	// cleanup

}

//-----------------------------------------------------------------------------

/* Messages */

const (
	pexTypeUnknown = Byte(0x00)
	pexTypeRequest = Byte(0x01)
	pexTypeAddrs   = Byte(0x02)
)

// TODO: check for unnecessary extra bytes at the end.
func decodeMessage(bz ByteSlice) (msg Message) {
	switch Byte(bz[0]) {
	case pexTypeRequest:
		return &PexRequestMessage{}
	case pexTypeAddrs:
		return readPexAddrsMessage(bytes.NewReader(bz[1:]))
	default:
		return nil
	}
}

/*
A PexRequestMessage requests additional peer addresses.
*/
type PexRequestMessage struct {
}

func (m *PexRequestMessage) WriteTo(w io.Writer) (n int64, err error) {
	n, err = WriteOnto(pexTypeRequest, w, n, err)
	return
}

/*
A message with announced peer addresses.
*/
type PexAddrsMessage struct {
	Addrs []*NetAddress
}

func readPexAddrsMessage(r io.Reader) *PexAddrsMessage {
	numAddrs := int(ReadUInt32(r))
	addrs := []*NetAddress{}
	for i := 0; i < numAddrs; i++ {
		addr := ReadNetAddress(r)
		addrs = append(addrs, addr)
	}
	return &PexAddrsMessage{
		Addrs: addrs,
	}
}

func (m *PexAddrsMessage) WriteTo(w io.Writer) (n int64, err error) {
	n, err = WriteOnto(pexTypeAddrs, w, n, err)
	n, err = WriteOnto(UInt32(len(m.Addrs)), w, n, err)
	for _, addr := range m.Addrs {
		n, err = WriteOnto(addr, w, n, err)
	}
	return
}
