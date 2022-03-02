---
order: 1
title: Overview
parent:
  title: Spec
  order: 7
---

# Tendermint Specifications

This directory hosts the canonical Markdown specifications of the Tendermint Protocol.

It shall be used to describe protocol semantics, namely the BFT consensus engine, leader election, block propagation and light client verification. The specification includes encoding descriptions used in interprocess communication to comply with the protocol. It defines the interface between the application and Tendermint. The english specifications are often accompanies with a TLA+ specification.

## Table of Contents

- [Overview](#overview)
- [Application Blockchain Interface](./abci++/README.md)
- [Encoding and Digests](./core/encoding.md)
- [Core Data Structures](./core/data_structures.md)
- [State](./core/state.md)
- [Consensus Algorithm](./consensus/consensus.md)
- [Creating a proposal](./consensus/creating-proposal.md)
- [Time](./consensus/proposer-based-timestamp/README.md)
- [Light Client](./consensus/light-client/README.md)
- [The Base P2P Layer](./p2p/node.md)
- [Peer Exchange (PEX)](./p2p/messages/pex.md)
- [Remote Procedure Calls](./rpc/README.md)
- [Write-Ahead Log](./consensus/wal.md)
- [Ivy Proofs](./ivy-proofs/README.md)

## Contibuting

Contributions are welcome.

Proposals at an early stage can first be drafted as Github issues. To progress, a proposal will often need to be written out and approved as a [Request For Comment (RFC)](../docs/rfc/README.md).

The standard language for coding blocks is Golang.

If you find discrepancies between the spec and the code that
do not have an associated issue or pull request on github,
please submit them to our [bug bounty](https://tendermint.com/security)!

## Overview

Tendermint provides Byzantine Fault Tolerant State Machine Replication using
hash-linked batches of transactions. Such transaction batches are called "blocks".
Hence, Tendermint defines a "blockchain".

Each block in Tendermint has a unique index - its Height.
Height's in the blockchain are monotonic.
Each block is committed by a known set of weighted Validators.
Membership and weighting within this validator set may change over time.
Tendermint guarantees the safety and liveness of the blockchain
so long as less than 1/3 of the total weight of the Validator set
is malicious or faulty.

A commit in Tendermint is a set of signed messages from more than 2/3 of
the total weight of the current Validator set. Validators take turns proposing
blocks and voting on them. Once enough votes are received, the block is considered
committed. These votes are included in the _next_ block as proof that the previous block
was committed - they cannot be included in the current block, as that block has already been
created.

Once a block is committed, it can be executed against an application.
The application returns results for each of the transactions in the block.
The application can also return changes to be made to the validator set,
as well as a cryptographic digest of its latest state.

Tendermint is designed to enable efficient verification and authentication
of the latest state of the blockchain. To achieve this, it embeds
cryptographic commitments to certain information in the block "header".
This information includes the contents of the block (eg. the transactions),
the validator set committing the block, as well as the various results returned by the application.
Note, however, that block execution only occurs _after_ a block is committed.
Thus, application results can only be included in the _next_ block.

Also note that information like the transaction results and the validator set are never
directly included in the block - only their cryptographic digests (Merkle roots) are.
Hence, verification of a block requires a separate data structure to store this information.
We call this the `State`. Block verification also requires access to the previous block.
