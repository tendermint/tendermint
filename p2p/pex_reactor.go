package p2p

import (
	"bytes"
	"errors"
	"fmt"
	"math/rand"
	"reflect"
	"sync/atomic"
	"time"

	"github.com/tendermint/tendermint/binary"
	. "github.com/tendermint/tendermint/common"
	"github.com/tendermint/tendermint/events"
)

var pexErrInvalidMessage = errors.New("Invalid PEX message")

const (
	PexChannel               = byte(0x00)
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

	evsw events.Fireable
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
			Id:                PexChannel,
			Priority:          1,
			SendQueueCapacity: 10,
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
	_, msg, err := DecodeMessage(msgBytes)
	if err != nil {
		log.Warn("Error decoding message", "error", err)
		return
	}
	log.Info("Received message", "msg", msg)

	switch msg.(type) {
	case *pexRequestMessage:
		// src requested some peers.
		// TODO: prevent abuse.
		pexR.SendAddrs(src, pexR.book.GetSelection())
	case *pexAddrsMessage:
		// We received some peer addresses from src.
		// TODO: prevent abuse.
		// (We don't want to get spammed with bad peers)
		srcAddr := src.Connection().RemoteAddress
		for _, addr := range msg.(*pexAddrsMessage).Addrs {
			pexR.book.AddAddress(addr, srcAddr)
		}
	default:
		log.Warn(Fmt("Unknown message type %v", reflect.TypeOf(msg)))
	}

}

// Asks peer for more addresses.
func (pexR *PEXReactor) RequestPEX(peer *Peer) {
	peer.Send(PexChannel, &pexRequestMessage{})
}

func (pexR *PEXReactor) SendAddrs(peer *Peer, addrs []*NetAddress) {
	peer.Send(PexChannel, &pexAddrsMessage{Addrs: addrs})
}

// Ensures that sufficient peers are connected. (continuous)
func (pexR *PEXReactor) ensurePeersRoutine() {
	// Randomize when routine starts
	time.Sleep(time.Duration(rand.Int63n(500*ensurePeersPeriodSeconds)) * time.Millisecond)

	// fire once immediately.
	pexR.ensurePeers()
	// fire periodically
	timer := NewRepeatTimer("pex", ensurePeersPeriodSeconds*time.Second)
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
	log.Debug("Ensure peers", "numOutPeers", numOutPeers, "numDialing", numDialing, "numToDial", numToDial)
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
		for j := 0; j < 3; j++ {
			try := pexR.book.PickAddress(newBias)
			if try == nil {
				break
			}
			alreadySelected := toDial.Has(try.IP.String())
			alreadyDialing := pexR.sw.IsDialing(try)
			alreadyConnected := pexR.sw.Peers().Has(try.IP.String())
			if alreadySelected || alreadyDialing || alreadyConnected {
				/*
					log.Debug("Cannot dial address", "addr", try,
						"alreadySelected", alreadySelected,
						"alreadyDialing", alreadyDialing,
						"alreadyConnected", alreadyConnected)
				*/
				continue
			} else {
				log.Debug("Will dial address", "addr", try)
				picked = try
				break
			}
		}
		if picked == nil {
			continue
		}
		toDial.Set(picked.IP.String(), picked)
	}

	// Dial picked addresses
	for _, item := range toDial.Values() {
		go func(picked *NetAddress) {
			_, err := pexR.sw.DialPeerWithAddress(picked)
			if err != nil {
				pexR.book.MarkAttempt(picked)
			}
		}(item.(*NetAddress))
	}
}

// implements events.Eventable
func (pexR *PEXReactor) SetFireable(evsw events.Fireable) {
	pexR.evsw = evsw
}

//-----------------------------------------------------------------------------
// Messages

const (
	msgTypeRequest = byte(0x01)
	msgTypeAddrs   = byte(0x02)
)

type PexMessage interface{}

var _ = binary.RegisterInterface(
	struct{ PexMessage }{},
	binary.ConcreteType{&pexRequestMessage{}, msgTypeRequest},
	binary.ConcreteType{&pexAddrsMessage{}, msgTypeAddrs},
)

func DecodeMessage(bz []byte) (msgType byte, msg PexMessage, err error) {
	msgType = bz[0]
	n := new(int64)
	r := bytes.NewReader(bz)
	msg = binary.ReadBinary(struct{ PexMessage }{}, r, n, &err).(struct{ PexMessage }).PexMessage
	return
}

/*
A pexRequestMessage requests additional peer addresses.
*/
type pexRequestMessage struct {
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

func (m *pexAddrsMessage) String() string {
	return fmt.Sprintf("[pexAddrs %v]", m.Addrs)
}
