# Tendermint Spec

This repository contains specifications for the Tendermint protocol.

There are currently two implementations of the Tendermint protocol,
maintained by two separate-but-collaborative entities:
One in [Go](https://github.com/tendermint/tendermint),
maintained by Interchain GmbH,
and one in [Rust](https://github.com/informalsystems/tendermint-rs),
maintained by Informal Systems.

### Data Structures

- [Encoding and Digests](./spec/core/encoding.md)
- [Blockchain](./spec/core/data_structures.md)
- [State](./spec/core/state.md)

### Consensus Protocol

- [Consensus Algorithm](./spec/consensus/consensus.md)
- [Creating a proposal](./spec/consensus/creating-proposal.md)
- [Time](./spec/consensus/bft-time.md)
- [Light-Client](./spec/consensus/light-client/README.md)

### P2P and Network Protocols

- [The Base P2P Layer](./spec/p2p/node.md): multiplex the protocols ("reactors") on authenticated and encrypted TCP connections

#### P2P Messages

- [Peer Exchange (PEX)](./spec/p2p/messages/pex.md): gossip known peer addresses so peers can find each other
- [Block Sync](./spec/p2p/messages/block-sync.md): gossip blocks so peers can catch up quickly
- [Consensus](./spec/p2p/messages/consensus.md): gossip votes and block parts so new blocks can be committed
- [Mempool](./spec/p2p/messages/mempool.md): gossip transactions so they get included in blocks
- [Evidence](./spec/p2p/messages/evidence.md): sending invalid evidence will stop the peer

### ABCI

- [ABCI](./spec/abci/README.md): Details about interactions between the
  application and consensus engine over ABCI

### ABCI++

- [ABCI++](./spec/abci++/README.md): Specification of interactions between the
  application and consensus engine over ABCI++

### RFC

- [RFC](./rfc/README.md): RFCs describe proposals to change the spec.
  
### ProtoBuf

- [Proto](./proto/README.md): The data structures of the Tendermint protocol are located in the `proto` directory. These specify P2P messages that each implementation should follow to be compatible.
