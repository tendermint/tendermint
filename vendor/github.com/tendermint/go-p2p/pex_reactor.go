package p2p

import (
	"bytes"
	"errors"
	"fmt"
	"math/rand"
	"reflect"
	"time"

	. "github.com/tendermint/go-common"
	"github.com/tendermint/go-wire"
)

var pexErrInvalidMessage = errors.New("Invalid PEX message")

const (
	PexChannel               = byte(0x00)
	ensurePeersPeriodSeconds = 30
	minNumOutboundPeers      = 10
	maxPexMessageSize        = 1048576 // 1MB
)

/*
PEXReactor handles PEX (peer exchange) and ensures that an
adequate number of peers are connected to the switch.
*/
type PEXReactor struct {
	BaseReactor

	sw   *Switch
	book *AddrBook
}

func NewPEXReactor(book *AddrBook) *PEXReactor {
	pexR := &PEXReactor{
		book: book,
	}
	pexR.BaseReactor = *NewBaseReactor(log, "PEXReactor", pexR)
	return pexR
}

func (pexR *PEXReactor) OnStart() error {
	pexR.BaseReactor.OnStart()
	go pexR.ensurePeersRoutine()
	return nil
}

func (pexR *PEXReactor) OnStop() {
	pexR.BaseReactor.OnStop()
}

// Implements Reactor
func (pexR *PEXReactor) GetChannels() []*ChannelDescriptor {
	return []*ChannelDescriptor{
		&ChannelDescriptor{
			ID:                PexChannel,
			Priority:          1,
			SendQueueCapacity: 10,
		},
	}
}

// Implements Reactor
func (pexR *PEXReactor) AddPeer(peer *Peer) {
	// Add the peer to the address book
	netAddr := NewNetAddressString(peer.ListenAddr)
	if peer.IsOutbound() {
		if pexR.book.NeedMoreAddrs() {
			pexR.RequestPEX(peer)
		}
	} else {
		// For inbound connections, the peer is its own source
		// (For outbound peers, the address is already in the books)
		pexR.book.AddAddress(netAddr, netAddr)
	}
}

// Implements Reactor
func (pexR *PEXReactor) RemovePeer(peer *Peer, reason interface{}) {
	// TODO
}

// Implements Reactor
// Handles incoming PEX messages.
func (pexR *PEXReactor) Receive(chID byte, src *Peer, msgBytes []byte) {

	// decode message
	_, msg, err := DecodeMessage(msgBytes)
	if err != nil {
		log.Warn("Error decoding message", "error", err)
		return
	}
	log.Notice("Received message", "msg", msg)

	switch msg := msg.(type) {
	case *pexRequestMessage:
		// src requested some peers.
		// TODO: prevent abuse.
		pexR.SendAddrs(src, pexR.book.GetSelection())
	case *pexAddrsMessage:
		// We received some peer addresses from src.
		// TODO: prevent abuse.
		// (We don't want to get spammed with bad peers)
		srcAddr := src.Connection().RemoteAddress
		for _, addr := range msg.Addrs {
			pexR.book.AddAddress(addr, srcAddr)
		}
	default:
		log.Warn(Fmt("Unknown message type %v", reflect.TypeOf(msg)))
	}

}

// Asks peer for more addresses.
func (pexR *PEXReactor) RequestPEX(peer *Peer) {
	peer.Send(PexChannel, struct{ PexMessage }{&pexRequestMessage{}})
}

func (pexR *PEXReactor) SendAddrs(peer *Peer, addrs []*NetAddress) {
	peer.Send(PexChannel, struct{ PexMessage }{&pexAddrsMessage{Addrs: addrs}})
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
		case <-pexR.Quit:
			break FOR_LOOP
		}
	}

	// Cleanup
	timer.Stop()
}

// Ensures that sufficient peers are connected. (once)
func (pexR *PEXReactor) ensurePeers() {
	numOutPeers, _, numDialing := pexR.Switch.NumPeers()
	numToDial := minNumOutboundPeers - (numOutPeers + numDialing)
	log.Info("Ensure peers", "numOutPeers", numOutPeers, "numDialing", numDialing, "numToDial", numToDial)
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
			alreadyDialing := pexR.Switch.IsDialing(try)
			alreadyConnected := pexR.Switch.Peers().Has(try.IP.String())
			if alreadySelected || alreadyDialing || alreadyConnected {
				/*
					log.Info("Cannot dial address", "addr", try,
						"alreadySelected", alreadySelected,
						"alreadyDialing", alreadyDialing,
						"alreadyConnected", alreadyConnected)
				*/
				continue
			} else {
				log.Info("Will dial address", "addr", try)
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
			_, err := pexR.Switch.DialPeerWithAddress(picked)
			if err != nil {
				pexR.book.MarkAttempt(picked)
			}
		}(item.(*NetAddress))
	}

	// If we need more addresses, pick a random peer and ask for more.
	if pexR.book.NeedMoreAddrs() {
		if peers := pexR.Switch.Peers().List(); len(peers) > 0 {
			i := rand.Int() % len(peers)
			peer := peers[i]
			log.Info("No addresses to dial. Sending pexRequest to random peer", "peer", peer)
			pexR.RequestPEX(peer)
		}
	}
}

//-----------------------------------------------------------------------------
// Messages

const (
	msgTypeRequest = byte(0x01)
	msgTypeAddrs   = byte(0x02)
)

type PexMessage interface{}

var _ = wire.RegisterInterface(
	struct{ PexMessage }{},
	wire.ConcreteType{&pexRequestMessage{}, msgTypeRequest},
	wire.ConcreteType{&pexAddrsMessage{}, msgTypeAddrs},
)

func DecodeMessage(bz []byte) (msgType byte, msg PexMessage, err error) {
	msgType = bz[0]
	n := new(int)
	r := bytes.NewReader(bz)
	msg = wire.ReadBinary(struct{ PexMessage }{}, r, maxPexMessageSize, n, &err).(struct{ PexMessage }).PexMessage
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
