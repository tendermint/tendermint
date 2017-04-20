package p2p

import (
	"bytes"
	"fmt"
	"math/rand"
	"reflect"
	"time"

	cmn "github.com/tendermint/go-common"
	wire "github.com/tendermint/go-wire"
)

const (
	// PexChannel is a channel for PEX messages
	PexChannel = byte(0x00)

	// period to ensure peers connected
	defaultEnsurePeersPeriod = 30 * time.Second
	minNumOutboundPeers      = 10
	maxPexMessageSize        = 1048576 // 1MB

	// maximum messages one peer can send to us during `msgCountByPeerFlushInterval`
	defaultMaxMsgCountByPeer    = 1000
	msgCountByPeerFlushInterval = 1 * time.Hour
)

// PEXReactor handles PEX (peer exchange) and ensures that an
// adequate number of peers are connected to the switch.
//
// It uses `AddrBook` (address book) to store `NetAddress`es of the peers.
//
// ## Preventing abuse
//
// For now, it just limits the number of messages from one peer to
// `defaultMaxMsgCountByPeer` messages per `msgCountByPeerFlushInterval` (1000
// msg/hour).
//
// NOTE [2017-01-17]:
//   Limiting is fine for now. Maybe down the road we want to keep track of the
//   quality of peer messages so if peerA keeps telling us about peers we can't
//   connect to then maybe we should care less about peerA. But I don't think
//   that kind of complexity is priority right now.
type PEXReactor struct {
	BaseReactor

	sw                *Switch
	book              *AddrBook
	ensurePeersPeriod time.Duration

	// tracks message count by peer, so we can prevent abuse
	msgCountByPeer    *cmn.CMap
	maxMsgCountByPeer uint16
}

// NewPEXReactor creates new PEX reactor.
func NewPEXReactor(b *AddrBook) *PEXReactor {
	r := &PEXReactor{
		book:              b,
		ensurePeersPeriod: defaultEnsurePeersPeriod,
		msgCountByPeer:    cmn.NewCMap(),
		maxMsgCountByPeer: defaultMaxMsgCountByPeer,
	}
	r.BaseReactor = *NewBaseReactor(log, "PEXReactor", r)
	return r
}

// OnStart implements BaseService
func (r *PEXReactor) OnStart() error {
	r.BaseReactor.OnStart()
	r.book.Start()
	go r.ensurePeersRoutine()
	go r.flushMsgCountByPeer()
	return nil
}

// OnStop implements BaseService
func (r *PEXReactor) OnStop() {
	r.BaseReactor.OnStop()
	r.book.Stop()
}

// GetChannels implements Reactor
func (r *PEXReactor) GetChannels() []*ChannelDescriptor {
	return []*ChannelDescriptor{
		&ChannelDescriptor{
			ID:                PexChannel,
			Priority:          1,
			SendQueueCapacity: 10,
		},
	}
}

// AddPeer implements Reactor by adding peer to the address book (if inbound)
// or by requesting more addresses (if outbound).
func (r *PEXReactor) AddPeer(p *Peer) {
	if p.IsOutbound() {
		// For outbound peers, the address is already in the books.
		// Either it was added in DialSeeds or when we
		// received the peer's address in r.Receive
		if r.book.NeedMoreAddrs() {
			r.RequestPEX(p)
		}
	} else { // For inbound connections, the peer is its own source
		addr, err := NewNetAddressString(p.ListenAddr)
		if err != nil {
			// this should never happen
			log.Error("Error in AddPeer: invalid peer address", "addr", p.ListenAddr, "error", err)
			return
		}
		r.book.AddAddress(addr, addr)
	}
}

// RemovePeer implements Reactor.
func (r *PEXReactor) RemovePeer(p *Peer, reason interface{}) {
	// If we aren't keeping track of local temp data for each peer here, then we
	// don't have to do anything.
}

// Receive implements Reactor by handling incoming PEX messages.
func (r *PEXReactor) Receive(chID byte, src *Peer, msgBytes []byte) {
	srcAddr := src.Connection().RemoteAddress
	srcAddrStr := srcAddr.String()

	r.IncrementMsgCountForPeer(srcAddrStr)
	if r.ReachedMaxMsgCountForPeer(srcAddrStr) {
		log.Warn("Maximum number of messages reached for peer", "peer", srcAddrStr)
		// TODO remove src from peers?
		return
	}

	_, msg, err := DecodeMessage(msgBytes)
	if err != nil {
		log.Warn("Error decoding message", "error", err)
		return
	}
	log.Notice("Received message", "msg", msg)

	switch msg := msg.(type) {
	case *pexRequestMessage:
		// src requested some peers.
		r.SendAddrs(src, r.book.GetSelection())
	case *pexAddrsMessage:
		// We received some peer addresses from src.
		// (We don't want to get spammed with bad peers)
		for _, addr := range msg.Addrs {
			if addr != nil {
				r.book.AddAddress(addr, srcAddr)
			}
		}
	default:
		log.Warn(fmt.Sprintf("Unknown message type %v", reflect.TypeOf(msg)))
	}
}

// RequestPEX asks peer for more addresses.
func (r *PEXReactor) RequestPEX(p *Peer) {
	p.Send(PexChannel, struct{ PexMessage }{&pexRequestMessage{}})
}

// SendAddrs sends addrs to the peer.
func (r *PEXReactor) SendAddrs(p *Peer, addrs []*NetAddress) {
	p.Send(PexChannel, struct{ PexMessage }{&pexAddrsMessage{Addrs: addrs}})
}

// SetEnsurePeersPeriod sets period to ensure peers connected.
func (r *PEXReactor) SetEnsurePeersPeriod(d time.Duration) {
	r.ensurePeersPeriod = d
}

// SetMaxMsgCountByPeer sets maximum messages one peer can send to us during 'msgCountByPeerFlushInterval'.
func (r *PEXReactor) SetMaxMsgCountByPeer(v uint16) {
	r.maxMsgCountByPeer = v
}

// ReachedMaxMsgCountForPeer returns true if we received too many
// messages from peer with address `addr`.
// NOTE: assumes the value in the CMap is non-nil
func (r *PEXReactor) ReachedMaxMsgCountForPeer(addr string) bool {
	return r.msgCountByPeer.Get(addr).(uint16) >= r.maxMsgCountByPeer
}

// Increment or initialize the msg count for the peer in the CMap
func (r *PEXReactor) IncrementMsgCountForPeer(addr string) {
	var count uint16
	countI := r.msgCountByPeer.Get(addr)
	if countI != nil {
		count = countI.(uint16)
	}
	count++
	r.msgCountByPeer.Set(addr, count)
}

// Ensures that sufficient peers are connected. (continuous)
func (r *PEXReactor) ensurePeersRoutine() {
	// Randomize when routine starts
	ensurePeersPeriodMs := r.ensurePeersPeriod.Nanoseconds() / 1e6
	time.Sleep(time.Duration(rand.Int63n(ensurePeersPeriodMs)) * time.Millisecond)

	// fire once immediately.
	r.ensurePeers()

	// fire periodically
	ticker := time.NewTicker(r.ensurePeersPeriod)

	for {
		select {
		case <-ticker.C:
			r.ensurePeers()
		case <-r.Quit:
			ticker.Stop()
			return
		}
	}
}

// ensurePeers ensures that sufficient peers are connected. (once)
//
// Old bucket / New bucket are arbitrary categories to denote whether an
// address is vetted or not, and this needs to be determined over time via a
// heuristic that we haven't perfected yet, or, perhaps is manually edited by
// the node operator. It should not be used to compute what addresses are
// already connected or not.
//
// TODO Basically, we need to work harder on our good-peer/bad-peer marking.
// What we're currently doing in terms of marking good/bad peers is just a
// placeholder. It should not be the case that an address becomes old/vetted
// upon a single successful connection.
func (r *PEXReactor) ensurePeers() {
	numOutPeers, _, numDialing := r.Switch.NumPeers()
	numToDial := minNumOutboundPeers - (numOutPeers + numDialing)
	log.Info("Ensure peers", "numOutPeers", numOutPeers, "numDialing", numDialing, "numToDial", numToDial)
	if numToDial <= 0 {
		return
	}

	toDial := make(map[string]*NetAddress)

	// Try to pick numToDial addresses to dial.
	for i := 0; i < numToDial; i++ {
		// The purpose of newBias is to first prioritize old (more vetted) peers
		// when we have few connections, but to allow for new (less vetted) peers
		// if we already have many connections. This algorithm isn't perfect, but
		// it somewhat ensures that we prioritize connecting to more-vetted
		// peers.
		newBias := cmn.MinInt(numOutPeers, 8)*10 + 10
		var picked *NetAddress
		// Try to fetch a new peer 3 times.
		// This caps the maximum number of tries to 3 * numToDial.
		for j := 0; j < 3; j++ {
			try := r.book.PickAddress(newBias)
			if try == nil {
				break
			}
			_, alreadySelected := toDial[try.IP.String()]
			alreadyDialing := r.Switch.IsDialing(try)
			alreadyConnected := r.Switch.Peers().Has(try.IP.String())
			if alreadySelected || alreadyDialing || alreadyConnected {
				// log.Info("Cannot dial address", "addr", try,
				// 	"alreadySelected", alreadySelected,
				// 	"alreadyDialing", alreadyDialing,
				//  "alreadyConnected", alreadyConnected)
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
		toDial[picked.IP.String()] = picked
	}

	// Dial picked addresses
	for _, item := range toDial {
		go func(picked *NetAddress) {
			_, err := r.Switch.DialPeerWithAddress(picked, false)
			if err != nil {
				r.book.MarkAttempt(picked)
			}
		}(item)
	}

	// If we need more addresses, pick a random peer and ask for more.
	if r.book.NeedMoreAddrs() {
		if peers := r.Switch.Peers().List(); len(peers) > 0 {
			i := rand.Int() % len(peers)
			peer := peers[i]
			log.Info("No addresses to dial. Sending pexRequest to random peer", "peer", peer)
			r.RequestPEX(peer)
		}
	}
}

func (r *PEXReactor) flushMsgCountByPeer() {
	ticker := time.NewTicker(msgCountByPeerFlushInterval)

	for {
		select {
		case <-ticker.C:
			r.msgCountByPeer.Clear()
		case <-r.Quit:
			ticker.Stop()
			return
		}
	}
}

//-----------------------------------------------------------------------------
// Messages

const (
	msgTypeRequest = byte(0x01)
	msgTypeAddrs   = byte(0x02)
)

// PexMessage is a primary type for PEX messages. Underneath, it could contain
// either pexRequestMessage, or pexAddrsMessage messages.
type PexMessage interface{}

var _ = wire.RegisterInterface(
	struct{ PexMessage }{},
	wire.ConcreteType{&pexRequestMessage{}, msgTypeRequest},
	wire.ConcreteType{&pexAddrsMessage{}, msgTypeAddrs},
)

// DecodeMessage implements interface registered above.
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
