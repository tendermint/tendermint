package pex

import (
	"bytes"
	"fmt"
	"math/rand"
	"reflect"
	"sort"
	"sync"
	"time"

	"github.com/pkg/errors"
	wire "github.com/tendermint/go-wire"
	cmn "github.com/tendermint/tmlibs/common"

	"github.com/tendermint/tendermint/p2p"
	"github.com/tendermint/tendermint/p2p/conn"
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

	maxAttemptsToDial = 16 // ~ 35h in total (last attempt - 18h)
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

	attemptsToDial sync.Map // address (string) -> {number of attempts (int), last time dialed (time.Time)}
}

// PEXReactorConfig holds reactor specific configuration data.
type PEXReactorConfig struct {
	// Seed/Crawler mode
	SeedMode bool

	// Seeds is a list of addresses reactor may use
	// if it can't connect to peers in the addrbook.
	Seeds []string
}

type _attemptsToDial struct {
	number     int
	lastDialed time.Time
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
	var (
		seed   = rand.New(rand.NewSource(time.Now().UnixNano()))
		jitter = seed.Int63n(r.ensurePeersPeriod.Nanoseconds())
	)

	// Randomize first round of communication to avoid thundering herd.
	// If no potential peers are present directly start connecting so we guarantee
	// swift setup with the help of configured seeds.
	if r.hasPotentialPeers() {
		time.Sleep(time.Duration(jitter))
	}

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
	var (
		out, in, dial = r.Switch.NumPeers()
		numToDial     = defaultMinNumOutboundPeers - (out + dial)
	)
	r.Logger.Info(
		"Ensure peers",
		"numOutPeers", out,
		"numInPeers", in,
		"numDialing", dial,
		"numToDial", numToDial,
	)

	if numToDial <= 0 {
		return
	}

	// bias to prefer more vetted peers when we have fewer connections.
	// not perfect, but somewhate ensures that we prioritize connecting to more-vetted
	// NOTE: range here is [10, 90]. Too high ?
	newBias := cmn.MinInt(out, 8)*10 + 10

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
	for _, addr := range toDial {
		go r.dialPeer(addr)
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
	if out+in+dial+len(toDial) == 0 {
		r.Logger.Info("No addresses to dial nor connected peers. Falling back to seeds")
		r.dialSeeds()
	}
}

func (r *PEXReactor) dialPeer(addr *p2p.NetAddress) {
	var attempts int
	var lastDialed time.Time
	if lAttempts, attempted := r.attemptsToDial.Load(addr.DialString()); attempted {
		attempts = lAttempts.(_attemptsToDial).number
		lastDialed = lAttempts.(_attemptsToDial).lastDialed
	}

	if attempts > maxAttemptsToDial {
		r.Logger.Error("Reached max attempts to dial", "addr", addr, "attempts", attempts)
		r.book.MarkBad(addr)
		return
	}

	// exponential backoff if it's not our first attempt to dial given address
	if attempts > 0 {
		jitterSeconds := time.Duration(rand.Float64() * float64(time.Second)) // 1s == (1e9 ns)
		backoffDuration := jitterSeconds + ((1 << uint(attempts)) * time.Second)
		sinceLastDialed := time.Since(lastDialed)
		if sinceLastDialed < backoffDuration {
			r.Logger.Debug("Too early to dial", "addr", addr, "backoff_duration", backoffDuration, "last_dialed", lastDialed, "time_since", sinceLastDialed)
			return
		}
	}

	err := r.Switch.DialPeerWithAddress(addr, false)
	if err != nil {
		r.Logger.Error("Dialing failed", "addr", addr, "err", err, "attempts", attempts)
		// TODO: detect more "bad peer" scenarios
		if _, ok := err.(p2p.ErrSwitchAuthenticationFailure); ok {
			r.book.MarkBad(addr)
		} else {
			r.book.MarkAttempt(addr)
		}
		// record attempt
		r.attemptsToDial.Store(addr.DialString(), _attemptsToDial{attempts + 1, time.Now()})
	} else {
		// cleanup any history
		r.attemptsToDial.Delete(addr.DialString())
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
		err := r.Switch.DialPeerWithAddress(seedAddr, false)
		if err == nil {
			return
		}
		r.Switch.Logger.Error("Error dialing seed", "err", err, "seed", seedAddr)
	}
	r.Switch.Logger.Error("Couldn't connect to any seeds")
}

// AttemptsToDial returns the number of attempts to dial specific address. It
// returns 0 if never attempted or successfully connected.
func (r *PEXReactor) AttemptsToDial(addr *p2p.NetAddress) int {
	lAttempts, attempted := r.attemptsToDial.Load(addr.DialString())
	if attempted {
		return lAttempts.(_attemptsToDial).number
	} else {
		return 0
	}
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

// hasPotentialPeers indicates if there is a potential peer to connect to, by
// consulting the Switch as well as the AddrBook.
func (r *PEXReactor) hasPotentialPeers() bool {
	out, in, dial := r.Switch.NumPeers()

	return out+in+dial > 0 && len(r.book.ListOfKnownAddresses()) > 0
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
		err := r.Switch.DialPeerWithAddress(pi.Addr, false)
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
	Addrs []*p2p.NetAddress
}

func (m *pexAddrsMessage) String() string {
	return fmt.Sprintf("[pexAddrs %v]", m.Addrs)
}
