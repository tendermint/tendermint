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


#### PeerTransport

Responsible for the emitting and connecting to Peers. The implementation of
`Peer` is left to the transport, which implies that the chosen transport
dictates the characteristics of the implementation handed back to the `Switch`.


``` go
// PeerTransport proxies incoming and outgoing peer connections.
type PeerTransport interface {
	fmt.Stringer

	// Accept returns a newly connected Peer.
	Accept(peerConfig) (Peer, error)

	// Dial connects to a Peer.
	Dial(NetAddress, peerConfig) (Peer, error)

	// ExternalAddress is the configured address to advertise.
	// TODO(xla): Is currently expected from the old Listener interface and
	// shouldn't be part of the transport as it increases the surfaces and can be
	// handled differently on the caller side.
	ExternalAddress() NetAddress

	// lifecycle methods
	Close() error
	Listen(NetAddress) error
}
```

#### Switch

**TODO**



## Status

Proposed.

## Consequences

### Positive

### Negative

### Neutral
