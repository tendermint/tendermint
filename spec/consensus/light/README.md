# Tendermint Light Client Protocol

## Contents

- [Motivation](#motivation)
- [Structure](#structure)
- [Core Verification](./verification.md)
- [Fork Detection](./detection.md)
- [Fork Accountability](./accountability.md)

## Motivation

The Tendermint Light Client is motivated by the need for a light weight protocol
to sync with a Tendermint blockchain, with the least processing necessary to
securely verify a recent state. The protocol consists
primarily of checking hashes and verifying Tendermint commit signatures to
update trusted validator sets and committed block headers from the chain.

Motivating use cases include:

- Light Node: a daemon that syncs a blockchain to the latest committed header by making RPC requests to full nodes.
- State Sync: a reactor that syncs a blockchain to a recent committed state by making P2P requests to full nodes.
- IBC Client: an ABCI application library that syncs a blockchain to a recent committed header by receiving proof-carrying
transactions from "IBC relayers", who make RPC requests to full nodes on behalf of the IBC clients.

## Structure

The Tendermint Light Client consists of three primary components:  

- [Core Verification](./verification.md): verifying hashes, signatures, and validator set changes
- [Fork Detection](./detection.md): talking to multiple peers to detect byzantine behaviour
- [Fork Accountability](./accountability.md): analyzing byzantine behaviour to hold validators accountable.

While every light client must perform core verification and fork detection
to achieve their prescribed security level, fork accountability is expected to
be done by full nodes and validators, and is thus more accurately a component of
the full node consensus protocol, though it is included here since it is
primarily concerned with providing security to light clients.

Light clients are fundamentally synchronous protocols, 
where security is grounded ultimately in the interval during which a validator can be punished
for byzantine behaviour. We assume here that such intervals have fixed and known minima
referred to commonly as a blockchain's Unbonding Period.

A secure light client must guarantee that all three components - 
core verification, fork detection, and fork accountability - 
each with their own synchrony assumptions and fault model, can execute
sequentially and to completion within the given Unbonding Period.

