# Peer-to-Peer Communication

This document describes the implementation of the peer-to-peer (p2p)
communication layer in Tendermint.

It is part of an [effort](https://github.com/tendermint/tendermint/issues/9089)
to produce a high-level specification of the operation of the p2p layer adopted
in production Tendermint networks.

This documentation, therefore, considers the releases `0.34.*` of Tendermint, more
specifically, the branch [`v0.34.x`](https://github.com/tendermint/tendermint/tree/v0.34.x)
of this repository.

## Overview

A Tendermint network is composed of multiple Tendermint instances, hereafter
called **nodes**, that interact by exchanging messages.

Tendermint assumes a partially-connected network model.
This means that a node is not assumed to be directly connected to every other
node in the network.
Instead, each node is directly connected to a subset of other nodes in the
network, hereafter called its **peers**.

The peer-to-peer (p2p) communication layer is responsible for establishing
connections between nodes in a Tendermint network,
for managing the communication between a node and its peers,
and for intermediating the exchange of messages between peers in Tendermint protocols.

## Contents

The documentation follows the organization of the `p2p` package of Tendermint,
which implements the following abstractions:

- [Transport](./transport.md): establishes secure and authenticated
   connections with peers;
- [Switch](./switch.md): responsible for dialing peers and accepting
   connections from peers, for managing established connections, and for
   routing messages between the reactors and peers,
   that is, between local and remote instances of the Tendermint protocols;
- [PEX Reactor](./pex.md): a reactor is the implementation of a protocol which
  exchanges messages through the p2p layer. The PEX reactor manages the [Address Book](./addressbook.md)  and implements both the [PEX protocol](./pex-protocol.md) and the  [Peer Manager](./peer_manager.md) role.
   - [Peer Exchange protocol](./pex-protocol.md): enables nodes to exchange peer addresses, thus implementing a peer discovery service;
   - [Address Book](./addressbook.md): stores discovered peer addresses and
  quality metrics associated to peers with which the node has interacted;
   - [Peer Manager](./peer_manager.md): defines when and to which peers a node
  should dial, in order to establish outbound connections;
- Finally, [Types](./types.md) and [Configuration](./configuration.md) provide
   a list of existing types and configuration parameters used by the p2p layer implementation.

## Further References 

Existing documentation referring to the p2p layer:

- https://github.com/tendermint/tendermint/tree/main/spec/p2p: p2p-related
  configuration flags; overview of connections, peer instances, and reactors;
  overview of peer discovery and node types; peer identity, secure connections
  and peer authentication handshake.
- https://github.com/tendermint/tendermint/tree/main/spec/p2p/messages: message
  types and channel IDs of Block Sync, Mempool, Evidence, State Sync, PEX, and
  Consensus reactors.
- https://docs.tendermint.com/v0.34/tendermint-core: the p2p layer
  configuration and operation is documented in several pages.
  This content is not necessarily up-to-date, some settings and concepts may
  refer to the release `v0.35`, that was [discontinued][v35postmorten].
- https://github.com/tendermint/tendermint/tree/master/docs/tendermint-core/pex:
  peer types, peer discovery, peer management overview, address book and peer
  ranking. This documentation refers to the release `v0.35`, that was [discontinued][v35postmorten].

[v35postmorten]: https://interchain-io.medium.com/discontinuing-tendermint-v0-35-a-postmortem-on-the-new-networking-layer-3696c811dabc
