# RFC 002: Interprocess Communication (IPC) in Tendermint

## Changelog

- 08-Sep-2021: Initial draft (@creachadair).


## Abstract

Communication in Tendermint among consensus nodes, applications, and operator
tools all use different message formats and transport mechanisms.  In some
cases there are multiple options. Having all these options complicates both the
code and the developer experience, and hides bugs. To support a more robust,
trustworthy, and usable system, we should document which communication paths
are essential, which could be removed or reduced in scope, and what we can
improve for the most important use cases.


## Background

The Tendermint state replication engine has a complex IPC footprint.

1. Consensus nodes communicate with each other using a networked peer-to-peer
   message-passing protocol.

2. Consensus nodes communicate with the application whose state is being
   replicated via the [Application BlockChain Interface (ABCI)][abci].

3. Consensus nodes export a network-accessible [RPC service][rpc-service] to
   support operations (bootstrapping, debugging) and synchronization of [light clients][light-client].
   This interface is also used by the [`tendermint` CLI][tm-cli].

4. Consensus nodes export a gRPC service exposing a subset of the methods of
   the RPC service described by (3). This was intended to simplify the
   implementation of tools that already use gRPC to communicate with an
   application (via the Cosmos SDK), and wanted to also talk to the consensus
   node without implementing yet another RPC protocol.

   The gRPC interface to the consensus node has been deprecated and is slated
   for removal in the forthcoming Tendermint v0.36 release.


## Discussion

TODO: What about the verifier/signer? I think this is maybe baked into the
consensus node.  Can/does it run separately?  If so, how does it communicate
with the consensus node?  Does an application ever talk directly to the
verifier/signer?

### ABCI Transport

In ABCI, the _application_ is the server, and the Tendermint consensus engine
is the client.  Most applications implement the server using the [Cosmos SDK][cosmos-sdk],
which handles low-level details of the ABCI interaction and provides a
higher-level interface to the rest of the application. The SDK is written in Go.

Beneath the SDK, the application communicates with Tendermint core in one of
two ways:

- In-process direct calls (for applications written in Go and compiled against
  the Tendermint code).  This is an optimization for the common case where an
  application is written in Go, to save on the overhead of marshaling and
  unmarshaling requests and responses within the same process:
  [`abci/client/local_client.go`][local-client]

- A custom remote procedure protocol built on wire-format protobuf messages
  using a socket (the "socket protocol"): [`abci/server/socket_server.go`][socket-server]

### RPC Transport

The consensus node RPC service allows callers to query consensus parameters
(genesis data, transactions, commits), node status (network info, health
checks), application state (abci_query, abci_info), mempool state, and other
attributes of the node and its application. The service also provides methods
allowing transactions and evidence to be injected ("broadcast") into the
blockchain.

The RPC service is exposed in several ways:

- HTTP GET: Queries may be sent as URI parameters, with method names in the path.

- HTTP POST: Queries may be sent as JSON-RPC request messages in the body of an
  HTTP POST request.  The server uses a custom implementation of JSON-RPC that
  is not fully compatible with the [JSON-RPC 2.0 spec][json-rpc], but handles
  the common cases.

- Websocket: Queries may be sent as JSON-RPC request messages via a websocket.
  This transport uses more or less the same JSON-RPC plumbing as the HTTP POST
  handler.

  TODO: There appear to be some methods that are _only_ exported via websocket.
  Figure out what is going on there and why it's that way. I think these may be
  here for the same reason as the gRPC subset, viz., to support external
  clients who could now talk directly to the application.

- gRPC: A subset of queries may be issued in protocol buffer format to the gRPC
  interface described above under (4). As noted, this endpoint is deprecated
  and will be removed in v0.36.

### Opportunities for Simplification

**Claim:** There are too many IPC mechanisms.

The preponderance of ABCI usage is via the Cosmos SDK, which means the
application and the consensus node are compiled together into a single binary,
and the consensus node calls the ABCI methods of the application directly as Go
functions.

TODO: What are the remaining use cases for the socket protocol and gRPC? Ethan
mentioned some cases like web-based explorer UIs and possibly other tools.

As far as I can tell the primary consumers of the multi-headed "RPC service"
are the light client and the `tendermint` command-line client. There is
probably some local use via curl, but I expect that is mostly ad hoc. Ethan
reports that nodes are often configured with the ports to the RPC service
blocked, which is good for security but complicates use by the light client.

TODO: Figure out what is going on with the weird pseudo-inheritance service
interface.  The BaseService type makes sense, but its embeds are more
complicated.

### Context: ABCI Issues

In the original design of ABCI, the presumption was that all access to the
application would be mediated by the consensus node. This makes sense, since
outside access could change application state and corrupt the consensus
process. Of course, even without outside access an application could behave
nondeterministically, but allowing other programs to send it requests was seen
as inviting trouble.

Later, however, it was noted that most of the time, tools specific to a
particular application don't really want to talk to the consensus module
directly. The application is the "owner" of the state machine the consensus
engine is replicating, so tools that want to know things about application
state should talk to the application. Otherwise, such tools would have to bake
in knowledge about Tendermint (e.g., its interfaces and data structures) that
they might otherwise not need or care about.

However, this raises another issue: The consensus node is the ABCI client, so
it is inconvenient for the application to "push" work into the consensus
module. In theory you could work around this by having the consensus module
"poll" the application for work that needs done, but that has unsatisfactory
implications for performance and robustness, as well as being harder to
understand.

There has apparently been some discussion about trying to make a more
bidirectional communication between the consensus node and the application, but
this issue seems to still be under discussion. TODO: Impact of ABCI++ for this
question?

### Context: RPC Issues

The RPC system serves several masters, and has a complex surface area. I
believe there are some improvements that can be exposed by separating some of
these concerns.

For light clients, some work is already underway to move toward using P2P
message passing rather than RPC to synchronize light client state with the rest
of the network.  Because of the way light client synchronization interacts with
event logging, this is not as simple as just swapping out the transport: More
details of the event system will need to be made visible via P2P, and that in
turn has some implications for fault tolerance (in particular, I think the
ability of a badly-behaved client to DDoS the network without punishment is a
concern). However, once this work is done, it should in principle be possible
to remove the dependency of light clients on the RPC service.

For bootstrapping and operations, the RPC mechanism still makes sense, but can
be further simplified. Here are some specific proposals:

- Remove the HTTP GET interface entirely.
- Remove the gRPC interface (this is already planned for v0.36).
- Remove the websocket interface (TODO: are websockets used anymore?)
- Simplify the JSON-RPC plumbing to remove unnecessary reflection and interface
  wrapping.

These changes would preserve the ability of operators to issue queries with
curl (but would require using JSON-RPC instead of URI parameters). That would
be a little less user-friendly, but for a use case that should not be that
prevalent.

These changes would also preserve compatibility with existing JSON-RPC based
code paths like the `tendermint` CLI and the light client (even ahead of
further work to remove that dependency).

**Design goal:** An operator should be able to disable non-local access to the
RPC server on any node in the network without impairing the ability of the
network to function for service of state replication, including light clients.

**Design principle:** Any communication required to operate the network should use P2P.

### Options for RPC Transport

[ADR 057][adr-57] proposes using gRPC for the Tendermint RPC implementation.
This is still possible, but if we are able to simplify and decouple the
concerns as described above, I do not think it should be necessary.

While JSON-RPC is not the best possible RPC protocol for all situations, it has
some advantages over gRPC for our domain. Specifically:

- It is easy to call JSON-RPC manually from the command-line, which helps with
  a common concern for the RPC service, local debugging and operations.

- gRPC has an enormous dependency footprint for both clients and servers, and
  many of the features it provides to support security and performance
  (encryption, compression, streaming, etc.) are mostly irrelevant to local
  use.

- If we intend to migrate light clients off RPC to use P2P entirely, there is
  no advantage to forcing a temporary migration to gRPC along the way; and once
  the light client is not dependent on the RPC service, the efficiency of the
  protocol is much less important.

- We can still get the benefits of generated data types using protocol buffers, even
  without using gRPC:

  - Protobuf defines a standard JSON encoding for all message types so
    languages with protobuf support do not need to worry about type mapping
    oddities.

  - Using JSON means that even languages _without_ good protobuf support can
    implement the protocol with a bit more work, and I expect this situation to
    be rare.

Even if a language lacks a good standard JSON-RPC mechanism, the protocol is
lightweight and can be implemented by simple send/receive over TCP or
Unix-domain sockets with no need for code generation, encryption, etc. gRPC
uses a complex HTTP/2 based transport that is not easily replicated.

## References

[abci]: https://github.com/tendermint/spec/tree/95cf253b6df623066ff7cd4074a94e7a3f147c7a/spec/abci
[rpc-service]: https://docs.tendermint.com/master/rpc/
[light-client]: https://docs.tendermint.com/master/tendermint-core/light-client.html
[tm-cli]: https://github.com/tendermint/tendermint/tree/master/cmd/tendermint
[cosmos-sdk]: https://github.com/cosmos/cosmos-sdk/
[local-client]: https://github.com/tendermint/tendermint/blob/master/abci/client/local_client.go
[socket-server]: https://github.com/tendermint/tendermint/blob/master/abci/server/socket_server.go
[json-rpc]: https://www.jsonrpc.org/specification
[adr-57]: https://github.com/tendermint/tendermint/blob/master/docs/architecture/adr-057-RPC.md
