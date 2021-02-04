# ADR 062: P2P Architecture and Abstractions

## Changelog

- 2020-11-09: Initial version (@erikgrinaker)

- 2020-11-13: Remove stream IDs, move peer errors onto channel, note on moving PEX into core (@erikgrinaker)

- 2020-11-16: Notes on recommended reactor implementation patterns, approve ADR (@erikgrinaker)

## Context

In [ADR 061](adr-061-p2p-refactor-scope.md) we decided to refactor the peer-to-peer (P2P) networking stack. The first phase is to redesign and refactor the internal P2P architecture, while retaining protocol compatibility as far as possible.

## Alternative Approaches

Several variations of the proposed design were considered, including e.g. calling interface methods instead of passing messages (like the current architecture), merging channels with streams, exposing the internal peer data structure to reactors, being message format-agnostic via arbitrary codecs, and so on. This design was chosen because it has very loose coupling, is simpler to reason about and more convenient to use, avoids race conditions and lock contention for internal data structures, gives reactors better control of message ordering and processing semantics, and allows for QoS scheduling and backpressure in a very natural way.

[multiaddr](https://github.com/multiformats/multiaddr) was considered as a transport-agnostic peer address format over regular URLs, but it does not appear to have very widespread adoption, and advanced features like protocol encapsulation and tunneling do not appear to be immediately useful to us.

There were also proposals to use LibP2P instead of maintaining our own P2P stack, which were rejected (for now) in [ADR 061](adr-061-p2p-refactor-scope.md).

The initial version of this ADR had a byte-oriented multi-stream transport API, but this had to be abandoned/postponed to maintain backwards-compatibility with the existing MConnection protocol which is message-oriented. See the rejected RFC in [tendermint/spec#227](https://github.com/tendermint/spec/pull/227) for details.

## Decision

The P2P stack will be redesigned as a message-oriented architecture, primarily relying on Go channels for communication and scheduling. It will use IO stream transports to exchange raw bytes with individual peers, bidirectional peer-addressable channels to send and receive Protobuf messages, and a router to route messages between reactors and peers. Message passing is asynchronous with at-most-once delivery.

## Detailed Design

This ADR is primarily concerned with the architecture and interfaces of the P2P stack, not implementation details. Separate ADRs may be submitted for individual components, since implementation may be non-trivial. The interfaces described here should therefore be considered a rough architecture outline, not a complete and final design.

Primary design objectives have been:

* Loose coupling between components, for a simpler, more robust, and test-friendly architecture.
* Pluggable transports (not necessarily networked).
* Better scheduling of messages, with improved prioritization, backpressure, and performance.
* Centralized peer lifecycle and connection management.
* Better peer address detection, advertisement, and exchange.
* Wire-level backwards compatibility with current P2P network protocols, except where it proves too obstructive.

The main abstractions in the new stack are:

* `Transport`: An arbitrary mechanism to exchange binary messages with a peer across a `Connection`.
* `Channel`: A bidirectional channel to asynchronously exchange Protobuf messages with peers using node ID addressing.
* `Router`: Maintains transport connections to relevant peers and routes channel messages.
* `PeerManager`: Manages peer lifecycle information, e.g. deciding which peers to dial and when, using a `peerStore` for storage.
* Reactor: A design pattern loosely defined as "something which listens on a channel and reacts to messages".

These abstractions are illustrated in the following diagram (representing the internals of node A) and described in detail below.

![P2P Architecture Diagram](img/adr-062-architecture.svg)

### Transports

Transports are arbitrary mechanisms for exchanging binary messages with a peer. For example, a gRPC transport would connect to a peer over TCP/IP and send data using the gRPC protocol, while an in-memory transport might communicate with a peer running in another goroutine using internal Go channels. Note that transports don't have a notion of a "peer" or "node" as such - instead, they establish connections between arbitrary endpoint addresses (e.g. IP address and port number), to decouple them from the rest of the P2P stack.

Transports must satisfy the following requirements:

* Be connection-oriented, and support both listening for inbound connections and making outbound connections using endpoint addresses.

* Support sending binary messages with distinct channel IDs (although channels and channel IDs are a higher-level application protocol concept explained in the Router section, they are threaded through the transport layer as well for backwards compatibilty with the existing MConnection protocol).

* Exchange the MConnection `NodeInfo` and public key via a node handshake, and possibly encrypt or sign the traffic as appropriate.

The initial transport is a port of the current MConnection protocol currently used by Tendermint, and should be backwards-compatible at the wire level, as wel as an in-memory transport used for testing. There are plans to explore a QUIC transport that may replace MConnection.

The `Transport` interface is as follows:

```go
// Transport is a connection-oriented mechanism for exchanging data with a peer.
type Transport interface {
	// Protocols returns the protocols supported by the transport. The Router
	// uses this to pick a transport for an Endpoint.
	Protocols() []Protocol

	// Endpoints returns the local endpoints the transport is listening on, if any.
	// How to listen is transport-dependent, e.g. MConnTransport uses Listen() while
	// MemoryTransport starts listening via MemoryNetwork.CreateTransport().
	Endpoints() []Endpoint

	// Accept waits for the next inbound connection on a listening endpoint, blocking
	// until either a connection is available or the transport is closed. On closure,
	// io.EOF is returned and further Accept calls are futile.
	Accept() (Connection, error)

	// Dial creates an outbound connection to an endpoint.
	Dial(context.Context, Endpoint) (Connection, error)

	// Close stops accepting new connections, but does not close active connections.
	Close() error
}
```

How the transport configures listening is transport-dependent, and not covered by the interface. This typically happens during transport construction, where a single instance of the transport is created and set to listen on an appropriate network interface before being passed to the router.

#### Endpoints

`Endpoint` represents a transport endpoint (e.g. an IP address and port). A connection always has two endpoints: one at the local node and one at the remote peer. Outbound connections to remote endpoints are made via `Dial()`, and inbound connections to listening endpoints are returned via `Accept()`.

The `Endpoint` struct is:

```go
// Endpoint represents a transport connection endpoint, either local or remote.
//
// Endpoints are not necessarily networked (see e.g. MemoryTransport) but all
// networked endpoints must use IP as the underlying transport protocol to allow
// e.g. IP address filtering. Either IP or Path (or both) must be set.
type Endpoint struct {
	// Protocol specifies the transport protocol.
	Protocol Protocol

	// IP is an IP address (v4 or v6) to connect to. If set, this defines the
	// endpoint as a networked endpoint.
	IP net.IP

	// Port is a network port (either TCP or UDP). If 0, a default port may be
	// used depending on the protocol.
	Port uint16

	// Path is an optional transport-specific path or identifier.
	Path string
}

// Protocol identifies a transport protocol.
type Protocol string
```

Endpoints are arbitrary transport-specific addresses, but if they are networked they must use IP addresses and thus rely on IP as a fundamental packet routing protocol. This enables policies for address discovery, advertisement, and exchange - for example, a private `192.168.0.0/24` IP address should only be advertised to peers on that IP network, while the public address `8.8.8.8` may be advertised to all peers. Similarly, any port numbers if given must represent TCP and/or UDP port numbers, in order to use [UPnP](https://en.wikipedia.org/wiki/Universal_Plug_and_Play) to autoconfigure e.g. NAT gateways.

Non-networked endpoints (without an IP address) are considered local, and will only be advertised to other peers connecting via the same protocol. For example, the in-memory transport used for testing uses `Endpoint{Protocol: "memory", Path: "foo"}` as an address for the node "foo", and this should only be advertised to other nodes using `Protocol: "memory"`.

#### Connections

A connection represents an established transport connection between two endpoints (i.e. two nodes), which can be used to exchange binary messages with logical channel IDs (corresponding to the higher-level channel IDs used in the router). Connections are set up either via `Transport.Dial()` (outbound) or `Transport.Accept()` (inbound).

Once a connection is esablished, `Transport.Handshake()` must be called to perform a node handshake, exchanging node info and public keys to verify node identities. Node handshakes should not really be part of the transport protocol (rather it should be part of the application protocol), this exists for backwards-compatibility with the existing MConnection protocol which conflates the two, and allows the router to do common `NodeInfo` verification across transports as well as varying e.g. `NodeInfo` contents by peer (e.g. to advertise different listen addresses to different peers).

This ADR initially proposed a byte-oriented multi-stream connection API that follows more typical networking API conventions (using e.g. `io.Reader` and `io.Writer` interfaces which easily compose with other libraries). This would also allow moving the responsibility for message framing, node handshakes, and traffic scheduling to the common router instead of reimplementing this across transports, and would allow making better use of multi-stream protocols such as QUIC. However, this would require minor breaking changes to the MConnection protocol which were rejected, see [tendermint/spec#227](https://github.com/tendermint/spec/pull/227) for details. This should be revisited when starting work on a QUIC transport.

The `Connection` interface is shown below. It omits certain additions that are currently implemented for backwards compatibility with the legacy P2P stack and are planned to be removed before the final release.

```go
// Connection represents an established connection between two endpoints.
type Connection interface {
	// Handshake executes a node handshake with the remote peer. It must be
	// called immediately after the connection is established, and returns the
	// remote peer's node info and public key. The caller is responsible for
	// validation.
	Handshake(context.Context, NodeInfo, crypto.PrivKey) (NodeInfo, crypto.PubKey, error)

	// ReceiveMessage returns the next message received on the connection,
	// blocking until one is available. Returns io.EOF if closed.
	ReceiveMessage() (ChannelID, []byte, error)

	// SendMessage sends a message on the connection. Returns io.EOF if closed.
	SendMessage(ChannelID, []byte) error

	// LocalEndpoint returns the local endpoint for the connection.
	LocalEndpoint() Endpoint

	// RemoteEndpoint returns the remote endpoint for the connection.
	RemoteEndpoint() Endpoint

	// Close closes the connection.
	Close() error
}
```

### Peers

Peers are other Tendermint network nodes. Each peer is identified by a unique `PeerID`, and has a set of `PeerAddress` addresses expressed as URLs that they can be reached at. Examples of peer addresses might be e.g.:

* `mconn://b10c@host.domain.com:25567/path`
* `unix:///var/run/tendermint/peer.sock`
* `memory:testpeer`

Addresses are resolved into one or more transport endpoints, e.g. by resolving DNS hostnames into IP addresses (which should be refreshed periodically). Peers should always be expressed as address URLs, and never as endpoints which are a lower-level construct.

```go
// PeerID is a unique peer ID, generally expressed in hex form.
type PeerID []byte

// PeerAddress is a peer address URL. The User field, if set, gives the
// hex-encoded remote PeerID, which should be verified with the remote peer's
// public key as returned by the connection.
type PeerAddress url.URL

// Resolve resolves a PeerAddress into a set of Endpoints, typically by
// expanding out a DNS name in Host to its IP addresses. Field mapping:
//
//   Scheme → Endpoint.Protocol
//   Host   → Endpoint.IP
//   Port   → Endpoint.Port
//   Path+Query+Fragment,Opaque → Endpoint.Path
//
func (a PeerAddress) Resolve(ctx context.Context) []Endpoint { return nil }
```

The P2P stack needs to track a lot of internal information about peers, such as endpoints, status, priorities, and so on. This is done in an internal `peer` struct, which should not be exposed outside of the `p2p` package (e.g. to reactors) in order to avoid race conditions and lock contention - other packages should use `PeerID`.

The `peer` struct might look like the following, but is intentionally underspecified and will depend on implementation requirements (for example, it will almost certainly have to track statistics about connection failures and retries):

```go
// peer tracks internal status information about a peer.
type peer struct {
    ID        PeerID
    Status    PeerStatus
    Priority  PeerPriority
    Endpoints map[PeerAddress][]Endpoint // Resolved endpoints by address.
}

// PeerStatus specifies peer statuses.
type PeerStatus string

const (
    PeerStatusNew     = "new"     // New peer which we haven't tried to contact yet.
    PeerStatusUp      = "up"      // Peer which we have an active connection to.
    PeerStatusDown    = "down"    // Peer which we're temporarily disconnected from.
    PeerStatusRemoved = "removed" // Peer which has been removed.
    PeerStatusBanned  = "banned"  // Peer which is banned for misbehavior.
)

// PeerPriority specifies peer priorities.
type PeerPriority int

const (
    PeerPriorityNormal PeerPriority = iota + 1
    PeerPriorityValidator
    PeerPriorityPersistent
)
```

Peer information is stored in a `peerStore`, which may be persisted in an underlying database, and will replace the current address book either partially or in full. It is kept internal to avoid race conditions and tight coupling, and should at the very least contain basic CRUD functionality as outlined below, but will likely need additional functionality and is intentionally underspecified:

```go
// peerStore contains information about peers, possibly persisted to disk.
type peerStore struct {
    peers map[string]*peer // Entire set in memory, with PeerID.String() keys.
    db    dbm.DB           // Database for persistence, if non-nil.
}

func (p *peerStore) Delete(id PeerID) error     { return nil }
func (p *peerStore) Get(id PeerID) (peer, bool) { return peer{}, false }
func (p *peerStore) List() []peer               { return nil }
func (p *peerStore) Set(peer peer) error        { return nil }
```

Peer address detection, advertisement and exchange (including detection of externally-reachable addresses via e.g. NAT gateways) is out of scope for this ADR, but may be covered in a separate ADR. The current PEX reactor should probably be absorbed into the core P2P stack and protocol instead of running as a separate reactor, since this needs to mutate the core peer data structures and will thus be tightly coupled with the router.

### Channels

While low-level data exchange happens via transport IO streams, the high-level API is based on a bidirectional `Channel` that can send and receive Protobuf messages addressed by `PeerID`. A channel is identified by an arbitrary `ChannelID` identifier, and can exchange Protobuf messages of one specific type (since the type to unmarshal into must be known). Message delivery is asynchronous and at-most-once.

The channel can also be used to report peer errors, e.g. when receiving an invalid or malignant message. This may cause the peer to be disconnected or banned depending on the router's policy.

A `Channel` has this interface:

```go
// Channel is a bidirectional channel for Protobuf message exchange with peers.
type Channel struct {
    // ID contains the channel ID.
    ID ChannelID

    // messageType specifies the type of messages exchanged via the channel, and
    // is used e.g. for automatic unmarshaling.
    messageType proto.Message

    // In is a channel for receiving inbound messages. Envelope.From is always
    // set.
    In <-chan Envelope

    // Out is a channel for sending outbound messages. Envelope.To or Broadcast
    // must be set, otherwise the message is discarded.
    Out chan<- Envelope

    // Error is a channel for reporting peer errors to the router, typically used
    // when peers send an invalid or malignant message.
    Error chan<- PeerError
}

// Close closes the channel, and is equivalent to close(Channel.Out). This will
// cause Channel.In to be closed when appropriate. The ID can then be reused.
func (c *Channel) Close() error { return nil }

// ChannelID is an arbitrary channel ID.
type ChannelID uint16

// Envelope specifies the message receiver and sender.
type Envelope struct {
    From      PeerID        // Message sender, or empty for outbound messages.
    To        PeerID        // Message receiver, or empty for inbound messages.
    Broadcast bool          // Send message to all connected peers, ignoring To.
    Message   proto.Message // Payload.
}

// PeerError is a peer error reported by a reactor via the Error channel. The
// severity may cause the peer to be disconnected or banned depending on policy.
type PeerError struct {
    PeerID   PeerID
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
```

A channel can reach any connected peer, and is implemented using transport streams against each individual peer, with an initial handshake to exchange the channel ID and any other metadata. The channel will automatically (un)marshal Protobuf to byte slices and use length-prefixed framing (the de facto standard for Protobuf streams) when writing them to the stream.

Message scheduling and queueing is left as an implementation detail, and can use any number of algorithms such as FIFO, round-robin, priority queues, etc. Since message delivery is not guaranteed, both inbound and outbound messages may be dropped, buffered, or blocked as appropriate.

Since a channel can only exchange messages of a single type, it is often useful to use a wrapper message type with e.g. a Protobuf `oneof` field that specifies a set of inner message types that it can contain. The channel can automatically perform this (un)wrapping if the outer message type implements the `Wrapper` interface (see [Reactor Example](#reactor-example) for an example):

```go
// Wrapper is a Protobuf message that can contain a variety of inner messages.
// If a Channel's message type implements Wrapper, the channel will
// automatically (un)wrap passed messages using the container type, such that
// the channel can transparently support multiple message types.
type Wrapper interface {
    // Wrap will take a message and wrap it in this one.
    Wrap(proto.Message) error

    // Unwrap will unwrap the inner message contained in this message.
    Unwrap() (proto.Message, error)
}
```

### Routers

The router manages all P2P networking for a node, and is responsible for keeping track of network peers, maintaining transport connections, and routing channel messages. As such, it must do e.g. connection retries and backoff, message QoS scheduling and backpressure, peer quality assessments, and endpoint detection and advertisement. In addition, the router provides a mechanism to subscribe to peer updates (e.g. peers connecting or disconnecting), and handles reported peer errors from reactors.

The implementation of the router is likely to be non-trivial, and is intentionally unspecified here. A separate ADR will likely be submitted for this. It is unclear whether message routing/scheduling and peer lifecycle management can be split into two separate components, or if these need to be tightly coupled.

The `Router` API is as follows:

```go
// Router manages connections to peers and routes Protobuf messages between them
// and local reactors. It also provides peer status updates and error reporting.
type Router struct{}

// NewRouter creates a new router, using the given peer store to track peers.
// Transports must be pre-initialized to listen on appropriate endpoints.
func NewRouter(peerStore *peerStore, transports map[Protocol]Transport) *Router { return nil }

// Channel opens a new channel with the given ID. messageType should be an empty
// Protobuf message of the type that will be passed through the channel. The
// message can implement Wrapper for automatic message (un)wrapping.
func (r *Router) Channel(id ChannelID, messageType proto.Message) (*Channel, error) { return nil, nil }

// PeerUpdates returns a channel with peer updates. The caller must cancel the
// context to end the subscription, and keep consuming messages in a timely
// fashion until the channel is closed to avoid blocking updates.
func (r *Router) PeerUpdates(ctx context.Context) PeerUpdates { return nil }

// PeerUpdates is a channel for receiving peer updates.
type PeerUpdates <-chan PeerUpdate

// PeerUpdate is a peer status update for reactors.
type PeerUpdate struct {
    PeerID PeerID
    Status PeerStatus
}
```

### Reactor Example

While reactors are a first-class concept in the current P2P stack (i.e. there is an explicit `p2p.Reactor` interface), they will simply be a design pattern in the new stack, loosely defined as "something which listens on a channel and reacts to messages".

Since reactors have very few formal constraints, they can be implemented in a variety of ways. There is currently no recommended pattern for implementing reactors, to avoid overspecification and scope creep in this ADR. However, prototyping and developing a reactor pattern should be done early during implementation, to make sure reactors built using the `Channel` interface can satisfy the needs for convenience, deterministic tests, and reliability.

Below is a trivial example of a simple echo reactor implemented as a function. The reactor will exchange the following Protobuf messages:

```protobuf
message EchoMessage {
    oneof inner {
        PingMessage ping = 1;
        PongMessage pong = 2;
    }
}

message PingMessage {
    string content = 1;
}

message PongMessage {
    string content = 1;
}
```

Implementing the `Wrapper` interface for `EchoMessage` allows transparently passing `PingMessage` and `PongMessage` through the channel, where it will automatically be (un)wrapped in an `EchoMessage`:

```go
func (m *EchoMessage) Wrap(inner proto.Message) error {
    switch inner := inner.(type) {
    case *PingMessage:
        m.Inner = &EchoMessage_PingMessage{Ping: inner}
    case *PongMessage:
        m.Inner = &EchoMessage_PongMessage{Pong: inner}
    default:
        return fmt.Errorf("unknown message %T", inner)
    }
    return nil
}

func (m *EchoMessage) Unwrap() (proto.Message, error) {
    switch inner := m.Inner.(type) {
    case *EchoMessage_PingMessage:
        return inner.Ping, nil
    case *EchoMessage_PongMessage:
        return inner.Pong, nil
    default:
        return nil, fmt.Errorf("unknown message %T", inner)
    }
}
```

The reactor itself would be implemented e.g. like this:

```go
// RunEchoReactor wires up an echo reactor to a router and runs it.
func RunEchoReactor(router *p2p.Router) error {
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    channel, err := router.Channel(1, &EchoMessage{})
    if err != nil {
        return err
    }
    defer channel.Close()

    return EchoReactor(ctx, channel, router.PeerUpdates(ctx))
}

// EchoReactor provides an echo service, pinging all known peers until cancelled.
func EchoReactor(ctx context.Context, channel *p2p.Channel, peerUpdates p2p.PeerUpdates) error {
    ticker := time.NewTicker(5 * time.Second)
    defer ticker.Stop()

    for {
        select {
        // Send ping message to all known peers every 5 seconds.
        case <-ticker.C:
            channel.Out <- Envelope{
                Broadcast: true,
                Message:   &PingMessage{Content: "👋"},
            }

        // When we receive a message from a peer, either respond to ping, output
        // pong, or report peer error on unknown message type.
        case envelope := <-channel.In:
            switch msg := envelope.Message.(type) {
            case *PingMessage:
                channel.Out <- Envelope{
                    To:      envelope.From,
                    Message: &PongMessage{Content: msg.Content},
                }

            case *PongMessage:
                fmt.Printf("%q replied with %q\n", envelope.From, msg.Content)

            default:
                channel.Error <- PeerError{
                    PeerID:   envelope.From,
                    Err:      fmt.Errorf("unexpected message %T", msg),
                    Severity: PeerErrorSeverityLow,
                }
            }

        // Output info about any peer status changes.
        case peerUpdate := <-peerUpdates:
            fmt.Printf("Peer %q changed status to %q", peerUpdate.PeerID, peerUpdate.Status)

        // Exit when context is cancelled.
        case <-ctx.Done():
            return nil
        }
    }
}
```

### Implementation Plan

The existing P2P stack should be gradually migrated towards this design. The easiest path would likely be:

1. Implement the `Channel` and `PeerUpdates` APIs as shims on top of the current `Switch` and `Peer` APIs, and rewrite all reactors to use them instead.

2. Port the `privval` package to no longer use `SecretConnection` (e.g. by using gRPC instead), or temporarily duplicate its functionality.

3. Rewrite the current MConn connection and transport code to use the new `Transport` API, and migrate existing code to use it instead.

4. Implement the new `peer` and `peerStore` APIs, and either make the current address book a shim on top of these or replace it.

5. Replace the existing `Switch` abstraction with the new `Router`.

6. Move the PEX reactor and other address advertisement/exchange into the P2P core, possibly the `Router`.

7. Consider rewriting and/or cleaning up reactors and other P2P-related code to make better use of the new abstractions.

A note on backwards-compatibility: the current MConn protocol takes whole messages expressed as byte slices and splits them up into `PacketMsg` messages, where the final packet of a message has `PacketMsg.EOF` set. In order to maintain wire-compatibility with this protocol, the MConn transport needs to be aware of message boundaries, even though it does not care what the messages actually are. One way to handle this is to break abstraction boundaries and have the transport decode the input's length-prefixed message framing and use this to determine message boundaries, unless we accept breaking the protocol here.

Similarly, implementing channel handshakes with the current MConn protocol would require doing an initial connection handshake as today and use that information to "fake" the local channel handshake without it hitting the wire.

## Status

Accepted

## Consequences

### Positive

* Reduced coupling and simplified interfaces should lead to better understandability, increased reliability, and more testing.

* Using message passing via Go channels gives better control of backpressure and quality-of-service scheduling.

* Peer lifecycle and connection management is centralized in a single entity, making it easier to reason about.

* Detection, advertisement, and exchange of node addresses will be improved.

* Additional transports (e.g. QUIC) can be implemented and used in parallel with the existing MConn protocol.

* The P2P protocol will not be broken in the initial version, if possible.

### Negative

* Fully implementing the new design as indended is likely to require breaking changes to the P2P protocol at some point, although the initial implementation shouldn't.

* Gradually migrating the existing stack and maintaining backwards-compatibility will be more labor-intensive than simply replacing the entire stack.

* A complete overhaul of P2P internals is likely to cause temporary performance regressions and bugs as the implementation matures.

* Hiding peer management information inside the `p2p` package may prevent certain functionality or require additional deliberate interfaces for information exchange, as a tradeoff to simplify the design, reduce coupling, and avoid race conditions and lock contention.

### Neutral

* Implementation details around e.g. peer management, message scheduling, and peer and endpoint advertisement are not yet determined.

## References

* [ADR 061: P2P Refactor Scope](adr-061-p2p-refactor-scope.md)
* [#5670 p2p: internal refactor and architecture redesign](https://github.com/tendermint/tendermint/issues/5670)
