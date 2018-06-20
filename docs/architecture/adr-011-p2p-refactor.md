# ADR 011: P2P Refactor

## Context

In its current implementation the separation of concerns betweeen different
components (AddrBook, Peer, PEX, Switch) is not solid, for reference see
#1325, #1356. This comes at no surprise as this is one of the oldest packages
in the codebase with almost 4 years of history. Over time different
requirements have been tried to fit in and the scope of the package expanded,
especially given its center role in the entire codebase. Additionally, a range
of potential security issues have been surfaced through external audits which
could be addressed with structural changes and better separation.

## Decision

### Architecture

This section lays out the proposed architecture for the components in the `p2p`
package.

### Types

This section expands on the proposed types (changed and new) with their
respective APIs.

#### Switch

``` go
```

#### PeerBehaviour

``` go
// PeerBehaviour is the interface that can be expected by external callers who
// want to inform about Peer actions which should result in trust changes.
// Currently those are the vetting of a Peer and informing about bad behaviour.
type PeerBehaviour interface {
	// Errored informs about any kind of Peer misbehaviour. It expects one of the
	// ErrPeerBehaviour errors to properly assess the severity and in some cases
	// disconnect or ban the Peer entirely.
	Errored(*Peer, error)

	// Vetted should be called if the Peer has shown to be reliable.
	Vetted(*Peer)
}
```

#### PeerTransport

Responsible for emitting and connecting to Peers. The implementation of `Peer`
is left to the transport, which implies that the chosen transport dictates the
characteristics of the implementation handed back to the `Switch`. It is the
place where we enforce low-level guards, like dropping connections from our own
node.

``` go
// PeerTransport proxies incoming and outgoing peer connections.
type PeerTransport interface {
	// Accept returns a newly connected Peer.
	Accept(peerConfig) (Peer, error)

	// Dial connects to a Peer.
	Dial(NetAddress, peerConfig) (Peer, error)
}
```

#### PEX

#### AddrBook

**TODO**



## Status

Proposed.

## Consequences

### Positive

### Negative

### Neutral
