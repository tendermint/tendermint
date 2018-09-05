package pex

import (
	"fmt"
	"reflect"
	"sort"
	"sync"
	"time"

	"github.com/pkg/errors"

	amino "github.com/tendermint/go-amino"
	cmn "github.com/tendermint/tendermint/libs/common"
	"github.com/tendermint/tendermint/p2p"
	"github.com/tendermint/tendermint/p2p/conn"
)

type Peer = p2p.Peer

const (
	// PexChannel is a channel for PEX messages
	PexChannel = byte(0x00)

	// over-estimate of max NetAddress size
	// hexID (40) + IP (16) + Port (2) + Name (100) ...
	// NOTE: dont use massive DNS name ..
	maxAddressSize = 256

	// NOTE: amplificaiton factor!
	// small request results in up to maxMsgSize response
	maxMsgSize = maxAddressSize * maxGetSelection

	// ensure we have enough peers
	defaultEnsurePeersPeriod = 30 * time.Second

	// Seed/Crawler constants

	// We want seeds to only advertise good peers. Therefore they should wait at
	// least as long as we expect it to take for a peer to become good before
	// disconnecting.
	// see consensus/reactor.go: blocksToContributeToBecomeGoodPeer
	// 10000 blocks assuming 1s blocks ~ 2.7 hours.
	defaultSeedDisconnectWaitPeriod = 3 * time.Hour

	defaultCrawlPeerInterval = 2 * time.Minute // don't redial for this. TODO: back-off. what for?

	defaultCrawlPeersPeriod = 30 * time.Second // check some peers every this

	maxAttemptsToDial = 16 // ~ 35h in total (last attempt - 18h)

	// if node connects to seed, it does not have any trusted peers.
	// Especially in the beginning, node should have more trusted peers than
	// untrusted.
	biasToSelectNewPeers = 30 // 70 to select good peers
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
	ensurePeersPeriod time.Duration // TODO: should go in the config

	// maps to prevent abuse
	requestsSent         *cmn.CMap // ID->struct{}: unanswered send requests
	lastReceivedRequests *cmn.CMap // ID->time.Time: last time peer requested from us

	seedAddrs []*p2p.NetAddress

	attemptsToDial sync.Map // address (string) -> {number of attempts (int), last time dialed (time.Time)}
}

func (r *PEXReactor) minReceiveRequestInterval() time.Duration {
	// NOTE: must be less than ensurePeersPeriod, otherwise we'll request
	// peers too quickly from others and they'll think we're bad!
	return r.ensurePeersPeriod / 3
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
	err := r.book.Start()
	if err != nil && err != cmn.ErrAlreadyStarted {
		return err
	}

	numOnline, seedAddrs, err := r.checkSeeds()
	if err != nil {
		return err
	} else if numOnline == 0 && r.book.Empty() {
		return errors.New("Address book is empty, and could not connect to any seed nodes")
	}

	r.seedAddrs = seedAddrs

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
		// inbound peer is its own source
		addr := p.NodeInfo().NetAddress()
		src := addr

		// add to book. dont RequestAddrs right away because
		// we don't trust inbound as much - let ensurePeersRoutine handle it.
		err := r.book.AddAddress(addr, src)
		r.logErrAddrBook(err)
	}
}

func (r *PEXReactor) logErrAddrBook(err error) {
	if err != nil {
		switch err.(type) {
		case ErrAddrBookNilAddr:
			r.Logger.Error("Failed to add new address", "err", err)
		default:
			// non-routable, self, full book, private, etc.
			r.Logger.Debug("Failed to add new address", "err", err)
		}
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
	msg, err := decodeMsg(msgBytes)
	if err != nil {
		r.Logger.Error("Error decoding message", "src", src, "chId", chID, "msg", msg, "err", err, "bytes", msgBytes)
		r.Switch.StopPeerForError(src, err)
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
		// NOTE: this is a prime candidate for amplification attacks
		// so it's important we
		// 1) restrict how frequently peers can request
		// 2) limit the output size
		if r.config.SeedMode {
			r.SendAddrs(src, r.book.GetSelectionWithBias(biasToSelectNewPeers))
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

// enforces a minimum amount of time between requests
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
	minInterval := r.minReceiveRequestInterval()
	if now.Sub(lastReceived) < minInterval {
		return fmt.Errorf("Peer (%v) sent next PEX request too soon. lastReceived: %v, now: %v, minInterval: %v. Disconnecting",
			src.ID(),
			lastReceived,
			now,
			minInterval,
		)
	}
	r.lastReceivedRequests.Set(id, now)
	return nil
}

// RequestAddrs asks peer for more addresses if we do not already
// have a request out for this peer.
func (r *PEXReactor) RequestAddrs(p Peer) {
	r.Logger.Debug("Request addrs", "from", p)
	id := string(p.ID())
	if r.requestsSent.Has(id) {
		return
	}
	r.requestsSent.Set(id, struct{}{})
	p.Send(PexChannel, cdc.MustMarshalBinaryBare(&pexRequestMessage{}))
}

// ReceiveAddrs adds the given addrs to the addrbook if theres an open
// request for this peer and deletes the open request.
// If there's no open request for the src peer, it returns an error.
func (r *PEXReactor) ReceiveAddrs(addrs []*p2p.NetAddress, src Peer) error {
	id := string(src.ID())
	if !r.requestsSent.Has(id) {
		return cmn.NewError("Received unsolicited pexAddrsMessage")
	}
	r.requestsSent.Delete(id)

	srcAddr := src.NodeInfo().NetAddress()
	for _, netAddr := range addrs {
		// NOTE: GetSelection methods should never return nil addrs
		if netAddr == nil {
			return cmn.NewError("received nil addr")
		}

		err := r.book.AddAddress(netAddr, srcAddr)
		r.logErrAddrBook(err)

		// If this address came from a seed node, try to connect to it without waiting.
		for _, seedAddr := range r.seedAddrs {
			if seedAddr.Equals(srcAddr) {
				r.ensurePeers()
			}
		}
	}
	return nil
}

// SendAddrs sends addrs to the peer.
func (r *PEXReactor) SendAddrs(p Peer, netAddrs []*p2p.NetAddress) {
	p.Send(PexChannel, cdc.MustMarshalBinaryBare(&pexAddrsMessage{Addrs: netAddrs}))
}

// SetEnsurePeersPeriod sets period to ensure peers connected.
func (r *PEXReactor) SetEnsurePeersPeriod(d time.Duration) {
	r.ensurePeersPeriod = d
}

// Ensures that sufficient peers are connected. (continuous)
func (r *PEXReactor) ensurePeersRoutine() {
	var (
		seed   = cmn.NewRand()
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
		numToDial     = r.Switch.MaxNumOutboundPeers() - (out + dial)
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
		if r.Switch.IsDialingOrExistingAddress(try) {
			continue
		}
		// TODO: consider moving some checks from toDial into here
		// so we don't even consider dialing peers that we want to wait
		// before dialling again, or have dialed too many times already
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
			peer := peers[cmn.RandInt()%peersCount] // nolint: gas
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

func (r *PEXReactor) dialAttemptsInfo(addr *p2p.NetAddress) (attempts int, lastDialed time.Time) {
	_attempts, ok := r.attemptsToDial.Load(addr.DialString())
	if !ok {
		return
	}
	atd := _attempts.(_attemptsToDial)
	return atd.number, atd.lastDialed
}

func (r *PEXReactor) dialPeer(addr *p2p.NetAddress) {
	attempts, lastDialed := r.dialAttemptsInfo(addr)

	if attempts > maxAttemptsToDial {
		r.Logger.Error("Reached max attempts to dial", "addr", addr, "attempts", attempts)
		r.book.MarkBad(addr)
		return
	}

	// exponential backoff if it's not our first attempt to dial given address
	if attempts > 0 {
		jitterSeconds := time.Duration(cmn.RandFloat64() * float64(time.Second)) // 1s == (1e9 ns)
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
			r.attemptsToDial.Delete(addr.DialString())
		} else {
			r.book.MarkAttempt(addr)
			// FIXME: if the addr is going to be removed from the addrbook (hard to
			// tell at this point), we need to Delete it from attemptsToDial, not
			// record another attempt.
			// record attempt
			r.attemptsToDial.Store(addr.DialString(), _attemptsToDial{attempts + 1, time.Now()})
		}
	} else {
		// cleanup any history
		r.attemptsToDial.Delete(addr.DialString())
	}
}

// checkSeeds checks that addresses are well formed.
// Returns number of seeds we can connect to, along with all seeds addrs.
// return err if user provided any badly formatted seed addresses.
// Doesn't error if the seed node can't be reached.
// numOnline returns -1 if no seed nodes were in the initial configuration.
func (r *PEXReactor) checkSeeds() (numOnline int, netAddrs []*p2p.NetAddress, err error) {
	lSeeds := len(r.config.Seeds)
	if lSeeds == 0 {
		return -1, nil, nil
	}
	netAddrs, errs := p2p.NewNetAddressStrings(r.config.Seeds)
	numOnline = lSeeds - len(errs)
	for _, err := range errs {
		switch e := err.(type) {
		case p2p.ErrNetAddressLookup:
			r.Logger.Error("Connecting to seed failed", "err", e)
		default:
			return 0, nil, errors.Wrap(e, "seed node configuration has error")
		}
	}
	return
}

// randomly dial seeds until we connect to one or exhaust them
func (r *PEXReactor) dialSeeds() {
	perm := cmn.RandPerm(len(r.seedAddrs))
	// perm := r.Switch.rng.Perm(lSeeds)
	for _, i := range perm {
		// dial a random seed
		seedAddr := r.seedAddrs[i]
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
	}
	return 0
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
		// Ask for more addresses
		peer := r.Switch.Peers().Get(pi.Addr.ID)
		if peer != nil {
			r.RequestAddrs(peer)
		}
	}
}

// attemptDisconnects checks if we've been with each peer long enough to disconnect
func (r *PEXReactor) attemptDisconnects() {
	for _, peer := range r.Switch.Peers().List() {
		if peer.Status().Duration < defaultSeedDisconnectWaitPeriod {
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

func RegisterPexMessage(cdc *amino.Codec) {
	cdc.RegisterInterface((*PexMessage)(nil), nil)
	cdc.RegisterConcrete(&pexRequestMessage{}, "tendermint/p2p/PexRequestMessage", nil)
	cdc.RegisterConcrete(&pexAddrsMessage{}, "tendermint/p2p/PexAddrsMessage", nil)
}

func decodeMsg(bz []byte) (msg PexMessage, err error) {
	if len(bz) > maxMsgSize {
		return msg, fmt.Errorf("Msg exceeds max size (%d > %d)", len(bz), maxMsgSize)
	}
	err = cdc.UnmarshalBinaryBare(bz, &msg)
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
