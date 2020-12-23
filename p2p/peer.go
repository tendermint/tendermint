package p2p

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"runtime/debug"
	"strconv"
	"sync"
	"time"

	"github.com/tendermint/tendermint/libs/cmap"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/libs/service"
	tmconn "github.com/tendermint/tendermint/p2p/conn"
)

// PeerAddress is a peer address URL.
type PeerAddress struct {
	*url.URL
}

// ParsePeerAddress parses a peer address URL into a PeerAddress.
func ParsePeerAddress(address string) (PeerAddress, error) {
	u, err := url.Parse(address)
	if err != nil || u == nil {
		return PeerAddress{}, fmt.Errorf("unable to parse peer address %q: %w", address, err)
	}
	if u.Scheme == "" {
		u.Scheme = string(defaultProtocol)
	}
	pa := PeerAddress{URL: u}
	if err = pa.Validate(); err != nil {
		return PeerAddress{}, err
	}
	return pa, nil
}

// NodeID returns the address node ID.
func (a PeerAddress) NodeID() NodeID {
	return NodeID(a.User.Username())
}

// Resolve resolves a PeerAddress into a set of Endpoints, by expanding
// out a DNS name in Host to its IP addresses. Field mapping:
//
//   Scheme → Endpoint.Protocol
//   Host   → Endpoint.IP
//   User   → Endpoint.PeerID
//   Port   → Endpoint.Port
//   Path+Query+Fragment,Opaque → Endpoint.Path
//
func (a PeerAddress) Resolve(ctx context.Context) ([]Endpoint, error) {
	ips, err := net.DefaultResolver.LookupIP(ctx, "ip", a.Host)
	if err != nil {
		return nil, err
	}
	port, err := a.parsePort()
	if err != nil {
		return nil, err
	}

	path := a.Path
	if a.RawPath != "" {
		path = a.RawPath
	}
	if a.Opaque != "" { // used for e.g. "about:blank" style URLs
		path = a.Opaque
	}
	if a.RawQuery != "" {
		path += "?" + a.RawQuery
	}
	if a.RawFragment != "" {
		path += "#" + a.RawFragment
	}

	endpoints := make([]Endpoint, 0, len(ips))
	for _, ip := range ips {
		endpoint := Endpoint{
			PeerID:   NodeID(a.User.Username()),
			Protocol: Protocol(a.Scheme),
			IP:       ip,
			Port:     port,
			Path:     path,
		}
		endpoints = append(endpoints, endpoint)
	}
	return endpoints, nil
}

// Validates validates a PeerAddress.
func (a PeerAddress) Validate() error {
	if a.Scheme == "" {
		return errors.New("no protocol")
	}
	if id := a.User.Username(); id == "" {
		return errors.New("no peer ID")
	} else if err := NodeID(id).Validate(); err != nil {
		return fmt.Errorf("invalid peer ID: %w", err)
	}
	if a.Hostname() == "" && len(a.Query()) == 0 && a.Opaque == "" {
		return errors.New("no host or path given")
	}
	if port, err := a.parsePort(); err != nil {
		return err
	} else if port > 0 && a.Hostname() == "" {
		return errors.New("cannot specify port without host")
	}
	return nil
}

// parsePort returns the port number as a uint16.
func (a PeerAddress) parsePort() (uint16, error) {
	if portString := a.Port(); portString != "" {
		port64, err := strconv.ParseUint(portString, 10, 16)
		if err != nil {
			return 0, fmt.Errorf("invalid port %q: %w", portString, err)
		}
		return uint16(port64), nil
	}
	return 0, nil
}

// PeerStatus specifies peer statuses.
type PeerStatus string

const (
	PeerStatusNew     = PeerStatus("new")     // New peer which we haven't tried to contact yet.
	PeerStatusUp      = PeerStatus("up")      // Peer which we have an active connection to.
	PeerStatusDown    = PeerStatus("down")    // Peer which we're temporarily disconnected from.
	PeerStatusRemoved = PeerStatus("removed") // Peer which has been removed.
	PeerStatusBanned  = PeerStatus("banned")  // Peer which is banned for misbehavior.
)

// PeerPriority specifies peer priorities.
type PeerPriority int

const (
	PeerPriorityNormal PeerPriority = iota + 1
	PeerPriorityValidator
	PeerPriorityPersistent
)

// PeerError is a peer error reported by a reactor via the Error channel. The
// severity may cause the peer to be disconnected or banned depending on policy.
type PeerError struct {
	PeerID   NodeID
	Err      error
	Severity PeerErrorSeverity
}

// PeerErrorSeverity determines the severity of a peer error.
type PeerErrorSeverity string

const (
	PeerErrorSeverityLow      PeerErrorSeverity = "low"      // Mostly ignored.
	PeerErrorSeverityHigh     PeerErrorSeverity = "high"     // May disconnect.
	PeerErrorSeverityCritical PeerErrorSeverity = "critical" // Ban.
)

// PeerUpdatesCh defines a wrapper around a PeerUpdate go channel that allows
// a reactor to listen for peer updates and safely close it when stopping.
type PeerUpdatesCh struct {
	closeOnce sync.Once

	// updatesCh defines the go channel in which the router sends peer updates to
	// reactors. Each reactor will have its own PeerUpdatesCh to listen for updates
	// from.
	updatesCh chan PeerUpdate

	// doneCh is used to signal that a PeerUpdatesCh is closed. It is the
	// reactor's responsibility to invoke Close.
	doneCh chan struct{}
}

// NewPeerUpdates returns a reference to a new PeerUpdatesCh.
func NewPeerUpdates() *PeerUpdatesCh {
	return &PeerUpdatesCh{
		updatesCh: make(chan PeerUpdate, 1),
		doneCh:    make(chan struct{}),
	}
}

// Updates returns a read-only go channel where a consuming reactor can listen
// for peer updates sent from the router.
func (puc *PeerUpdatesCh) Updates() <-chan PeerUpdate {
	return puc.updatesCh
}

// Close closes the PeerUpdatesCh channel. It should only be closed by the respective
// reactor when stopping and ensure nothing is listening for updates.
//
// NOTE: After a PeerUpdatesCh is closed, the router may safely assume it can no
// longer send on the internal updatesCh, however it should NEVER explicitly close
// it as that could result in panics by sending on a closed channel.
func (puc *PeerUpdatesCh) Close() {
	puc.closeOnce.Do(func() {
		close(puc.doneCh)
	})
}

// Done returns a read-only version of the PeerUpdatesCh's internal doneCh go
// channel that should be used by a router to signal when it is safe to explicitly
// not send any peer updates.
func (puc *PeerUpdatesCh) Done() <-chan struct{} {
	return puc.doneCh
}

// PeerUpdate is a peer status update for reactors.
type PeerUpdate struct {
	PeerID NodeID
	Status PeerStatus
}

// peerStore manages information about peers. It is currently a bare-bones
// in-memory store of peer addresses, and will be fleshed out later.
//
// The main function of peerStore is currently to dispense peers to connect to
// (via peerStore.Dispense), giving the caller exclusive "ownership" of that
// peer until the peer is returned (via peerStore.Return). This is used to
// schedule and synchronize peer dialing and accepting in the Router, e.g.
// making sure we only have a single connection (in either direction) to peers.
type peerStore struct {
	mtx     sync.Mutex
	peers   map[NodeID]*storePeer
	claimed map[NodeID]bool
}

// newPeerStore creates a new peer store.
func newPeerStore() *peerStore {
	return &peerStore{
		peers:   map[NodeID]*storePeer{},
		claimed: map[NodeID]bool{},
	}
}

// Add adds a peer to the store, given as an address.
func (s *peerStore) Add(address PeerAddress) error {
	if err := address.Validate(); err != nil {
		return err
	}
	peerID := address.NodeID()

	s.mtx.Lock()
	defer s.mtx.Unlock()

	peer, ok := s.peers[peerID]
	if !ok {
		peer = newStorePeer(peerID)
		s.peers[peerID] = peer
	} else if s.claimed[peerID] {
		// FIXME: We need to handle modifications of claimed peers somehow.
		return fmt.Errorf("peer %q is claimed", peerID)
	}
	peer.AddAddress(address)
	return nil
}

// Claim claims a peer. The caller has exclusive ownership of the peer, and must
// return it by calling Return(). Returns nil if the peer could not be claimed.
// If the peer is not known to the store, it is registered and claimed.
func (s *peerStore) Claim(id NodeID) *storePeer {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	if s.claimed[id] {
		return nil
	}
	peer, ok := s.peers[id]
	if !ok {
		peer = newStorePeer(id)
		s.peers[id] = peer
	}
	s.claimed[id] = true
	return peer
}

// Dispense finds an appropriate peer to contact and claims it. The caller has
// exclusive ownership of the peer, and must return it by calling Return(). The
// peer will not be dispensed again until returned.
//
// Returns nil if no appropriate peers are available.
func (s *peerStore) Dispense() *storePeer {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	for key, peer := range s.peers {
		switch {
		case len(peer.Addresses) == 0:
		case s.claimed[key]:
		default:
			s.claimed[key] = true
			return peer
		}
	}
	return nil
}

// Return returns a claimed peer, making it available for other
// callers to claim.
func (s *peerStore) Return(id NodeID) {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	delete(s.claimed, id)
}

// storePeer is a peer stored in the peerStore.
//
// FIXME: This should be renamed peer or something else once the old peer is
// removed.
type storePeer struct {
	ID        NodeID
	Addresses []PeerAddress
}

// newStorePeer creates a new storePeer.
func newStorePeer(id NodeID) *storePeer {
	return &storePeer{
		ID:        id,
		Addresses: []PeerAddress{},
	}
}

// AddAddress adds an address to a peer, unless it already exists. It does not
// validate the address.
func (p *storePeer) AddAddress(address PeerAddress) {
	// We just do a linear search for now.
	addressString := address.String()
	for _, a := range p.Addresses {
		if a.String() == addressString {
			return
		}
	}
	p.Addresses = append(p.Addresses, address)
}

// ============================================================================
// Types and business logic below may be deprecated.
//
// TODO: Rename once legacy p2p types are removed.
// ref: https://github.com/tendermint/tendermint/issues/5670
// ============================================================================

//go:generate mockery --case underscore --name Peer

const metricsTickerDuration = 10 * time.Second

// Peer is an interface representing a peer connected on a reactor.
type Peer interface {
	service.Service
	FlushStop()

	ID() NodeID           // peer's cryptographic ID
	RemoteIP() net.IP     // remote IP of the connection
	RemoteAddr() net.Addr // remote address of the connection

	IsOutbound() bool   // did we dial the peer
	IsPersistent() bool // do we redial this peer when we disconnect

	CloseConn() error // close original connection

	NodeInfo() NodeInfo // peer's info
	Status() tmconn.ConnectionStatus
	SocketAddr() *NetAddress // actual address of the socket

	Send(byte, []byte) bool
	TrySend(byte, []byte) bool

	Set(string, interface{})
	Get(string) interface{}
}

//----------------------------------------------------------

// peerConn contains the raw connection and its config.
type peerConn struct {
	outbound   bool
	persistent bool
	conn       Connection
	ip         net.IP // cached RemoteIP()
}

func newPeerConn(outbound, persistent bool, conn Connection) peerConn {
	return peerConn{
		outbound:   outbound,
		persistent: persistent,
		conn:       conn,
	}
}

// ID only exists for SecretConnection.
func (pc peerConn) ID() NodeID {
	return NodeIDFromPubKey(pc.conn.PubKey())
}

// Return the IP from the connection RemoteAddr
func (pc peerConn) RemoteIP() net.IP {
	if pc.ip == nil {
		pc.ip = pc.conn.RemoteEndpoint().IP
	}
	return pc.ip
}

// peer implements Peer.
//
// Before using a peer, you will need to perform a handshake on connection.
type peer struct {
	service.BaseService

	// raw peerConn and the multiplex connection
	peerConn

	// peer's node info and the channel it knows about
	// channels = nodeInfo.Channels
	// cached to avoid copying nodeInfo in hasChannel
	nodeInfo    NodeInfo
	channels    []byte
	reactors    map[byte]Reactor
	onPeerError func(Peer, interface{})

	// User data
	Data *cmap.CMap

	metrics       *Metrics
	metricsTicker *time.Ticker
}

type PeerOption func(*peer)

func newPeer(
	pc peerConn,
	reactorsByCh map[byte]Reactor,
	onPeerError func(Peer, interface{}),
	options ...PeerOption,
) *peer {
	nodeInfo := pc.conn.NodeInfo()
	p := &peer{
		peerConn:      pc,
		nodeInfo:      nodeInfo,
		channels:      nodeInfo.Channels, // TODO
		reactors:      reactorsByCh,
		onPeerError:   onPeerError,
		Data:          cmap.NewCMap(),
		metricsTicker: time.NewTicker(metricsTickerDuration),
		metrics:       NopMetrics(),
	}

	p.BaseService = *service.NewBaseService(nil, "Peer", p)
	for _, option := range options {
		option(p)
	}

	return p
}

// onError calls the peer error callback.
func (p *peer) onError(err interface{}) {
	p.onPeerError(p, err)
}

// String representation.
func (p *peer) String() string {
	if p.outbound {
		return fmt.Sprintf("Peer{%v %v out}", p.conn, p.ID())
	}

	return fmt.Sprintf("Peer{%v %v in}", p.conn, p.ID())
}

//---------------------------------------------------
// Implements service.Service

// SetLogger implements BaseService.
func (p *peer) SetLogger(l log.Logger) {
	p.Logger = l
}

// OnStart implements BaseService.
func (p *peer) OnStart() error {
	if err := p.BaseService.OnStart(); err != nil {
		return err
	}

	go p.processMessages()
	go p.metricsReporter()

	return nil
}

// processMessages processes messages received from the connection.
func (p *peer) processMessages() {
	defer func() {
		if r := recover(); r != nil {
			p.Logger.Error("peer message processing panic", "err", r, "stack", string(debug.Stack()))
			p.onError(fmt.Errorf("panic during peer message processing: %v", r))
		}
	}()

	for {
		chID, msg, err := p.conn.ReceiveMessage()
		if err != nil {
			p.onError(err)
			return
		}
		reactor, ok := p.reactors[chID]
		if !ok {
			p.onError(fmt.Errorf("unknown channel %v", chID))
			return
		}
		reactor.Receive(chID, p, msg)
	}
}

// FlushStop mimics OnStop but additionally ensures that all successful
// .Send() calls will get flushed before closing the connection.
// NOTE: it is not safe to call this method more than once.
func (p *peer) FlushStop() {
	p.metricsTicker.Stop()
	p.BaseService.OnStop()
	if err := p.conn.FlushClose(); err != nil {
		p.Logger.Debug("error while stopping peer", "err", err)
	}
}

// OnStop implements BaseService.
func (p *peer) OnStop() {
	p.metricsTicker.Stop()
	p.BaseService.OnStop()
	if err := p.conn.Close(); err != nil {
		p.Logger.Debug("error while stopping peer", "err", err)
	}
}

//---------------------------------------------------
// Implements Peer

// ID returns the peer's ID - the hex encoded hash of its pubkey.
func (p *peer) ID() NodeID {
	return p.nodeInfo.ID()
}

// IsOutbound returns true if the connection is outbound, false otherwise.
func (p *peer) IsOutbound() bool {
	return p.peerConn.outbound
}

// IsPersistent returns true if the peer is persitent, false otherwise.
func (p *peer) IsPersistent() bool {
	return p.peerConn.persistent
}

// NodeInfo returns a copy of the peer's NodeInfo.
func (p *peer) NodeInfo() NodeInfo {
	return p.nodeInfo
}

// SocketAddr returns the address of the socket.
// For outbound peers, it's the address dialed (after DNS resolution).
// For inbound peers, it's the address returned by the underlying connection
// (not what's reported in the peer's NodeInfo).
func (p *peer) SocketAddr() *NetAddress {
	return p.peerConn.conn.RemoteEndpoint().NetAddress()
}

// Status returns the peer's ConnectionStatus.
func (p *peer) Status() tmconn.ConnectionStatus {
	return p.conn.Status()
}

// Send msg bytes to the channel identified by chID byte. Returns false if the
// send queue is full after timeout, specified by MConnection.
func (p *peer) Send(chID byte, msgBytes []byte) bool {
	if !p.IsRunning() {
		// see Switch#Broadcast, where we fetch the list of peers and loop over
		// them - while we're looping, one peer may be removed and stopped.
		return false
	} else if !p.hasChannel(chID) {
		return false
	}
	res, err := p.conn.SendMessage(chID, msgBytes)
	if err == io.EOF {
		return false
	} else if err != nil {
		p.onError(err)
		return false
	}
	if res {
		labels := []string{
			"peer_id", string(p.ID()),
			"chID", fmt.Sprintf("%#x", chID),
		}
		p.metrics.PeerSendBytesTotal.With(labels...).Add(float64(len(msgBytes)))
	}
	return res
}

// TrySend msg bytes to the channel identified by chID byte. Immediately returns
// false if the send queue is full.
func (p *peer) TrySend(chID byte, msgBytes []byte) bool {
	if !p.IsRunning() {
		return false
	} else if !p.hasChannel(chID) {
		return false
	}
	res, err := p.conn.TrySendMessage(chID, msgBytes)
	if err == io.EOF {
		return false
	} else if err != nil {
		p.onError(err)
		return false
	}
	if res {
		labels := []string{
			"peer_id", string(p.ID()),
			"chID", fmt.Sprintf("%#x", chID),
		}
		p.metrics.PeerSendBytesTotal.With(labels...).Add(float64(len(msgBytes)))
	}
	return res
}

// Get the data for a given key.
func (p *peer) Get(key string) interface{} {
	return p.Data.Get(key)
}

// Set sets the data for the given key.
func (p *peer) Set(key string, data interface{}) {
	p.Data.Set(key, data)
}

// hasChannel returns true if the peer reported
// knowing about the given chID.
func (p *peer) hasChannel(chID byte) bool {
	for _, ch := range p.channels {
		if ch == chID {
			return true
		}
	}
	// NOTE: probably will want to remove this
	// but could be helpful while the feature is new
	p.Logger.Debug(
		"Unknown channel for peer",
		"channel",
		chID,
		"channels",
		p.channels,
	)
	return false
}

// CloseConn closes original connection. Used for cleaning up in cases where the peer had not been started at all.
func (p *peer) CloseConn() error {
	return p.peerConn.conn.Close()
}

//---------------------------------------------------
// methods only used for testing
// TODO: can we remove these?

// CloseConn closes the underlying connection
func (pc *peerConn) CloseConn() {
	pc.conn.Close()
}

// RemoteAddr returns peer's remote network address.
func (p *peer) RemoteAddr() net.Addr {
	endpoint := p.conn.RemoteEndpoint()
	return &net.TCPAddr{
		IP:   endpoint.IP,
		Port: int(endpoint.Port),
	}
}

//---------------------------------------------------

func PeerMetrics(metrics *Metrics) PeerOption {
	return func(p *peer) {
		p.metrics = metrics
	}
}

func (p *peer) metricsReporter() {
	for {
		select {
		case <-p.metricsTicker.C:
			status := p.conn.Status()
			var sendQueueSize float64
			for _, chStatus := range status.Channels {
				sendQueueSize += float64(chStatus.SendQueueSize)
			}

			p.metrics.PeerPendingSendBytes.With("peer_id", string(p.ID())).Set(sendQueueSize)
		case <-p.Quit():
			return
		}
	}
}
