package p2p

import (
	"bytes"
	"errors"
	"fmt"
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
	minNumOutboundPeers      = 10
	maxNumPeers              = 50
)

/*
PeerManager handles PEX (peer exchange) and ensures that an
adequate number of peers are connected to the switch.
*/
type PeerManager struct {
	sw      *Switch
	book    *AddrBook
	quit    chan struct{}
	started uint32
	stopped uint32
}

func NewPeerManager(sw *Switch, book *AddrBook) *PeerManager {
	pm := &PeerManager{
		sw:   sw,
		book: book,
		quit: make(chan struct{}),
	}
	return pm
}

func (pm *PeerManager) Start() {
	if atomic.CompareAndSwapUint32(&pm.started, 0, 1) {
		log.Info("Starting peerManager")
		go pm.ensurePeersHandler()
		go pm.pexHandler()
	}
}

func (pm *PeerManager) Stop() {
	if atomic.CompareAndSwapUint32(&pm.stopped, 0, 1) {
		log.Info("Stopping peerManager")
		close(pm.quit)
	}
}

// Ensures that sufficient peers are connected. (continuous)
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

	// Cleanup
	timer.Stop()
}

// Ensures that sufficient peers are connected. (once)
func (pm *PeerManager) ensurePeers() {
	numOutPeers, _, numDialing := pm.sw.NumPeers()
	numToDial := minNumOutboundPeers - (numOutPeers + numDialing)
	if numToDial <= 0 {
		return
	}
	toDial := NewCMap()

	// Try to pick numToDial addresses to dial.
	// TODO: improve logic.
	for i := 0; i < numToDial; i++ {
		newBias := MinInt(numOutPeers, 8)*10 + 10
		var picked *NetAddress
		// Try to fetch a new peer 3 times.
		// This caps the maximum number of tries to 3 * numToDial.
		for j := 0; i < 3; j++ {
			picked = pm.book.PickAddress(newBias)
			if picked == nil {
				return
			}
			if toDial.Has(picked.String()) ||
				pm.sw.IsDialing(picked) ||
				pm.sw.Peers().Has(picked) {
				continue
			} else {
				break
			}
		}
		if picked == nil {
			continue
		}
		toDial.Set(picked.String(), picked)
	}

	// Dial picked addresses
	for _, item := range toDial.Values() {
		picked := item.(*NetAddress)
		go func() {
			_, err := pm.sw.DialPeerWithAddress(picked)
			if err != nil {
				pm.book.MarkAttempt(picked)
			}
		}()
	}
}

// Handles incoming Pex messages.
func (pm *PeerManager) pexHandler() {

	for {
		inPkt := pm.sw.Receive(PexCh) // {Peer, Time, Packet}
		if inPkt == nil {
			// Client has stopped
			break
		}

		// decode message
		msg := decodeMessage(inPkt.Bytes)
		log.Info("pexHandler received %v", msg)

		switch msg.(type) {
		case *PexRequestMessage:
			// inPkt.Peer requested some peers.
			// TODO: prevent abuse.
			addrs := pm.book.GetSelection()
			response := &PexAddrsMessage{Addrs: addrs}
			pkt := NewPacket(PexCh, response)
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

	// Cleanup

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
	// log.Debug("decoding msg bytes: %X", bz)
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

// TODO: define NewPexRequestPacket instead?

func NewPexRequestMessage() *PexRequestMessage {
	return &PexRequestMessage{}
}

func (m *PexRequestMessage) WriteTo(w io.Writer) (n int64, err error) {
	n, err = WriteOnto(pexTypeRequest, w, n, err)
	return
}

func (m *PexRequestMessage) String() string {
	return "[PexRequest]"
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

func (m *PexAddrsMessage) String() string {
	return fmt.Sprintf("[PexAddrs %v]", m.Addrs)
}
