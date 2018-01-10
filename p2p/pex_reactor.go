package p2p

import (
	"bytes"
	"fmt"
	"math/rand"
	"reflect"
	"sort"
	"time"

	"github.com/pkg/errors"
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
	BaseReactor

	book              *AddrBook
	config            *PEXReactorConfig
	ensurePeersPeriod time.Duration

	// maps to prevent abuse
	requestsSent         *cmn.CMap // ID->struct{}: unanswered send requests
	lastReceivedRequests *cmn.CMap // ID->time.Time: last time peer requested from us
}

// PEXReactorConfig holds reactor specific configuration data.
type PEXReactorConfig struct {
	// Seeds is a list of addresses reactor may use if it can't connect to peers
	// in the addrbook.
	Seeds []string
}

// NewPEXReactor creates new PEX reactor.
func NewPEXReactor(b *AddrBook, config *PEXReactorConfig) *PEXReactor {
	r := &PEXReactor{
		book:                 b,
		config:               config,
		ensurePeersPeriod:    defaultEnsurePeersPeriod,
		requestsSent:         cmn.NewCMap(),
		lastReceivedRequests: cmn.NewCMap(),
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

	// return err if user provided a bad seed address
	if err := r.checkSeeds(); err != nil {
		return err
	}

	go r.ensurePeersRoutine()
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
		// For outbound peers, the address is already in the books -
		// either via DialPeersAsync or r.Receive.
		// Ask it for more peers if we need.
		if r.book.NeedMoreAddrs() {
			r.RequestPEX(p)
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
		// We received a request for peers from src.
		if err := r.receiveRequest(src); err != nil {
			r.Switch.StopPeerForError(src, err)
			return
		}
		r.SendAddrs(src, r.book.GetSelection())
	case *pexAddrsMessage:
		// We received some peer addresses from src.
		if err := r.ReceivePEX(msg.Addrs, src); err != nil {
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

// RequestPEX asks peer for more addresses if we do not already
// have a request out for this peer.
func (r *PEXReactor) RequestPEX(p Peer) {
	id := string(p.ID())
	if r.requestsSent.Has(id) {
		return
	}
	r.requestsSent.Set(id, struct{}{})
	p.Send(PexChannel, struct{ PexMessage }{&pexRequestMessage{}})
}

// ReceivePEX adds the given addrs to the addrbook if theres an open
// request for this peer and deletes the open request.
// If there's no open request for the src peer, it returns an error.
func (r *PEXReactor) ReceivePEX(addrs []*NetAddress, src Peer) error {
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
func (r *PEXReactor) SendAddrs(p Peer, netAddrs []*NetAddress) {
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
	numOutPeers, numInPeers, numDialing := r.Switch.NumPeers()
	numToDial := minNumOutboundPeers - (numOutPeers + numDialing)
	r.Logger.Info("Ensure peers", "numOutPeers", numOutPeers, "numDialing", numDialing, "numToDial", numToDial)
	if numToDial <= 0 {
		return
	}

	// bias to prefer more vetted peers when we have fewer connections.
	// not perfect, but somewhate ensures that we prioritize connecting to more-vetted
	// NOTE: range here is [10, 90]. Too high ?
	newBias := cmn.MinInt(numOutPeers, 8)*10 + 10

	toDial := make(map[ID]*NetAddress)
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
		go func(picked *NetAddress) {
			_, err := r.Switch.DialPeerWithAddress(picked, false)
			if err != nil {
				r.book.MarkAttempt(picked)
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
			r.RequestPEX(peer)
		}
	}

	// If we are not connected to nor dialing anybody, fallback to dialing a seed.
	if numOutPeers+numInPeers+numDialing+len(toDial) == 0 {
		r.Logger.Info("No addresses to dial nor connected peers. Falling back to seeds")
		r.dialSeed()
	}
}

// check seed addresses are well formed
func (r *PEXReactor) checkSeeds() error {
	lSeeds := len(r.config.Seeds)
	if lSeeds == 0 {
		return nil
	}
	_, errs := NewNetAddressStrings(r.config.Seeds)
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}

// Explores the network searching for more peers. (continuous)
// Seed/Crawler Mode causes this node to quickly disconnect
// from peers, except other seed nodes.
func (r *PEXReactor) seedCrawlerMode() {
	// Do an initial crawl
	r.crawlPeers()

	// Fire periodically
	ticker := time.NewTicker(defaultSeedModePeriod)

	for {
		select {
		case <-ticker.C:
			r.attemptDisconnects()
			r.crawlPeers()
		case <-r.Quit:
			return
		}
	}
}

// crawlStatus handles temporary data needed for the
// network crawling performed during seed/crawler mode.
type crawlStatus struct {
	// The remote address of a potential peer we learned about
	Addr *NetAddress

	// Not empty if we are connected to the address
	PeerID string

	// The last time we attempt to reach this address
	LastAttempt time.Time

	// The last time we successfully reached this address
	LastSuccess time.Time
}

// oldestFirst implements sort.Interface for []crawlStatus
// based on the LastAttempt field.
type oldestFirst []crawlStatus

func (of oldestFirst) Len() int           { return len(of) }
func (of oldestFirst) Swap(i, j int)      { of[i], of[j] = of[j], of[i] }
func (of oldestFirst) Less(i, j int) bool { return of[i].LastAttempt.Before(of[j].LastAttempt) }

// getCrawlStatus returns addresses of potential peers that we wish to validate.
// NOTE: The status information is ordered as described above.
func (r *PEXReactor) getCrawlStatus() []crawlStatus {
	var of oldestFirst

	addrs := r.book.ListOfKnownAddresses()
	// Go through all the addresses in the AddressBook
	for _, addr := range addrs {
		var peerID string

		// Check if a peer is already connected from this addr
		if p := r.Switch.peers.GetByRemoteAddr(addr.Addr); p != nil {
			peerID = p.Key()
		}

		of = append(of, crawlStatus{
			Addr:        addr.Addr,
			PeerID:      peerID,
			LastAttempt: addr.LastAttempt,
			LastSuccess: addr.LastSuccess,
		})
	}
	sort.Sort(of)
	return of
}

// crawlPeers will crawl the network looking for new peer addresses. (once)
//
// TODO Basically, we need to work harder on our good-peer/bad-peer marking.
// What we're currently doing in terms of marking good/bad peers is just a
// placeholder. It should not be the case that an address becomes old/vetted
// upon a single successful connection.
func (r *PEXReactor) crawlPeers() {
	crawlerStatus := r.getCrawlStatus()

	now := time.Now()
	// Use addresses we know of to reach additional peers
	for _, cs := range crawlerStatus {
		// Do not dial peers that are already connected
		if cs.PeerID != "" {
			continue
		}
		// Do not attempt to connect with peers we recently dialed
		if now.Sub(cs.LastAttempt) < defaultCrawlPeerInterval {
			continue
		}
		// Otherwise, attempt to connect with the known address
		p, err := r.Switch.DialPeerWithAddress(cs.Addr, false)
		if err != nil {
			r.book.MarkAttempt(cs.Addr)
			continue
		}
		// Enter the peer ID into our crawl status information
		cs.PeerID = p.Key()
		r.book.MarkGood(cs.Addr)
	}
	// Crawl the connected peers asking for more addresses
	for _, cs := range crawlerStatus {
		if cs.PeerID == "" {
			continue
		}
		// We will wait a minimum period of time before crawling peers again
		if now.Sub(cs.LastAttempt) >= defaultCrawlPeerInterval {
			p := r.Switch.Peers().Get(cs.PeerID)
			if p != nil {
				r.RequestPEX(p)
				r.book.MarkAttempt(cs.Addr)
			}
		}
	}
}

// attemptDisconnects checks the crawlStatus info for Peers to disconnect from. (once)
func (r *PEXReactor) attemptDisconnects() {
	crawlerStatus := r.getCrawlStatus()

	now := time.Now()
	// Go through each peer we have connected with
	// looking for opportunities to disconnect
	for _, cs := range crawlerStatus {
		if cs.PeerID == "" {
			continue
		}
		// Remain connected to each peer for a minimum period of time
		if now.Sub(cs.LastSuccess) < defaultSeedDisconnectWaitPeriod {
			continue
		}
		// Fetch the Peer using the saved ID
		p := r.Switch.Peers().Get(cs.PeerID)
		if p == nil {
			continue
		}
		// Do not disconnect from persistent peers.
		// Specifically, we need to remain connected to other seeds
		if p.IsPersistent() {
			continue
		}
		// Otherwise, disconnect from the peer
		r.Switch.StopPeerGracefully(p)
	}
}

// crawlStatus handles temporary data needed for the
// network crawling performed during seed/crawler mode.
type crawlStatus struct {
	// The remote address of a potential peer we learned about
	Addr *NetAddress

	// Not empty if we are connected to the address
	PeerID string

	// The last time we attempt to reach this address
	LastAttempt time.Time

	// The last time we successfully reached this address
	LastSuccess time.Time
}

// oldestAttempt implements sort.Interface for []crawlStatus
// based on the LastAttempt field.
type oldestAttempt []crawlStatus

func (oa oldestAttempt) Len() int           { return len(oa) }
func (oa oldestAttempt) Swap(i, j int)      { oa[i], oa[j] = oa[j], oa[i] }
func (oa oldestAttempt) Less(i, j int) bool { return oa[i].LastAttempt.Before(oa[j].LastAttempt) }

// getCrawlStatus returns addresses of potential peers that we wish to validate.
// NOTE: The status information is ordered as described above.
func (r *PEXReactor) getCrawlStatus() []crawlStatus {
	var oa oldestAttempt

	addrs := r.book.ListOfKnownAddresses()
	// Go through all the addresses in the AddressBook
	for _, addr := range addrs {
		p := r.Switch.peers.GetByRemoteAddr(addr.Addr)

		oa = append(oa, crawlStatus{
			Addr:        addr.Addr,
			PeerID:      p.Key(),
			LastAttempt: addr.LastAttempt,
			LastSuccess: addr.LastSuccess,
		})
	}
	sort.Sort(oa)
	return oa
}

// crawlPeers will crawl the network looking for new peer addresses. (once)
//
// TODO Basically, we need to work harder on our good-peer/bad-peer marking.
// What we're currently doing in terms of marking good/bad peers is just a
// placeholder. It should not be the case that an address becomes old/vetted
// upon a single successful connection.
func (r *PEXReactor) crawlPeers() {
	crawlerStatus := r.getCrawlStatus()

	now := time.Now()
	// Use addresses we know of to reach additional peers
	for _, cs := range crawlerStatus {
		// Do not dial peers that are already connected
		if cs.PeerID != "" {
			continue
		}
		// Do not attempt to connect with peers we recently dialed
		if now.Sub(cs.LastAttempt) < defaultCrawlPeerInterval {
			continue
		}
		// Otherwise, attempt to connect with the known address
		p, err := r.Switch.DialPeerWithAddress(cs.Addr, false)
		if err != nil {
			r.book.MarkAttempt(cs.Addr)
			continue
		}
		// Enter the peer ID into our crawl status information
		cs.PeerID = p.Key()
		r.book.MarkGood(cs.Addr)
	}
	// Crawl the connected peers asking for more addresses
	for _, cs := range crawlerStatus {
		if cs.PeerID == "" {
			continue
		}
		// We will wait a minimum period of time before crawling peers again
		if now.Sub(cs.LastAttempt) >= defaultCrawlPeerInterval {
			p := r.Switch.peers.Get(cs.PeerID)
			if p != nil {
				r.RequestPEX(p)
			}
		}
	}
}

// attemptDisconnects checks the crawlStatus info for Peers to disconnect from. (once)
func (r *PEXReactor) attemptDisconnects() {
	crawlerStatus := r.getCrawlStatus()

	now := time.Now()
	// Go through each peer we have connected with
	// looking for opportunities to disconnect
	for _, cs := range crawlerStatus {
		if cs.PeerID == "" {
			continue
		}
		// Remain connected to each peer for a minimum period of time
		if now.Sub(cs.LastSuccess) < defaultSeedDisconnectWaitPeriod {
			continue
		}
		// Fetch the Peer using the saved ID
		p := r.Switch.peers.Get(cs.PeerID)
		if p == nil {
			continue
		}
		// Do not disconnect from persistent peers.
		// Specifically, we need to remain connected to other seeds
		if p.IsPersistent() {
			continue
		}
		// Otherwise, disconnect from the peer
		r.Switch.StopPeerGracefully(p)
	}
}

// randomly dial seeds until we connect to one or exhaust them
func (r *PEXReactor) dialSeed() {
	lSeeds := len(r.config.Seeds)
	if lSeeds == 0 {
		return
	}
	seedAddrs, _ := NewNetAddressStrings(r.config.Seeds)

	perm := r.Switch.rng.Perm(lSeeds)
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
