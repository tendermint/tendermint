# Fast Sync Subprotocol Specification

This directory contains English and TLA+ specifications for the FastSync
protocol as it is currently implemented in the Tendermint Core codebase.

## English Specification

The [English Specification](fastsync.md) provides a detailed description of the
fast sync problem and the properties a correct protocol must satisfy. It also
includes a detailed description of the protocol as currently implemented in Go,
and an anlaysis of the implementation with respect to the properties.

It was found that the current implementation does not satisfy certain
properties, and is therefore not a correct solution to the fast sync problem.
The issue discovered holds for all previous implementations of the protocol. A
fix is described which is straight forward to implement.

## TLA+ Specification

Two TLA+ specifications are provided: a high level [specification
of the protocol](fastsync.tla) and a low level [specification of the scheduler
component of the implementation](scheduler.tla). Both specifications contain
properties that may be checked by the TLC model checker, though only for small
values of the relevant parameters.

We will continue to refine these specifications in our research work,
to deduplicate
the redundancies between them, improve their utility to researchers and
engineers, and to improve their verifiability. For now, they provide a complete
description of the fast sync protocol in TLA+; especially the
[scheduler.tla](scheduler.tla), which maps very closely to the current
implementation of the [scheduler in Go](https://github.com/tendermint/tendermint/blob/master/blockchain/v2/scheduler.go).

The [scheduler.tla](scheduler.tla) can be model checked in TLC with the following
parameters:

- Constants:
    - numRequests <- 2
    - PeerIDs <- 0..2
    - ultimateHeight <- 3
- Invariants:
    - TypeOK
- Properties:
    - TerminationWhenNoAdvance
    - TerminationGoodPeers
    - TerminationAllCases
- Proofs that properties are not vacuously true:
    - TerminationGoodPeersPre
    - TerminationAllCases
    - SchedulerIncreasePre
