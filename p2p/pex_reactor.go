package p2p

import (
	"bytes"
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	. "github.com/tendermint/tendermint/binary"
	. "github.com/tendermint/tendermint/common"
)

var pexErrInvalidMessage = errors.New("Invalid PEX message")

const (
	PexCh                    = byte(0x00)
	ensurePeersPeriodSeconds = 30
	minNumOutboundPeers      = 10
	maxNumPeers              = 50
)

/*
PEXReactor handles PEX (peer exchange) and ensures that an
adequate number of peers are connected to the switch.
*/
type PEXReactor struct {
	sw      *Switch
	quit    chan struct{}
	started uint32
	stopped uint32

	book *AddrBook
}

func NewPEXReactor(book *AddrBook) *PEXReactor {
	pexR := &PEXReactor{
		quit: make(chan struct{}),
		book: book,
	}
	return pexR
}

// Implements Reactor
func (pexR *PEXReactor) Start(sw *Switch) {
	if atomic.CompareAndSwapUint32(&pexR.started, 0, 1) {
		log.Info("Starting PEXReactor")
		pexR.sw = sw
		go pexR.ensurePeersRoutine()
	}
}

// Implements Reactor
func (pexR *PEXReactor) Stop() {
	if atomic.CompareAndSwapUint32(&pexR.stopped, 0, 1) {
		log.Info("Stopping PEXReactor")
		close(pexR.quit)
	}
}

// Implements Reactor
func (pexR *PEXReactor) GetChannels() []*ChannelDescriptor {
	return []*ChannelDescriptor{
		&ChannelDescriptor{
			Id:       PexCh,
			Priority: 1,
		},
	}
}

// Implements Reactor
func (pexR *PEXReactor) AddPeer(peer *Peer) {
	if peer.IsOutbound() {
		pexR.SendAddrs(peer, pexR.book.OurAddresses())
		if pexR.book.NeedMoreAddrs() {
			pexR.RequestPEX(peer)
		}
	}
}

// Implements Reactor
func (pexR *PEXReactor) RemovePeer(peer *Peer, reason interface{}) {
	// TODO
}

// Implements Reactor
// Handles incoming PEX messages.
func (pexR *PEXReactor) Receive(chId byte, src *Peer, msgBytes []byte) {

	// decode message
	msg, err := DecodeMessage(msgBytes)
	if err != nil {
		log.Warn("Error decoding message", "error", err)
		return
	}
	log.Info("Received message", "msg", msg)

	switch msg.(type) {
	case *pexRequestMessage:
		// src requested some peers.
		// TODO: prevent abuse.
		addrs := pexR.book.GetSelection()
		msg := &pexAddrsMessage{Addrs: addrs}
		queued := src.TrySend(PexCh, msg)
		if !queued {
			// ignore
		}
	case *pexAddrsMessage:
		// We received some peer addresses from src.
		// TODO: prevent abuse.
		// (We don't want to get spammed with bad peers)
		srcAddr := src.Connection().RemoteAddress
		for _, addr := range msg.(*pexAddrsMessage).Addrs {
			pexR.book.AddAddress(addr, srcAddr)
		}
	default:
		// Ignore unknown message.
	}

}

// Asks peer for more addresses.
func (pexR *PEXReactor) RequestPEX(peer *Peer) {
	peer.TrySend(PexCh, &pexRequestMessage{})
}

func (pexR *PEXReactor) SendAddrs(peer *Peer, addrs []*NetAddress) {
	peer.Send(PexCh, &pexAddrsMessage{Addrs: addrs})
}

// Ensures that sufficient peers are connected. (continuous)
func (pexR *PEXReactor) ensurePeersRoutine() {
	// fire once immediately.
	pexR.ensurePeers()
	// fire periodically
	timer := NewRepeatTimer(ensurePeersPeriodSeconds * time.Second)
FOR_LOOP:
	for {
		select {
		case <-timer.Ch:
			pexR.ensurePeers()
		case <-pexR.quit:
			break FOR_LOOP
		}
	}

	// Cleanup
	timer.Stop()
}

// Ensures that sufficient peers are connected. (once)
func (pexR *PEXReactor) ensurePeers() {
	numOutPeers, _, numDialing := pexR.sw.NumPeers()
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
			picked = pexR.book.PickAddress(newBias)
			if picked == nil {
				return
			}
			if toDial.Has(picked.String()) ||
				pexR.sw.IsDialing(picked) ||
				pexR.sw.Peers().Has(picked.String()) {
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
			_, err := pexR.sw.DialPeerWithAddress(picked)
			if err != nil {
				pexR.book.MarkAttempt(picked)
			}
		}()
	}
}

//-----------------------------------------------------------------------------
// Messages

const (
	msgTypeUnknown = byte(0x00)
	msgTypeRequest = byte(0x01)
	msgTypeAddrs   = byte(0x02)
)

// TODO: check for unnecessary extra bytes at the end.
func DecodeMessage(bz []byte) (msg interface{}, err error) {
	n := new(int64)
	msgType := bz[0]
	r := bytes.NewReader(bz)
	// log.Debug(Fmt("decoding msg bytes: %X", bz))
	switch msgType {
	case msgTypeRequest:
		msg = &pexRequestMessage{}
	case msgTypeAddrs:
		msg = ReadBinary(&pexAddrsMessage{}, r, n, &err)
	default:
		msg = nil
	}
	return
}

/*
A pexRequestMessage requests additional peer addresses.
*/
type pexRequestMessage struct {
}

func (m *pexRequestMessage) TypeByte() byte { return msgTypeRequest }

func (m *pexRequestMessage) String() string {
	return "[pexRequest]"
}

/*
A message with announced peer addresses.
*/
type pexAddrsMessage struct {
	Addrs []*NetAddress
}

func (m *pexAddrsMessage) TypeByte() byte { return msgTypeAddrs }

func (m *pexAddrsMessage) String() string {
	return fmt.Sprintf("[pexAddrs %v]", m.Addrs)
}
