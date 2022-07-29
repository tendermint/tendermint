# List of source files

Package [`p2p`](https://github.com/tendermint/tendermint/tree/v0.34.x/p2p) of branch `v0.34.x`.

State of July 2022.

Implementation of the peer-to-peer layer of Tendermint.

If follows a list of Go source files and their contents.

## `base_reactor.go`

`Reactor` interface.

`BaseReactor` implements `Reactor`.

## `conn_set.go`

`ConnSet` interface, a "lookup table for connections and their ips".
Internal type `connSet` implements the `ConnSet` interface.

## `errors.go`

`ErrRejected` struct carries reasons for which a peer was rejected.

Other errors.

## `fuzz.go`

`FuzzedConnection` wraps a `net.Conn` and injects random delays.
For testing purposes.

## `key.go`

`NodeKey` is the "persistent peer key", wraps a `crypto.PrivKey`.

## `metrics.go`

Prometheus `Metrics` exposed by the package.

## `netaddress.go`

Type `NetAddress` contains peer's ID (a string), IP, and port.

## `node_info.go`

Interface `NodeInfo`, implemented by `DefaultNodeInfo`,
is the "basic node information exchanged" with a peer during the handshake.

## `peer.go`

Interface `Peer`, implemented by non-exported `peer` struct,
represents a connected peer.

## `peer_set.go`

Type `PeerSet` implements a thread-safe table of peers.

Interface `IPeerSet` declares a subset of methods provided by `PeerSet`.
Type `PeerSet` implements the `IPeerSet` interface.

## `switch.go`

Documented in [switch.md](./switch.md).

## `transport.go`

TODO:

## `types.go`

Aliases for p2p's `conn` package types.

## Package `conn`

## Package `mock`

## Package `mocks`

## Package `pex

## Package `trust`

## Package `upnp`
