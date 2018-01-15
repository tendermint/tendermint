package p2p

import (
	"fmt"
	"math/rand"
	"reflect"
	"time"

	wire "github.com/tendermint/go-wire"
	cmn "github.com/tendermint/tmlibs/common"
)

const (
	// PexChannel is a channel for PEX messages
	PexChannel = byte(0x00)

	// period to ensure peers connected
	defaultEnsurePeersPeriod = 30 * time.Second
	minNumOutboundPeers      = 10
	maxPexMessageSize        = 1048576 // 1MB

	// maximum pex messages one peer can send to us during `msgCountByPeerFlushInterval`
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
	r.BaseReactor = *NewBaseReactor("PEXReactor", r)
	return r
}

// OnStart implements BaseService
func (r *PEXReactor) OnStart() error {
	if err := r.BaseReactor.OnStart(); err != nil {
		return err
	}
	err := r.book.Start()
	if err != nil && err != cmn.ErrAlreadyStarted {
		return err
	}
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
		{
			ID:                PexChannel,
			Priority:          1,
			SendQueueCapacity: 10,
		},
	}
}

// AddPeer implements Reactor by adding peer to the address book (if inbound)
// or by requesting more addresses (if outbound).
func (r *PEXReactor) AddPeer(p Peer) {
	if p.IsOutbound() {
		// For outbound peers, the address is already in the books.
		// Either it was added in DialSeeds or when we
		// received the peer's address in r.Receive
		if r.book.NeedMoreAddrs() {
			r.RequestPEX(p)
		}
	} else { // For inbound connections, the peer is its own source
		addr, err := NewNetAddressString(p.NodeInfo().ListenAddr)
		if err != nil {
			// peer gave us a bad ListenAddr. TODO: punish
			r.Logger.Error("Error in AddPeer: invalid peer address", "addr", p.NodeInfo().ListenAddr, "err", err)
			return
		}
		r.book.AddAddress(addr, addr)
	}
}

// RemovePeer implements Reactor.
func (r *PEXReactor) RemovePeer(p Peer, reason interface{}) {
	// If we aren't keeping track of local temp data for each peer here, then we
	// don't have to do anything.
}

// Receive implements Reactor by handling incoming PEX messages.
func (r *PEXReactor) Receive(chID byte, src Peer, msgBytes []byte) {
	srcAddrStr := src.NodeInfo().RemoteAddr
	srcAddr, err := NewNetAddressString(srcAddrStr)
	if err != nil {
		// this should never happen. TODO: cancel conn
		r.Logger.Error("Error in Receive: invalid peer address", "addr", srcAddrStr, "err", err)
		return
	}

	r.IncrementMsgCountForPeer(srcAddrStr)
	if r.ReachedMaxMsgCountForPeer(srcAddrStr) {
		r.Logger.Error("Maximum number of messages reached for peer", "peer", srcAddrStr)
		// TODO remove src from peers?
		return
	}

	_, msg, err := DecodeMessage(msgBytes)
	if err != nil {
		r.Logger.Error("Error decoding message", "err", err)
		return
	}
	r.Logger.Debug("Received message", "src", src, "chId", chID, "msg", msg)

	switch msg := msg.(type) {
	case *pexRequestMessage:
		// src requested some peers.
		// NOTE: we might send an empty selection
		r.SendAddrs(src, r.book.GetSelection())
	case *pexAddrsMessage:
		// We received some peer addresses from src.
		// TODO: (We don't want to get spammed with bad peers)
		for _, addr := range msg.Addrs {
			if addr != nil {
				r.book.AddAddress(addr, srcAddr)
			}
		}
	default:
		r.Logger.Error(fmt.Sprintf("Unknown message type %v", reflect.TypeOf(msg)))
	}
}

// RequestPEX asks peer for more addresses.
func (r *PEXReactor) RequestPEX(p Peer) {
	p.Send(PexChannel, struct{ PexMessage }{&pexRequestMessage{}})
}

// SendAddrs sends addrs to the peer.
func (r *PEXReactor) SendAddrs(p Peer, addrs []*NetAddress) {
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
	r.Logger.Info("Ensure peers", "numOutPeers", numOutPeers, "numDialing", numDialing, "numToDial", numToDial)
	if numToDial <= 0 {
		return
	}

	// bias to prefer more vetted peers when we have fewer connections.
	// not perfect, but somewhate ensures that we prioritize connecting to more-vetted
	// NOTE: range here is [10, 90]. Too high ?
	newBias := cmn.MinInt(numOutPeers, 8)*10 + 10

	toDial := make(map[string]*NetAddress)
	// Try maxAttempts times to pick numToDial addresses to dial
	maxAttempts := numToDial * 3
	for i := 0; i < maxAttempts && len(toDial) < numToDial; i++ {
		try := r.book.PickAddress(newBias)
		if try == nil {
			continue
		}
		if _, selected := toDial[try.IP.String()]; selected {
			continue
		}
		if dialling := r.Switch.IsDialing(try); dialling {
			continue
		}
		// XXX: Should probably use pubkey as peer key ...
		if connected := r.Switch.Peers().Has(try.String()); connected {
			continue
		}
		r.Logger.Info("Will dial address", "addr", try)
		toDial[try.IP.String()] = try
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
			i := rand.Int() % len(peers) // nolint: gas
			peer := peers[i]
			r.Logger.Info("No addresses to dial. Sending pexRequest to random peer", "peer", peer)
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

func init() {
	wire.RegisterInterface((*PexMessage)(nil), nil)
	wire.RegisterConcrete(&pexRequestMessage{}, "com.tendermint.p2p.request", nil)
	wire.RegisterConcrete(&pexAddrsMessage{}, "com.tendermint.p2p.addrs", nil)
}

// DecodeMessage implements interface registered above.
func DecodeMessage(bz []byte) (msgType byte, msg PexMessage, err error) {
	msgType = bz[0]
	pexMsg := struct{ PexMessage }{}
	err = wire.UnmarshalBinary(bz, pexMsg) // maxPexMessageSize
	return msgType, pexMsg.PexMessage, err
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
