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
	PexCh                    = byte(0x00)
	ensurePeersPeriodSeconds = 30
	minNumOutboundPeers      = 10
	maxNumPeers              = 50
)

/*
PEXAgent handles PEX (peer exchange) and ensures that an
adequate number of peers are connected to the switch.
*/
type PEXAgent struct {
	sw       *Switch
	swEvents chan interface{}
	quit     chan struct{}
	started  uint32
	stopped  uint32

	book *AddrBook
}

func NewPEXAgent(sw *Switch, book *AddrBook) *PEXAgent {
	swEvents := make(chan interface{})
	sw.AddEventListener("PEXAgent.swEvents", swEvents)
	pexA := &PEXAgent{
		sw:       sw,
		swEvents: swEvents,
		quit:     make(chan struct{}),
		book:     book,
	}
	return pexA
}

func (pexA *PEXAgent) Start() {
	if atomic.CompareAndSwapUint32(&pexA.started, 0, 1) {
		log.Info("Starting PEXAgent")
		go pexA.switchEventsRoutine()
		go pexA.requestRoutine()
		go pexA.ensurePeersRoutine()
	}
}

func (pexA *PEXAgent) Stop() {
	if atomic.CompareAndSwapUint32(&pexA.stopped, 0, 1) {
		log.Info("Stopping PEXAgent")
		close(pexA.quit)
		close(pexA.swEvents)
	}
}

// Asks peer for more addresses.
func (pexA *PEXAgent) RequestPEX(peer *Peer) {
	peer.TrySend(PexCh, &pexRequestMessage{})
}

func (pexA *PEXAgent) SendAddrs(peer *Peer, addrs []*NetAddress) {
	peer.Send(PexCh, &pexAddrsMessage{Addrs: addrs})
}

// For new outbound peers, announce our listener addresses if any,
// and if .book needs more addresses, ask for them.
func (pexA *PEXAgent) switchEventsRoutine() {
	for {
		swEvent, ok := <-pexA.swEvents
		if !ok {
			break
		}
		switch swEvent.(type) {
		case SwitchEventNewPeer:
			event := swEvent.(SwitchEventNewPeer)
			if event.Peer.IsOutbound() {
				pexA.SendAddrs(event.Peer, pexA.book.OurAddresses())
				if pexA.book.NeedMoreAddrs() {
					pexA.RequestPEX(event.Peer)
				}
			}
		case SwitchEventDonePeer:
			// TODO
		}
	}
}

// Ensures that sufficient peers are connected. (continuous)
func (pexA *PEXAgent) ensurePeersRoutine() {
	// fire once immediately.
	pexA.ensurePeers()
	// fire periodically
	timer := NewRepeatTimer(ensurePeersPeriodSeconds * time.Second)
FOR_LOOP:
	for {
		select {
		case <-timer.Ch:
			pexA.ensurePeers()
		case <-pexA.quit:
			break FOR_LOOP
		}
	}

	// Cleanup
	timer.Stop()
}

// Ensures that sufficient peers are connected. (once)
func (pexA *PEXAgent) ensurePeers() {
	numOutPeers, _, numDialing := pexA.sw.NumPeers()
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
			picked = pexA.book.PickAddress(newBias)
			if picked == nil {
				return
			}
			if toDial.Has(picked.String()) ||
				pexA.sw.IsDialing(picked) ||
				pexA.sw.Peers().Has(picked.String()) {
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
			_, err := pexA.sw.DialPeerWithAddress(picked)
			if err != nil {
				pexA.book.MarkAttempt(picked)
			}
		}()
	}
}

// Handles incoming PEX messages.
func (pexA *PEXAgent) requestRoutine() {

	for {
		inMsg, ok := pexA.sw.Receive(PexCh) // {Peer, Time, Packet}
		if !ok {
			// Client has stopped
			break
		}

		// decode message
		msg := decodeMessage(inMsg.Bytes)
		log.Info("requestRoutine received %v", msg)

		switch msg.(type) {
		case *pexRequestMessage:
			// inMsg.MConn.Peer requested some peers.
			// TODO: prevent abuse.
			addrs := pexA.book.GetSelection()
			msg := &pexAddrsMessage{Addrs: addrs}
			queued := inMsg.MConn.Peer.TrySend(PexCh, msg)
			if !queued {
				// ignore
			}
		case *pexAddrsMessage:
			// We received some peer addresses from inMsg.MConn.Peer.
			// TODO: prevent abuse.
			// (We don't want to get spammed with bad peers)
			srcAddr := inMsg.MConn.RemoteAddress
			for _, addr := range msg.(*pexAddrsMessage).Addrs {
				pexA.book.AddAddress(addr, srcAddr)
			}
		default:
			// Ignore unknown message.
			// pexA.sw.StopPeerForError(inMsg.MConn.Peer, pexErrInvalidMessage)
		}
	}

	// Cleanup

}

//-----------------------------------------------------------------------------

/* Messages */

const (
	msgTypeUnknown = byte(0x00)
	msgTypeRequest = byte(0x01)
	msgTypeAddrs   = byte(0x02)
)

// TODO: check for unnecessary extra bytes at the end.
func decodeMessage(bz []byte) (msg interface{}) {
	var n int64
	var err error
	// log.Debug("decoding msg bytes: %X", bz)
	switch bz[0] {
	case msgTypeRequest:
		return &pexRequestMessage{}
	case msgTypeAddrs:
		return readPexAddrsMessage(bytes.NewReader(bz[1:]), &n, &err)
	default:
		return nil
	}
}

/*
A pexRequestMessage requests additional peer addresses.
*/
type pexRequestMessage struct {
}

func (m *pexRequestMessage) WriteTo(w io.Writer) (n int64, err error) {
	WriteByte(w, msgTypeRequest, &n, &err)
	return
}

func (m *pexRequestMessage) String() string {
	return "[pexRequest]"
}

/*
A message with announced peer addresses.
*/
type pexAddrsMessage struct {
	Addrs []*NetAddress
}

func readPexAddrsMessage(r io.Reader, n *int64, err *error) *pexAddrsMessage {
	numAddrs := int(ReadUInt32(r, n, err))
	addrs := []*NetAddress{}
	for i := 0; i < numAddrs; i++ {
		addr := ReadNetAddress(r, n, err)
		addrs = append(addrs, addr)
	}
	return &pexAddrsMessage{
		Addrs: addrs,
	}
}

func (m *pexAddrsMessage) WriteTo(w io.Writer) (n int64, err error) {
	WriteByte(w, msgTypeAddrs, &n, &err)
	WriteUInt32(w, uint32(len(m.Addrs)), &n, &err)
	for _, addr := range m.Addrs {
		WriteBinary(w, addr, &n, &err)
	}
	return
}

func (m *pexAddrsMessage) String() string {
	return fmt.Sprintf("[pexAddrs %v]", m.Addrs)
}
