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

A Tendermint network is composed by multiple Tendermint instances, thereafter
called **nodes**, that interact by exchanging messages.

Tendermint assumes a partially-connected network model.
This means that a node is not assumed to be directly connected to every other
node in the network.
Instead, each node is directly connected to a subset of other nodes in the
network, thereafter called its **peers**.

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
- [PEX Reactor](./pex.md): responsible for peer discovery, ranking, and
   management, roles that are implemented by:
   - [Address Book](./addressbook.md): tracks information about other nodes
      in the network and allows ranking peers based on quality metrics;
   - [Peer Exchange protocol](./pex-protocol.md): enables nodes to exchange
      peer addresses, thus implementing a peer discovery mechanism;
   - [Peer Manager](./peer_manager.md): responsible for establishing
      connection with peers, defining when and which peers a node should dial;
- Finally, [Types](/.types.md) and [Configuration](./configuration.md) provide
   a list of existing types and configuration parameters used by the p2p layer implementation.

<!-- 
Move to Docs but reuse perhaps before moving

## Introduction 

This documentation aims at separating the logical components on a protocol level from the implementation details of each protocol. 

At a high level, the p2p layer in Tendermint has the following main functionalities:
1. Peer management: peer discovery and peer ranking
2. Peer connection handling: dialing and accepting connections
3. Message transfer
   

Peer discovery, peer management, connection handling and message types. 

The implementation of these three functionalities is split between different Tendermint components as shown in the tables below. 

#### **Peer communication** 
| [Peer discovery](peer_manager.md) | [Peer dialing](switch.md#dialing-peers) | [Accepting connections from peers](switch.md#accepting-peers) | Connection management (processing msgs) |
| ---| ---| ---| --- | 
| PEX / config | PEX / Switch | Reactors / Switch | Reactors| 


#### **Peer management**
| [Peer ranking](addressbook.md#pick-address) | Connection upgrading | [Evicting](pex-protocol.md#misbehavior)| 
| --- | --- | --- |
| PEX / reactors (only marking peers as good/bad); address book (actual ranking)| - | PEX reactor| 

### Node types

From a p2p perspective, within a network, Tendermint distinguishes between regular and [seed nodes](pex-protocol.md#seed-nodes). 
While regular nodes try to form connections between one another, the main role of a seed node is to provide other nodes with addresses. 
-->

## References 

Documents that describe some of the functionality of the p2p layer prior to this specification:

- https://github.com/tendermint/tendermint/tree/master/spec/p2p : Peer information (handshake and addresses); Mconn package;
Tendermint nodes can connect and communicate via p2p to one another. The main high level responsibilities of the p2p layer are  1) establishing and maintaining connections between peers and 2) managing the state of peers. 
- https://github.com/tendermint/tendermint/tree/master/docs/tendermint-core/pex : PEX reactor (*Note* I am not sure that the peer exchange section is valid anymore)
- 
