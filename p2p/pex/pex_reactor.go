package pex

import (
	"fmt"
	"math/rand"
	"reflect"
	"sort"
	"time"

	"github.com/pkg/errors"
	cmn "github.com/tendermint/tmlibs/common"

	"github.com/tendermint/tendermint/p2p"
	"github.com/tendermint/tendermint/p2p/conn"
	"github.com/tendermint/tendermint/wire"
)

type Peer = p2p.Peer

const (
	// PexChannel is a channel for PEX messages
	PexChannel = byte(0x00)

	maxPexMessageSize = 1048576 // 1MB

	// ensure we have enough peers
	defaultEnsurePeersPeriod   = 30 * time.Second
	defaultMinNumOutboundPeers = 10

	// Seed/Crawler constants
	// TODO:
	// We want seeds to only advertise good peers.
	// Peers are marked by external mechanisms.
	// We need a config value that can be set to be
	// on the order of how long it would take before a good
	// peer is marked good.
	defaultSeedDisconnectWaitPeriod = 2 * time.Minute  // disconnect after this
	defaultCrawlPeerInterval        = 2 * time.Minute  // dont redial for this. TODO: back-off
	defaultCrawlPeersPeriod         = 30 * time.Second // check some peers every this
)

// PEXReactor handles PEX (peer exchange) and ensures that an
// adequate number of peers are connected to the switch.
//
// It uses `AddrBook` (address book) to store `NetAddress`es of the peers.
//
// ## Preventing abuse
//
// Only accept pexAddrsMsg from peers we sent a corresponding pexRequestMsg too.
// Only accept one pexRequestMsg every ~defaultEnsurePeersPeriod.
type PEXReactor struct {
	p2p.BaseReactor

	book              AddrBook
	config            *PEXReactorConfig
	ensurePeersPeriod time.Duration

	// maps to prevent abuse
	requestsSent         *cmn.CMap // ID->struct{}: unanswered send requests
	lastReceivedRequests *cmn.CMap // ID->time.Time: last time peer requested from us
}

// PEXReactorConfig holds reactor specific configuration data.
type PEXReactorConfig struct {
	// Seed/Crawler mode
	SeedMode bool

	// Seeds is a list of addresses reactor may use
	// if it can't connect to peers in the addrbook.
	Seeds []string
}

// NewPEXReactor creates new PEX reactor.
func NewPEXReactor(b AddrBook, config *PEXReactorConfig) *PEXReactor {
	r := &PEXReactor{
		book:                 b,
		config:               config,
		ensurePeersPeriod:    defaultEnsurePeersPeriod,
		requestsSent:         cmn.NewCMap(),
		lastReceivedRequests: cmn.NewCMap(),
	}
	r.BaseReactor = *p2p.NewBaseReactor("PEXReactor", r)
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

	// return err if user provided a bad seed address
	if err := r.checkSeeds(); err != nil {
		return err
	}

	// Check if this node should run
	// in seed/crawler mode
	if r.config.SeedMode {
		go r.crawlPeersRoutine()
	} else {
		go r.ensurePeersRoutine()
	}
	return nil
}

// OnStop implements BaseService
func (r *PEXReactor) OnStop() {
	r.BaseReactor.OnStop()
	r.book.Stop()
}

// GetChannels implements Reactor
func (r *PEXReactor) GetChannels() []*conn.ChannelDescriptor {
	return []*conn.ChannelDescriptor{
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
		// For outbound peers, the address is already in the books -
		// either via DialPeersAsync or r.Receive.
		// Ask it for more peers if we need.
		if r.book.NeedMoreAddrs() {
			r.RequestAddrs(p)
		}
	} else {
		// For inbound peers, the peer is its own source,
		// and its NodeInfo has already been validated.
		// Let the ensurePeersRoutine handle asking for more
		// peers when we need - we don't trust inbound peers as much.
		addr := p.NodeInfo().NetAddress()
		r.book.AddAddress(addr, addr)
	}
}

// RemovePeer implements Reactor.
func (r *PEXReactor) RemovePeer(p Peer, reason interface{}) {
	id := string(p.ID())
	r.requestsSent.Delete(id)
	r.lastReceivedRequests.Delete(id)
}

// Receive implements Reactor by handling incoming PEX messages.
func (r *PEXReactor) Receive(chID byte, src Peer, msgBytes []byte) {
	_, msg, err := DecodeMessage(msgBytes)
	if err != nil {
		r.Logger.Error("Error decoding message", "err", err)
		return
	}
	r.Logger.Debug("Received message", "src", src, "chId", chID, "msg", msg)

	switch msg := msg.(type) {
	case *pexRequestMessage:
		// Check we're not receiving too many requests
		if err := r.receiveRequest(src); err != nil {
			r.Switch.StopPeerForError(src, err)
			return
		}

		// Seeds disconnect after sending a batch of addrs
		if r.config.SeedMode {
			// TODO: should we be more selective ?
			r.SendAddrs(src, r.book.GetSelection())
			r.Switch.StopPeerGracefully(src)
		} else {
			r.SendAddrs(src, r.book.GetSelection())
		}

	case *pexAddrsMessage:
		// If we asked for addresses, add them to the book
		if err := r.ReceiveAddrs(msg.Addrs, src); err != nil {
			r.Switch.StopPeerForError(src, err)
			return
		}
	default:
		r.Logger.Error(fmt.Sprintf("Unknown message type %v", reflect.TypeOf(msg)))
	}
}

func (r *PEXReactor) receiveRequest(src Peer) error {
	id := string(src.ID())
	v := r.lastReceivedRequests.Get(id)
	if v == nil {
		// initialize with empty time
		lastReceived := time.Time{}
		r.lastReceivedRequests.Set(id, lastReceived)
		return nil
	}

	lastReceived := v.(time.Time)
	if lastReceived.Equal(time.Time{}) {
		// first time gets a free pass. then we start tracking the time
		lastReceived = time.Now()
		r.lastReceivedRequests.Set(id, lastReceived)
		return nil
	}

	now := time.Now()
	if now.Sub(lastReceived) < r.ensurePeersPeriod/3 {
		return fmt.Errorf("Peer (%v) is sending too many PEX requests. Disconnecting", src.ID())
	}
	r.lastReceivedRequests.Set(id, now)
	return nil
}

// RequestAddrs asks peer for more addresses if we do not already
// have a request out for this peer.
func (r *PEXReactor) RequestAddrs(p Peer) {
	id := string(p.ID())
	if r.requestsSent.Has(id) {
		return
	}
	r.requestsSent.Set(id, struct{}{})
	p.Send(PexChannel, struct{ PexMessage }{&pexRequestMessage{}})
}

// ReceiveAddrs adds the given addrs to the addrbook if theres an open
// request for this peer and deletes the open request.
// If there's no open request for the src peer, it returns an error.
func (r *PEXReactor) ReceiveAddrs(addrs []*p2p.NetAddress, src Peer) error {
	id := string(src.ID())

	if !r.requestsSent.Has(id) {
		return errors.New("Received unsolicited pexAddrsMessage")
	}

	r.requestsSent.Delete(id)

	srcAddr := src.NodeInfo().NetAddress()
	for _, netAddr := range addrs {
		if netAddr != nil {
			r.book.AddAddress(netAddr, srcAddr)
		}
	}
	return nil
}

// SendAddrs sends addrs to the peer.
func (r *PEXReactor) SendAddrs(p Peer, netAddrs []*p2p.NetAddress) {
	p.Send(PexChannel, struct{ PexMessage }{&pexAddrsMessage{Addrs: netAddrs}})
}

// SetEnsurePeersPeriod sets period to ensure peers connected.
func (r *PEXReactor) SetEnsurePeersPeriod(d time.Duration) {
	r.ensurePeersPeriod = d
}

// Ensures that sufficient peers are connected. (continuous)
func (r *PEXReactor) ensurePeersRoutine() {
	// Randomize when routine starts
	ensurePeersPeriodMs := r.ensurePeersPeriod.Nanoseconds() / 1e6
	time.Sleep(time.Duration(rand.Int63n(ensurePeersPeriodMs)) * time.Millisecond)

	// fire once immediately.
	// ensures we dial the seeds right away if the book is empty
	r.ensurePeers()

	// fire periodically
	ticker := time.NewTicker(r.ensurePeersPeriod)
	for {
		select {
		case <-ticker.C:
			r.ensurePeers()
		case <-r.Quit():
			ticker.Stop()
			return
		}
	}
}

// ensurePeers ensures that sufficient peers are connected. (once)
//
// heuristic that we haven't perfected yet, or, perhaps is manually edited by
// the node operator. It should not be used to compute what addresses are
// already connected or not.
func (r *PEXReactor) ensurePeers() {
	numOutPeers, numInPeers, numDialing := r.Switch.NumPeers()
	numToDial := defaultMinNumOutboundPeers - (numOutPeers + numDialing)
	r.Logger.Info("Ensure peers", "numOutPeers", numOutPeers, "numDialing", numDialing, "numToDial", numToDial)
	if numToDial <= 0 {
		return
	}

	// bias to prefer more vetted peers when we have fewer connections.
	// not perfect, but somewhate ensures that we prioritize connecting to more-vetted
	// NOTE: range here is [10, 90]. Too high ?
	newBias := cmn.MinInt(numOutPeers, 8)*10 + 10

	toDial := make(map[p2p.ID]*p2p.NetAddress)
	// Try maxAttempts times to pick numToDial addresses to dial
	maxAttempts := numToDial * 3
	for i := 0; i < maxAttempts && len(toDial) < numToDial; i++ {
		try := r.book.PickAddress(newBias)
		if try == nil {
			continue
		}
		if _, selected := toDial[try.ID]; selected {
			continue
		}
		if dialling := r.Switch.IsDialing(try.ID); dialling {
			continue
		}
		if connected := r.Switch.Peers().Has(try.ID); connected {
			continue
		}
		r.Logger.Info("Will dial address", "addr", try)
		toDial[try.ID] = try
	}

	// Dial picked addresses
	for _, item := range toDial {
		go func(picked *p2p.NetAddress) {
			_, err := r.Switch.DialPeerWithAddress(picked, false)
			if err != nil {
				// TODO: detect more "bad peer" scenarios
				if _, ok := err.(p2p.ErrSwitchAuthenticationFailure); ok {
					r.book.MarkBad(picked)
				} else {
					r.book.MarkAttempt(picked)
				}
			}
		}(item)
	}

	// If we need more addresses, pick a random peer and ask for more.
	if r.book.NeedMoreAddrs() {
		peers := r.Switch.Peers().List()
		peersCount := len(peers)
		if peersCount > 0 {
			peer := peers[rand.Int()%peersCount] // nolint: gas
			r.Logger.Info("We need more addresses. Sending pexRequest to random peer", "peer", peer)
			r.RequestAddrs(peer)
		}
	}

	// If we are not connected to nor dialing anybody, fallback to dialing a seed.
	if numOutPeers+numInPeers+numDialing+len(toDial) == 0 {
		r.Logger.Info("No addresses to dial nor connected peers. Falling back to seeds")
		r.dialSeeds()
	}
}

// check seed addresses are well formed
func (r *PEXReactor) checkSeeds() error {
	lSeeds := len(r.config.Seeds)
	if lSeeds == 0 {
		return nil
	}
	_, errs := p2p.NewNetAddressStrings(r.config.Seeds)
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}

// randomly dial seeds until we connect to one or exhaust them
func (r *PEXReactor) dialSeeds() {
	lSeeds := len(r.config.Seeds)
	if lSeeds == 0 {
		return
	}
	seedAddrs, _ := p2p.NewNetAddressStrings(r.config.Seeds)

	perm := rand.Perm(lSeeds)
	// perm := r.Switch.rng.Perm(lSeeds)
	for _, i := range perm {
		// dial a random seed
		seedAddr := seedAddrs[i]
		peer, err := r.Switch.DialPeerWithAddress(seedAddr, false)
		if err != nil {
			r.Switch.Logger.Error("Error dialing seed", "err", err, "seed", seedAddr)
		} else {
			r.Switch.Logger.Info("Connected to seed", "peer", peer)
			return
		}
	}
	r.Switch.Logger.Error("Couldn't connect to any seeds")
}

//----------------------------------------------------------

// Explores the network searching for more peers. (continuous)
// Seed/Crawler Mode causes this node to quickly disconnect
// from peers, except other seed nodes.
func (r *PEXReactor) crawlPeersRoutine() {
	// Do an initial crawl
	r.crawlPeers()

	// Fire periodically
	ticker := time.NewTicker(defaultCrawlPeersPeriod)

	for {
		select {
		case <-ticker.C:
			r.attemptDisconnects()
			r.crawlPeers()
		case <-r.Quit():
			return
		}
	}
}

// crawlPeerInfo handles temporary data needed for the
// network crawling performed during seed/crawler mode.
type crawlPeerInfo struct {
	// The listening address of a potential peer we learned about
	Addr *p2p.NetAddress

	// The last time we attempt to reach this address
	LastAttempt time.Time

	// The last time we successfully reached this address
	LastSuccess time.Time
}

// oldestFirst implements sort.Interface for []crawlPeerInfo
// based on the LastAttempt field.
type oldestFirst []crawlPeerInfo

func (of oldestFirst) Len() int           { return len(of) }
func (of oldestFirst) Swap(i, j int)      { of[i], of[j] = of[j], of[i] }
func (of oldestFirst) Less(i, j int) bool { return of[i].LastAttempt.Before(of[j].LastAttempt) }

// getPeersToCrawl returns addresses of potential peers that we wish to validate.
// NOTE: The status information is ordered as described above.
func (r *PEXReactor) getPeersToCrawl() []crawlPeerInfo {
	var of oldestFirst

	// TODO: be more selective
	addrs := r.book.ListOfKnownAddresses()
	for _, addr := range addrs {
		if len(addr.ID()) == 0 {
			continue // dont use peers without id
		}

		of = append(of, crawlPeerInfo{
			Addr:        addr.Addr,
			LastAttempt: addr.LastAttempt,
			LastSuccess: addr.LastSuccess,
		})
	}
	sort.Sort(of)
	return of
}

// crawlPeers will crawl the network looking for new peer addresses. (once)
func (r *PEXReactor) crawlPeers() {
	peerInfos := r.getPeersToCrawl()

	now := time.Now()
	// Use addresses we know of to reach additional peers
	for _, pi := range peerInfos {
		// Do not attempt to connect with peers we recently dialed
		if now.Sub(pi.LastAttempt) < defaultCrawlPeerInterval {
			continue
		}
		// Otherwise, attempt to connect with the known address
		_, err := r.Switch.DialPeerWithAddress(pi.Addr, false)
		if err != nil {
			r.book.MarkAttempt(pi.Addr)
			continue
		}
	}
	// Crawl the connected peers asking for more addresses
	for _, pi := range peerInfos {
		// We will wait a minimum period of time before crawling peers again
		if now.Sub(pi.LastAttempt) >= defaultCrawlPeerInterval {
			peer := r.Switch.Peers().Get(pi.Addr.ID)
			if peer != nil {
				r.RequestAddrs(peer)
			}
		}
	}
}

// attemptDisconnects checks if we've been with each peer long enough to disconnect
func (r *PEXReactor) attemptDisconnects() {
	for _, peer := range r.Switch.Peers().List() {
		status := peer.Status()
		if status.Duration < defaultSeedDisconnectWaitPeriod {
			continue
		}
		if peer.IsPersistent() {
			continue
		}
		r.Switch.StopPeerGracefully(peer)
	}
}

//-----------------------------------------------------------------------------
// Messages

// PexMessage is a primary type for PEX messages. Underneath, it could contain
// either pexRequestMessage, or pexAddrsMessage messages.
type PexMessage interface{}

func init() {
	wire.RegisterInterface((*PexMessage)(nil), nil)
	wire.RegisterConcrete(&pexRequestMessage{}, "com.tendermint.p2p.pex.RequestMessage", nil)
	wire.RegisterConcrete(&pexAddrsMessage{}, "com.tendermint.p2p.pex.AddrsMessage", nil)
}

// DecodeMessage implements interface registered above.
func DecodeMessage(bz []byte) (byte, PexMessage, error) {
	msgType := bz[0]
	var msg PexMessage
	err := wire.UnmarshalBinary(bz, &msg)
	return msgType, msg, err
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
	Addrs []*p2p.NetAddress
}

func (m *pexAddrsMessage) String() string {
	return fmt.Sprintf("[pexAddrs %v]", m.Addrs)
}
