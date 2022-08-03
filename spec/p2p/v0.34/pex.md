# PEX Reactor

The PEX reactor has multiple roles in the p2p layer:

1. It manages the Address Book, which stores information about known peers
1. It implements the [Peer Exchange protocol](./pex-protocol.md), used by nodes
   to exchange peer addresses
1. It acts as Peer Manager, defining when the node should dial peers and which
   peers it should dial

The roles of PEX reactor, however, are intertwined:

- The peer addresses retrieved by the Peer Exchange protocol are added to the
  Address Book, from which the Peer Exchange protocol also retrieves the peer
  addresses it provides to other nodes.
- The peer addresses to which a node dials are retrieved from the Address Book,
  while the information about incoming peers, accepted by the switch, is added
  to the Address Book.
- In order to discover peer addresses, the Peer Exchange protocol may require
  the node to dial certain nodes. No other protocol (reactor) has this
  prerogative of establishing connections with peers.

## Start

The `OnStart` method starts the PEX reactor.

The reactors address book is started.
This loads the address book content from disk,
and starts a routine that periodically persists the address book content to disk.

The PEX reactor is configured with the address of a number of seed nodes,
the `Seeds` parameter of the `ReactorConfig`.
The seed nodes' addresses are parsed into `NetAddress` instances and resolved into IP addresses.
Valid seed node addresses are stored in the `seedAddr` field of the reactor.
If no valid seed node address is found, and the address book is empty,
the PEX reactor is not started and an error is returned.

The remaining operation of the reactor is determined by the `SeedMode` configuration parameter.
If the node is configured to operate in seed mode, the `crawlPeersRoutine` is started.
Otherwise, the `ensurePeersRoutine` is started.

### Errors

Errors encountered when starting the address book are returned.
An exception is made for the `service.ErrAlreadyStarted` error, which is ignored.

Errors encountered when parsing the configured addresses of seed nodes
are returned and cause the reactor startup to fail.
An exception is made for DNS resolution `ErrNetAddressLookup` errors,
which are not deemed fatal and are only logged as invalid addresses.

## Stop

The `OnStop` method stops the PEX reactor.

The reactors address book is stopped.

## GetChannels

The `GetChannels` method returns the descriptor of the channel used by the PEX protocol.

The channel ID is `PexChannel` (0),
Its priority is set to `1`,
the send queue capacity is set to `10`,
and the maximum message size is configured to `64000` bytes.

## AddPeer

The `AddPeer` method adds a new peer to the PEX protocol.

If the new peer is an inbound peer, i.e., if the node accepted a connection from the peer,
the peer's address is added to the address book.
Since the peer was authenticated when establishing a secret connection with it,
the source of the peer address is trusted, and its source is set by the peer itself.
In the case of an outbound peer, the node should already have its address in
the address book, as the switch has dialed the peer.

If the peer is an outbound peer, i.e., if the node has dialed the peer,
and the PEX protocol needs more addresses,
the node sends a PEX request to the peer.
The same is not done when inbound peers are added because they are deemed least
trustworthy than outbound peers.

> TODO: this is part of the operation of the PEX protocol, link to it.

## RemovePeer

The `RemovePeer` method removes a peer from the PEX protocol.

The peer's ID is removed from the tables of requests send but not yet replied,
and of received requests.

## Receive

> TODO: handle messages exchanged by the PEX protocol

When peer addresses are received from a seed node,
the node immediately attempts to dial the provided peer addresses.

> This behavior was introduced by https://github.com/tendermint/tendermint/issues/2093

## ensurePeersRoutine

TODO:

## dialSeeds

TODO:

## dialPeer

TODO:
