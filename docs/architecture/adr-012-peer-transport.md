# ADR 012: PeerTransport

## Context

One of the more apparent problems with the current architecture in the p2p
package is that there is no clear separation of concerns between different
components. Most notably the `Switch` is currently doing physical connection
handling. An artifact is the dependency of the Switch on `config.P2PConfig`.

Addresses:
* [#2046](https://github.com/tendermint/tendermint/issues/2046)
* [#2047](https://github.com/tendermint/tendermint/issues/2047)

First iteraton in [#2067](https://github.com/tendermint/tendermint/issues/2067)

## Decision

### PeerTransport

Responsible for emitting and connecting to Peers. The implementation of `Peer`
is left to the transport, which implies that the chosen transport dictates the
characteristics of the implementation handed back to the `Switch`. It is the
place where we enforce low-level guards, like dropping connections from our own
node.

``` go
// PeerTransport proxies incoming and outgoing peer connections.
type PeerTransport interface {
	// Accept returns a newly connected Peer.
	Accept() (Peer, error)

	// Dial connects to a Peer.
	Dial(NetAddress) (Peer, error)
}

// EXAMPLE OF DEFAULT IMPLEMENTATION

// multiplexTransport accepts tcp connections and upgrades to multiplexted
// peers.
type multiplexTransport struct {
	listener net.Listener

	acceptc chan accept
	closec  <-chan struct{}
	listenc <-chan struct{}

	addr             NetAddress
	dialTimeout      time.Duration
	handshakeTimeout time.Duration
	nodeInfo         NodeInfo
	nodeKey          NodeKey

	// TODO(xla): Remove when MConnection is refactored into mPeer.
	mConfig conn.MConnConfig
}

var _ PeerTransport = (*multiplexTransport)(nil)

// NewMTransport returns network connected multiplexed peers.
func NewMTransport(
	nodeAddr NetAddress,
	nodeInfo NodeInfo,
	nodeKey NodeKey,
) *multiplexTransport 
```

### Switch

From now the Switch will depend on a fully setup `PeerTransport` to
retrieve/reach out to its peers. As the more low-level concerns are pushed to
the transport, we can omit passing the `config.P2PConfig` to the Switch.

``` go
func NewSwitch(transport PeerTransport, opts ...SwitchOption) *Switch
```

## Status

In Review.

## Consequences

### Positive

* free Switch from transport concerns
* remove Switch dependency on P2PConfig
* pluggable Peer implementation
* dependency injection during tests

### Negative

* more setup for tests which depend on Switches

### Neutral

* multiplexed will be the default implementation
