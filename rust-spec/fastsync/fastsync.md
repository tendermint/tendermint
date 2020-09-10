# Fastsync

Fastsync is a protocol that is used by a node to catch-up to the
current state of a Tendermint blockchain. Its typical use case is a
node that was disconnected from the system for some time. The
recovering node locally has a copy of a prefix of the blockchain,
and the corresponding application state that is slightly outdated. It
then queries its peers for the blocks that were decided on by the
Tendermint blockchain during the period the full node was
disconnected. After receiving these blocks, it executes the
transactions in the blocks in order to catch-up to the current height
of the blockchain and the corresponding application state.

In practice it is sufficient to catch-up only close to the current
height: The Tendermint consensus reactor implements its own catch-up
functionality and can synchronize a node that is close to the current height,
perhaps within 10 blocks away from the current height of the blockchain.
Fastsync should bring a node within this range.

## Outline

- [Part I](#part-i---tendermint-blockchain): Introduction of Tendermint
blockchain terms that are relevant for FastSync protocol.

- [Part II](#part-ii---sequential-definition-of-fastsync-problem): Introduction
of the problem addressed by the Fastsync protocol.
    - [Fastsync Informal Problem
      statement](#Fastsync-Informal-Problem-statement): For the general
      audience, that is, engineers who want to get an overview over what
      the component is doing from a bird's eye view.

    - [Sequential Problem statement](#Sequential-Problem-statement):
      Provides a mathematical definition of the problem statement in
      its sequential form, that is, ignoring the distributed aspect of
      the implementation of the blockchain.

- [Part III](#part-iii---fastsync-as-distributed-system): Distributed
  aspects of the fast sync problem, system assumptions and temporal
  logic specifications.

    - [Computational Model](#Computational-Model):
      timing and correctness assumptions.

    - [Distributed Problem Statement](#Distributed-Problem-Statement):
      temporal properties that formalize safety and liveness
      properties of fast sync in distributed setting.

- [Part IV](#part-iv---fastsync-protocol): Specification of Fastsync V2
  (the protocol underlying the current Golang implementation).

    - [Definitions](#Definitions): Describes inputs, outputs,
       variables used by the protocol, auxiliary functions

    - [FastSync V2](#FastSync-V2): gives an outline of the solution,
       and details of the functions used (with preconditions,
       postconditions, error conditions).

    - [Algorithm Invariants](#Algorithm-Invariants): invariants over
       the protocol variables that the implementation should maintain.

- [Part V](#part-v---analysis-and-improvements): Analysis
  of Fastsync V2 that highlights several issues that prevent achieving
  some of the desired fault-tolerance properties. We also give some
  suggestions on how to address the issues in the future.

    - [Analysis of Fastsync V2](#Analysis-of-Fastsync-V2): describes
        undesirable scenarios of Fastsync V2, and why they violate
        desirable temporal logic specification in an unreliable
        distributed system.

    - [Suggestions](#Suggestions-for-an-Improved-Fastsync-Implementation)  to address the issues discussed in the analysis.

In this document we quite extensively use tags in order to be able to
reference assumptions, invariants, etc. in future communication. In
these tags we frequently use the following short forms:

- TMBC: Tendermint blockchain
- SEQ: for sequential specifications
- FS: Fastsync
- LIVE: liveness
- SAFE: safety
- INV: invariant
- A: assumption
- V2: refers to specifics of Fastsync V2
- FS-VAR: refers to properties of Fastsync protocol variables
- NewFS: refers to improved future Fastsync implementations

# Part I - Tendermint Blockchain

We will briefly list some of the notions of Tendermint blockchains that are
required for this specification. More details can be found [here][block].

#### **[TMBC-HEADER]**

A set of blockchain transactions is stored in a data structure called
*block*, which contains a field called *header*. (The data structure
*block* is defined [here][block]).  As the header contains hashes to
the relevant fields of the block, for the purpose of this
specification, we will assume that the blockchain is a list of
headers, rather than a list of blocks.

#### **[TMBC-SEQ]**

The Tendermint blockchain is a list *chain* of headers.

#### **[TMBC-SEQ-GROW]**

During operation, new headers may be appended to the list one by one.

> In the following, *ETIME* is a lower bound
> on the time interval between the times at which two
> successor blocks are added.

#### **[TMBC-SEQ-APPEND-E]**

If a header is appended at time *t* then no additional header will be
appended before time *t + ETIME*.

#### **[TMBC-AUTH-BYZ]**

We assume the authenticated Byzantine fault model in which no node (faulty or
correct) may break digital signatures, but otherwise, no additional
assumption is made about the internal behavior of faulty
nodes. That is, faulty nodes are only limited in that they cannot forge
messages.

<!-- The authenticated Byzantine model assumes [TMBC-Sign-NoForge] and -->
<!-- [TMBC-FaultyFull], that is, faulty nodes are limited in that they -->
<!-- cannot forge messages [TMBC-Sign-NoForge]. -->

> We observed that in the existing documentation the term
> *validator* refers to both a data structure and a full node that
> participates in the distributed computation. Therefore, we introduce
> the notions *validator pair* and *validator node*, respectively, to
> distinguish these notions in the cases where they are not clear from
> the context.

#### **[TMBC-VALIDATOR-PAIR]**

Given a full node, a
*validator pair* is a pair *(address, voting_power)*, where

- *address* is the address (public key) of a full node,
- *voting_power* is an integer (representing the full node's
  voting power in a given consensus instance).
  
> In the Golang implementation the data type for *validator
> pair* is called `Validator`.

#### **[TMBC-VALIDATOR-SET]**

A *validator set* is a set of validator pairs. For a validator set
*vs*, we write *TotalVotingPower(vs)* for the sum of the voting powers
of its validator pairs.

#### **[TMBC-CORRECT]**

We define a predicate *correctUntil(n, t)*, where *n* is a node and *t* is a
time point.
The predicate *correctUntil(n, t)* is true if and only if the node *n*
follows all the protocols (at least) until time *t*.

#### **[TMBC-TIME-PARAMS]**

A blockchain has the following configuration parameters:

- *unbondingPeriod*: a time duration.
- *trustingPeriod*: a time duration smaller than *unbondingPeriod*.

#### **[TMBC-FM-2THIRDS]**

If a block *h* is in the chain,
then there exists a subset *CorrV*
of *h.NextValidators*, such that:

- *TotalVotingPower(CorrV) > 2/3
    TotalVotingPower(h.NextValidators)*;
- For every validator pair *(n,p)* in *CorrV*, it holds *correctUntil(n,
    h.Time + trustingPeriod)*.

#### **[TMBC-CORR-FULL]**

Every correct full node locally stores a prefix of the
current list of headers from [**[TMBC-SEQ]**][TMBC-SEQ-link].

# Part II - Sequential Definition of Fastsync Problem

## Fastsync Informal Problem statement

A full node has as input a block of the blockchain at height *h* and
the corresponding application state (or the prefix of the current
blockchain until height *h*). It has access to a set *peerIDs* of full
nodes called *peers* that it knows of.  The full node uses the peers
to read blocks of the Tendermint blockchain (in a safe way, that is,
it checks the soundness conditions), until it has read the most recent
block and then terminates.

## Sequential Problem statement

*Fastsync* gets as input a block of height *h* and the corresponding
application state *s* that corresponds to the block and state of that
height of the blockchain, and produces
as output (i) a list *L* of blocks starting at height *h* to some height
*terminationHeight*, and (ii) the application state when applying the
transactions of the list *L* to *s*.

> In Tendermint, the commit for block of height *h* is contained in block *h + 1*,
> and thus the block of height *h + 1* is needed to verify the block of
> height *h*. Let us therefore clarify the following on the
> termination height:
> The returned value *terminationHeight* is the height of the block with the largest
> height that could be verified. In order to do so, *Fastsync* needs the
> block at height  *terminationHeight + 1* of the blockchain.

Fastsync has to satisfy the following properties:

#### **[FS-SEQ-SAFE-START]**

Let *bh* be the height of the blockchain at the time *Fastsync*
starts. By assumption we have *bh >= h*.
When *Fastsync* terminates, it outputs a list of all blocks from
height *h* to some height *terminationHeight >= bh - 1*.

> The above property is independent of how many blocks are added to the
> blockchain while Fastsync is running. It links the target height to the
> initial state. If Fastsync has to catch-up many blocks, it would be
> better to link the target height to a time close to the
> termination. This is captured by the following specification:

#### **[FS-SEQ-SAFE-SYNC]**

Let *eh* be the height of the blockchain at the time *Fastsync*
terminates. There is a constant *D >= 1* such that when *Fastsync*
terminates, it outputs a list of all blocks from height *h* to some
height *terminationHeight >= eh - D*.

#### **[FS-SEQ-SAFE-STATE]**

Upon termination, the application state is the one that corresponds to
the blockchain at height *terminationHeight*.

#### **[FS-SEQ-LIVE]**

*Fastsync* eventually terminates.

# Part III - FastSync as Distributed System

## Computational Model

#### **[FS-A-NODE]**

We consider a node *FS* that performs *Fastsync*.

#### **[FS-A-PEER-IDS]**

*FS* has access to a set *peerIDs* of IDs (public keys) of peers
     . During the execution of *Fastsync*, another protocol (outside
     of this specification) may add new IDs to *peerIDs*.

#### **[FS-A-PEER]**

Peers can be faulty, and we do not make any assumptions about the number or
ratio of correct/faulty nodes. Faulty processes may be Byzantine
according to [**[TMBC-AUTH-BYZ]**][TMBC-Auth-Byz-link].

#### **[FS-A-VAL]**

The system satisfies [**[TMBC-AUTH-BYZ]**][TMBC-Auth-Byz-link] and
[**[TMBC-FM-2THIRDS]**][TMBC-FM-2THIRDS-link]. Thus, there is a
blockchain that satisfies the soundness requirements (that is, the
validation rules in [[block]]).

#### **[FS-A-COMM]**

Communication between the node *FS* and all correct peers is reliable and
bounded in time: there is a message end-to-end delay *Delta* such that
if a message is sent at time *t* by a correct process to a correct
process, then it will be received and processed by time *t +
Delta*. This implies that we need a timeout of at least *2 Delta* for
remote procedure calls to ensure that the response of a correct peer
arrives before the timeout expires.

## Distributed Problem Statement

### Two Kinds of Termination

We do not assume that there is a correct full node in
*peerIDs*. Under this assumption no protocol can guarantee the combination
of the properties [FS-SEQ-LIVE] and
[FS-SEQ-SAFE-START] and [FS-SEQ-SAFE-SYNC] described in the sequential
specification above. Thus, in the (unreliable) distributed setting, we
consider two kinds of termination (successful and failure) and we will
specify below under what (favorable) conditions *Fastsync* ensures to
terminate successfully, and satisfy the requirements of the sequential
problem statement:

#### **[FS-DIST-LIVE]**

*Fastsync* eventually terminates: it either *terminates successfully* or
it *terminates with failure*.

### Fairness

As mentioned above, without assumptions on the correctness of some
peers, no protocol can achieve the required specifications. Therefore,
we consider the following (fairness) constraint in the
safety and liveness properties below:

#### **[FS-SOME-CORR-PEER]**

Initially, the set *peerIDs* contains at least one correct full node.

> While in principle the above condition [FS-SOME-CORR-PEER]
> can be part of a sufficient
> condition to solve [FS-SEQ-LIVE] and
> [FS-SEQ-SAFE-START] and [FS-SEQ-SAFE-SYNC] in the distributed
> setting (their corresponding properties are given below), we will discuss in
> [Part V](#part-v---analysis-and-improvements) that the
> current implementation of Fastsync (V2) requires the much
> stronger requirement [**[FS-ALL-CORR-PEER]**](#FS-ALL-CORR-PEER)
> given in Part V.

### Safety

> As this specification does
> not assume that a correct peer is at the most recent height
> of the blockchain (it might lag behind), the property [FS-SEQ-SAFE-START]
> cannot be ensured in an unreliable distributed setting. We consider
> the following relaxation. (Which is typically sufficient for
> Tendermint, as the consensus reactor then synchronizes from that
> height.)

#### **[FS-DIST-SAFE-START]**

Let *maxh* be the maximum
height of a correct peer [**[TMBC-CORR-FULL]**][TMBC-CORR-FULL-link]
in *peerIDs* at the time *Fastsync* starts. If *FastSync* terminates
successfully, it is at some height *terminationHeight >= maxh - 1*.

> To address [FS-SEQ-SAFE-SYNC] we consider the following property in
> the distributed setting. See the comments below on the relation to
> the sequential version.

#### **[FS-DIST-SAFE-SYNC]**

Under [FS-SOME-CORR-PEER], there exists a constant time interval *TD*, such
that if *term* is the time *Fastsync* terminates and
*maxh* is the maximum height of a correct peer
[**[TMBC-CORR-FULL]**][TMBC-CORR-FULL-link] in *peerIDs* at the time
*term - TD*, then if *FastSync* terminates successfully, it is at
some height *terminationHeight >= maxh - 1*.

> *TD* might depend on timeouts etc. We suggest that an acceptable
> value for *TD* is in the range of approx. 10 sec., that is the
> interval between two calls `QueryStatus()`; see below.
> We use *term - TD* as reference time, as we have to account
> for communication delay between the peer and *FS*. After the peer sent
> the last message to *FS*, the peer and *FS* run concurrently and
> independently. There is no assumption on the rate at which a peer can
> add blocks (e.g., it might be in the process of catching up
> itself). Hence, without additional assumption we cannot link
> [FS-DIST-SAFE-SYNC] to
> [**[FS-SEQ-SAFE-SYNC]**](#FS-SEQ-SAFE-SYNC), in particular to the
> parameter *D*. We discuss a
> way to achieve this below:
> **Relation to [FS-SEQ-SAFE-SYNC]:**  
> Under [FS-SOME-CORR-PEER], if *peerIDs* contains a full node that is
> "synchronized with the blockchain", and *blockchainheight* is the height
> of the blockchain at time *term*, then  *terminationHeight* may even
> achieve
> *blockchainheight - TD / ETIME*;
> cf. [**[TMBC-SEQ-APPEND-E]**][TMBC-SEQ-APPEND-E-link], that is,
> the parameter *D* from [FS-SEQ-SAFE-SYNC] is in the range of  *TD / ETIME*.

#### **[FS-DIST-SAFE-STATE]**

It is the same as the sequential version
[**[FS-SEQ-SAFE-STATE]**](#FS-SEQ-SAFE-STATE).

#### **[FS-DIST-NONABORT]**

If there is one correct process in *peerIDs* [FS-SOME-CORR-PEER],
*Fastsync* never terminates with failure. (Together with [FS-DIST-LIVE]
 that means it will terminate successfully.)

# Part IV - Fastsync protocol

Here we provide a specification of the FastSync V2 protocol as it is currently
implemented. The V2 design is the result of significant refactoring to improve
the testability and determinism in the implementation. The architecture is
detailed in
[ADR-43](https://github.com/tendermint/tendermint/blob/master/docs/architecture/adr-043-blockchain-riri-org.md).

In the original design, a go-routine (thread of execution) was spawned for each block requested, and
was responsible for both protocol logic and IO. In the V2 design, protocol logic
is decoupled from IO by using three total threads of execution: a scheduler, a
processer, and a demuxer.

The scheduler contains the business logic for managing
peers and requesting blocks from them, while the processor handles the
computationally expensive block execution. Both the scheduler and processor
are structured as finite state machines that receive input events and emit
output events. The demuxer is responsible for all IO, including translating
between internal events and IO messages, and routing events between components.

Protocols in Tendermint can be considered to consist of two
components: a "core" state machine and a "peer" state machine. The core state
machine refers to the internal state managed by the node, while the peer state
machine determines what messages to send to peers. In the FastSync design, the
core and peer state machines correspond to the processor and scheduler,
respectively.

In the case of FastSync, the core state machine (the processor) is effectively
just the Tendermint block execution function, while virtually all protocol logic
is contained in the peer state machine (the scheduler). The processor is
only implemented as a separate component due to the computationally expensive nature
of block execution. We therefore focus our specification here on the peer state machine
(the scheduler component), capturing the core state machine (the processor component)
in the single `Execute` function, defined below.

While the internal details of the `Execute` function are not relevant for the
FastSync protocol and are thus not part of this specification, they will be
defined in detail at a later date in a separate Block Execution specification.

## Definitions

> We now introduce variables and auxiliary functions used by the protocol.

### Inputs

- *startBlock*: the block Fastsync starts from
- *startState*: application state corresponding to *startBlock.Height*

#### **[FS-A-V2-INIT]**

- *startBlock* is from the blockchain
- *startState* is the application state of the blockchain at Height *startBlock.Height*.

### Variables

- *height*: kinitially *startBlock.Height + 1*
  > height should be thought of the "height of the next block we need to download"
- *state*: initially *startState*
- *peerIDs*: peer addresses [FS-A-PEER-IDS](#fs-a-peer-ids)
- *peerHeights*: stores for each peer the height it reported. initially 0
- *pendingBlocks*: stores for each height which peer was
  queried. initially nil for each height
- *receivedBlocks*: stores for each height which peer returned
  it. initially nil
- *blockstore*: stores for each height greater than
    *startBlock.Height*, the block of that height. initially nil for
    all heights
- *peerTimeStamp*: stores for each peer the last time a block was
  received

- *pendingTime*: stores for a given height the time a block was requested
- *peerRate*: stores for each peer the rate of received data in Bytes/second

### Auxiliary Functions

#### **[FS-FUNC-TARGET]**

- *TargetHeight = max {peerHeigts(addr): addr in peerIDs} union {height}*

#### **[FS-FUNC-MATCH]**

```go
func VerifyCommit(b Block, c Commit) Boolean
```

- Comment
    - Corresponds to `verifyCommit(chainID string, blockID
     types.BlockID, height int64, commit *types.Commit) error` in the
     current Golang implementation, which expects blockID and height
  (from the first block) and the
     corresponding commit from the following block. We use the
     simplified form for ease in presentation.

- Implementation remark
    <!-- - implements the check from -->
    <!--  [**[TMBC-SOUND-DISTR-PossCommit]**][TMBC-SOUND-DISTR-PossCommit--link], -->
    <!--  that is, that  *c* is a valid commit for block *b* -->
    - implements the check that  *c* is a valid commit for block *b*
- Expected precondition
    - *c* is a valid commit for block *b*
- Expected postcondition
    - *true* if precondition holds
    - *false* if precondition is violated
- Error condition
    - none

----

### Messages

Peers participating in FastSync exchange the following set of messages. Messages are
encoded using the Amino serialization protocol. We define each message here
using Go syntax, annoted with the Amino type name. The prefix `bc` refers to
`blockchain`, which is the name of the FastSync reactor in the Go
implementation.

#### bcBlockRequestMessage

```go
// type: "tendermint/blockchain/BlockRequest"
type bcBlockRequestMessage struct {
 Height int64
}
```

Remark:

- `msg.Height` > 0

#### bcNoBlockResponseMessage

```go
// type: "tendermint/blockchain/NoBlockResponse"
type bcNoBlockResponseMessage struct {
 Height int64
}
```

Remark:

- `msg.Height` > 0
- This message type is included in the protocol for convenience and is not expected to be sent between two correct peers

#### bcBlockResponseMessage

```go
// type: "tendermint/blockchain/BlockResponse"
type bcBlockResponseMessage struct {
 Block *types.Block
}
```

Remark:

- `msg.Block` is a Tendermint block as defined in [[block]].
- `msg.Block` != nil

#### bcStatusRequestMessage

```go
// type: "tendermint/blockchain/StatusRequest"
type bcStatusRequestMessage struct {
 Height int64
}
```

Remark:

- `msg.Height` > 0

#### bcStatusResponseMessage

```go
// type: "tendermint/blockchain/StatusResponse"
type bcStatusResponseMessage struct {
 Height int64
}
```

Remark:

- `msg.Height` > 0

### Remote Functions

Peers expose the following functions over
remote procedure calls. The "Expected precondition" are only expected for
correct peers (as no assumption is made on internals of faulty
processes [FS-A-PEER]). These functions are implemented using the above defined message types.

> In this document we describe the communication with peers
via asynchronous RPCs.

```go
func Status(addr Address) (int64, error)
```

- Implementation remark
    - RPC to full node *addr*
    - Request message: `bcStatusRequestMessage`.
    - Response message: `bcStatusResponseMessage`.
- Expected precondition
    - none
- Expected postcondition
    - if *addr* is correct: Returns the current height `height` of the
    peer. [FS-A-COMM]
    - if *addr* is faulty: Returns an arbitrary height. [**[TMBC-AUTH-BYZ]**][TMBC-Auth-Byz-link]
- Error condition
    - if *addr* is correct: none. By [FS-A-COMM] we assume communication is reliable and timely.
    - if *addr* is faulty: arbitrary error (including timeout). [**[TMBC-AUTH-BYZ]**][TMBC-Auth-Byz-link]

----

 ```go
func Block(addr Address, height int64) (Block, error)
```

- Implementation remark
    - RPC to full node *addr*
    - Request message: `bcBlockRequestMessage`.
    - Response message: `bcBlockResponseMessage` or `bcNoBlockResponseMessage`.
- Expected precondition
    - 'height` is less than or equal to height of the peer
- Expected postcondition
    - if *addr* is correct: Returns the block of height `height`
  from the blockchain. [FS-A-COMM]
    - if *addr* is faulty: Returns arbitrary or no block [**[TMBC-AUTH-BYZ]**][TMBC-Auth-Byz-link]
- Error condition
    - if *addr* is correct: precondition violated (returns `bcNoBlockResponseMessage`). [FS-A-COMM]
    - if *addr* is faulty: arbitrary error (including timeout). [**[TMBC-AUTH-BYZ]**][TMBC-Auth-Byz-link]

----

## FastSync V2

### Outline

The protocol is described in terms of functions that are triggered by
(external) events. The implementation uses a scheduler and a
de-multiplexer to deal with communicating with peers and to
trigger the execution of these functions:

- `QueryStatus()`: regularly (currently every 10sec; necessarily
  interval greater than *2 Delta*) queries all peers from *peerIDs*
  for their current height [TMBC-CORR-FULL]. It does so
  by calling `Status(n)` remotely on all peers *n*.
  
- `CreateRequest`: regularly checks whether certain blocks have no
  open request. If a block does not have an open request, it requests
  one from a peer. It does so by calling `Block(n,h)` remotely on one
  peer *n* for a missing height *h*.
  
> We have left the strategy how peers are selected unspecified, and
> the currently existing different implementations of Fastsync differ
> in this aspect. In V2, a peer *p* is selected with the minimum number of
> pending requests that can serve the required height *h*, that is
> with *peerHeight(p) >= h*.

The functions `Status` and `Block` are called by asynchronous
RPC. When they return, the following functions are called:

- `OnStatusResponse(addr Address, height int64)`: The full node with
  address *addr* returns its current height. The function updates the height
  information about *addr*, and may also increase *TargetHeight*.
  
- `OnBlockResponse(addr Address, b Block)`. The full node with
  address *addr* returns a block. It is added to *blockstore*. Then
  the auxiliary function `Execute` is called.

- `Execute()`: Iterates over the *blockstore*.  Checks soundness of
  the blocks, and
  executes the transactions of a sound block and updates *state*.

> In addition to the functions above, the following two features are
> implemented in Fastsync V2

#### **[FS-V2-PEER-REMOVE]**

Periodically, *peerTimeStamp* and *peerRate* and *pendingTime* are
analyzed.
If a peer *p*
has not provided a block recently (check of *peerTimeStamp[p]*) or it
has not provided sufficiently many data (check of *peerRate[p]*), then
*p* is removed from *peerIDs*. In addition, *pendingTime* is used to
estimate whether the peer that is responsible for the current height
has provided the corresponding block on time.

#### **[FS-V2-TIMEOUT]**

*Fastsync V2* starts a timeout whenever a block is
executed (that is, when the height is incremented). If the timeout expires
before the next block is executed, *Fastsync* terminates.
If this happens, then *Fastsync* terminates
with failure.

### Details

<!--
> Function signatures followed by pseudocode (optional) and a list of features (required):
> - Implementation remarks (optional)
>   - e.g. (local/remote) function called in the body of this function
> - Expected precondition
> - Expected postcondition
> - Error condition
---->

```go
func QueryStatus()
```

- Expected precondition
    - peerIDs initialized and non-empty
- Expected postcondition
    - call asynchronously `Status(n)` at each peer *n* in *peerIDs*.
- Error condition
    - fails if precondition is violated

----

```go
func OnStatusResponse(addr Address, ht int64)
```

- Comment
    - *ht* is a height
    - peers can provide the status without being called
- Expected precondition
    - *peerHeights(addr) <= ht*
- Expected postcondition
    - *peerHeights(addr) = ht*
    - *TargetHeight* is updated
- Error condition
    - if precondition is violated: *addr* not in *peerIDs* (that is,
      *addr* is removed from *peerIDs*)
- Timeout condition
    - if `OnStatusResponse(addr, ht)` was not invoked within *2 Delta* after
 `Status(addr)` was called:  *addr* not in *peerIDs*

----

```go
func CreateRequest
```

- Expected precondition
    - *height < TargetHeight*
    - *peerIDs* nonempty
- Expected postcondition
    - Function `Block` is called remotely at a peer *addr* in peerIDs
   for a missing height *h*  
   *Remark:* different implementations may have different
      strategies to balance the load over the peers
    - *pendingblocks(h) = addr*

----

```go
func OnBlockResponse(addr Address, b Block)
```

- Comment
    - if after adding block *b*, blocks of heights *height* and
      *height + 1* are in *blockstore*, then `Execute` is called
- Expected precondition
    - *pendingblocks(b.Height) = addr*
    - *b* satisfies basic soundness  
- Expected postcondition
    - if function `Execute` has been executed without error or was not
      executed:
        - *receivedBlocks(b.Height) = addr*
        - *blockstore(b.Height) = b*
        - *peerTimeStamp[addr]* is set to a time between invocation and
          return of the function.
        - *peerRate[addr]* is updated according to size of received
          block and time it has passed between current time and last block received from this peer (addr)
- Error condition
    - if precondition is violated: *addr* not in *peerIDs*; reset
 *pendingblocks(b.Height)* to nil;
- Timeout condition
    - if `OnBlockResponse(addr, b)` was not invoked within *2 Delta* after
 `Block(addr,h)` was called for *b.Height = h*: *addr* not in *peerIDs*

----

```go
func Execute()
```

- Comments
    - none
- Expected precondition
    - application state is the one of the blockchain at height
      *height - 1*
    - **[FS-V2-Verif]** for any two blocks *a* and *b* from
 *receivedBlocks*: if
   *a.Height + 1 = b.Height* then *VerifyCommit (a,b.Commit) = true*
- Expected postcondition
    - Any two blocks *a* and *b* violating [FS-V2-Verif]:
   *a* and *b* not in *blockstore*; nodes with Address
   receivedBlocks(a.Height) and receivedBlocks(b.Height) not in peerIDs
    - height is updated height of complete prefix that matches the blockchain
    - state is the one of the blockchain at height *height - 1*
    - if the new value of *height* is equal to *TargetHeight*, then
 Fastsync
 **terminates
   successfully**.
- Error condition
    - none

----

## Algorithm Invariants

> In contrast to the temporal properties above that define the problem
> statement, the following are invariants on the solution to the
> problem, that is on the algorithm. These invariants are useful for
> the verification, but can also guide the implementation.

#### **[FS-VAR-STATE-INV]**

It is always the case that *state* corresponds to the application state of the
blockchain of that height, that is, *state = chain[height -
1].AppState*; *chain* is defined in
[**[TMBC-SEQ]**][TMBC-SEQ-link].

#### **[FS-VAR-PEER-INV]**

It is always the case that the set *peerIDs* only contains nodes that
have not yet misbehaved (by sending wrong data or timing out).

#### **[FS-VAR-BLOCK-INV]**

For *startBlock.Height <= i < height - 1*, let *b(i)* be the block with
height *i* in *blockstore*, it always holds that
*VerifyCommit(b(i), b(i+1).Commit) = true*. This means that *height*
can only be incremented if all blocks with lower height have been verified.

# Part V - Analysis and Improvements

## Analysis of Fastsync V2

#### **[FS-ISSUE-KILL]**

If two blocks are not matching [FS-V2-Verif], `Execute` dismisses both
blocks and removes the peers that provided these blocks from
*peerIDs*. If block *a* was correct and provided by a correct peer *p*,
and block b was faulty and provided by a faulty peer, the protocol

- removes the correct peer *p*, although it might be useful to
  download blocks from it in the future
- removes the block *a*, so that a fresh copy of *a* needs to be downloaded
  again from another peer
  
By [FS-A-PEER] we do not put a restriction on the number
  of faulty peers, so that faulty peers can make *FS* to remove all
  correct peers from *peerIDs*. As a result, this version of
  *Fastsync* violates [FS-DIST-SAFE-SYNC].

#### **[FS-ISSUE-NON-TERM]**

Due to [**[FS-ISSUE-KILL]**](#fs-issue-kill), from some point on, only
faulty peers may be in *peerIDs*. They can thus control at which rate
*Fastsync* gets blocks. If the timeout duration from [FS-V2-TIMEOUT]
is greater than the time it takes to add a block to the blockchain
(LTIME in [**[TMBC-SEQ-APPEND-E]**][TMBC-SEQ-APPEND-E-link]), the
protocol may never terminate and thus violate [FS-DIST-LIVE].  This
scenario is even possible if a correct peer is always in *peerIDs*,
but faulty peers are regularly asked for blocks.

### Consequence

The issues [FS-ISSUE-KILL] and [FS-ISSUE-NON-TERM] explain why
does not satisfy the property [FS-DIST-LIVE] relevant for termination.
As a result, V2 only solves the specifications in a restricted form,
namely, when all peers are correct:

#### **[FS-ALL-CORR-PEER]**

At all times, the set *peerIDs* contains only correct full nodes.

With this restriction we can give the achieved properties:

#### **[FS-VC-ALL-CORR-NONABORT]**

Under [FS-ALL-CORR-PEER], *Fastsync* never terminates with failure.

#### **[FS-VC-ALL-CORR-LIVE]**

Under [FS-ALL-CORR-PEER], *Fastsync* eventually terminates successfully.

> In a fault tolerance context this is problematic,
> as it means that faulty peers can prevent *FastSync* from termination.
> We observe that this also touches other properties, namely,
> [FS-DIST-SAFE-START] and [FS-DIST-SAFE-SYNC]:
> Termination at an acceptable height are all conditional under
> "successful termination". The properties above severely restrict
> under which circumstances FastSync (V2) terminates successfully.
> As a result, large parts of the current
> implementation of  are not fault-tolerant. We will
> discuss this, and suggestions how to solve this after the
> description of the current protocol.

## Suggestions for an Improved Fastsync Implementation

### Solution for [FS-ISSUE-KILL]

To avoid [FS-ISSUE-KILL], we observe that
[**[TMBC-FM-2THIRDS]**][TMBC-FM-2THIRDS-link] ensures that from the
point a block was created, we assume that more than two thirds of the
validator nodes are correct until the *trustingPeriod* expires.  Under
this assumption, assume the trusting period of *startBlock* is not
expired by the time *FastSync* checks a block *b1* with height
*startBlock.Height + 1*. To do so, we first need to check whether the
Commit in the block *b2* with *startBlock.Height + 2* contains more
than 2/3 of the voting power in *startBlock.NextValidators*. If this
is the case we can check *VerifyCommit (b1,b2.Commit)*. If we perform
checks in this order we observe:

- By assumption, *startBlock* is OK,
- If the first check (2/3 of voting power) fails,
    the peer that provided block *b2* is faulty,
- If the first check passes and the second check
    fails (*VerifyCommit*), then the peer that provided *b1* is
    faulty.
- If both checks pass, we can trust *b1*

Based on this reasoning, we can ensure to only remove faulty peers
from *peerIDs*.  That is, if
we sequentially verify blocks starting with *startBlock*, we will
never remove a correct peer from *peerIDs* and we will be able to
ensure the following invariant:

#### **[NewFS-VAR-PEER-INV]**

If a peer never misbehaves, it is never removed from *peerIDs*. It
follows that under [FS-SOME-CORR-PEER], *peerIDs* is always non-empty.

> To ensure this, we suggest to change the protocol as follows:

#### Fastsync has the following configuration parameters

- *trustingPeriod*: a time duration; cf.
  [**[TMBC-TIME-PARAMS]**][TMBC-TIME-PARAMS-link].

> [NewFS-A-INIT] is the suggested replacement of [FS-A-V2-INIT]. This will
> allow us to use the established trust to understand precisely which
> peer reported an invalid block in order to ensure the
> invariant [NewFS-VAR-TRUST-INV] below:

#### **[NewFS-A-INIT]**

- *startBlock* is from the blockchain, and within *trustingPeriod*
(possible with some extra margin to ensure termination before
*trustingPeriod* expired)
- *startState* is the application state of the blockchain at Height
  *startBlock.Height*.
- *startHeight = startBlock.Height*

#### Additional Variables

- *trustedBlockstore*: stores for each height greater than or equal to
    *startBlock.Height*, the block of that height. Initially it
    contains only *startBlock*

#### **[NewFS-VAR-TRUST-INV]**

Let *b(i)* be the block in *trustedBlockstore*
with b(i).Height = i. It holds that
for *startHeight < i < height - 1*,
*VerifyCommit (b(i),b(i+1).Commit) = true*.

> We propose to update the function `Execute`. To do so, we first
> define the following helper functions:

```go
func ValidCommit(VS ValidatorSet, C Commit) Boolean
```

- Comments
    - checks validator set based on [**[TMBC-FM-2THIRDS]**][TMBC-FM-2THIRDS-link]
- Expected precondition
    - The validators in *C*
        - are a subset of VS
        - have more than 2/3 of the voting power in VS
- Expected postcondition
    - returns *true* if precondition holds, and *false* otherwise
- Error condition
    - none

----

```go
func SequentialVerify {
 while (true) {
  b1 = blockstore[height];
  b2 = blockstore[height+1];
  if b1 == nil or b2 == nil {
   exit;
  }
  if ValidCommit(trustedBlockstore[height - 1].NextValidators, b2.commit) {
   // we trust b2
   if VerifyCommit(b1, b2.commit) {
    trustedBlockstore.Add(b1);
    height = height + 1;
   }
   else {
    // as we trust b2, b1 must be faulty
    blockstore.RemoveFromPeer(receivedBlocks[height]);
    // we remove all blocks received from the faulty peer
    peerIDs.Remove(receivedBlocks(bnew.Height));
    exit;

   }
  } else {
   // b2 is faulty
   blockstore.RemoveFromPeer(receivedBlocks[height + 1]);
   // we remove all blocks received from the faulty peer
      peerIDs.Remove(receivedBlocks(bnew.Height));
   exit;   }
  }
}
```

- Comments
    - none
- Expected precondition
    - [NewFS-VAR-TRUST-INV]
- Expected postcondition
    - [NewFS-VAR-TRUST-INV]
    - there is no block *bnew* with *bnew.Height = height + 1* in
      *blockstore*
- Error condition
    - none

----

> Then `Execute` just consists in calling `SequentialVerify` and then
> updating the application state to the (new) height.

```go
func Execute()
```

- Comments
    - first `SequentialVerify` is executed
- Expected precondition
    - application state is the one of the blockchain at height
      *height - 1*
    - [NewFS-NOT-EXP] *trustedBlockstore[height-1].Time > now - trustingPeriod*
- Expected postcondition
    - there is no block *bnew* with *bnew.Height = height + 1* in
      *blockstore*
    - state is the one of the blockchain at height *height - 1*
    - if height = TargetHeight: **terminate successfully**
- Error condition
    - fails if [NewFS-NOT-EXP] is violated

----

### Solution for [FS-ISSUE-NON-TERM]

As discussed above, the advantageous termination requirement is the
combination of [FS-DIST-LIVE] and [FS-DIST-NONABORT], that is, *Fastsync*
should terminate successfully in case there is at least one correct
peer in *peerIDs*. For this we have to ensure that faulty processes
cannot slow us down and provide blocks at a lower rate than the
blockchain may grow. To ensure that we will have to add an assumption
on message delays.

#### **[NewFS-A-DELTA]**

*2 Delta < ETIME*; cf. [**[TMBC-SEQ-APPEND-E]**][TMBC-SEQ-APPEND-E-link].

> This assumption implies that the timeouts for `OnBlockResponse` and
> `OnStatusResponse` are such that a faulty peer that tries to respond
> slower than *2 Delta* will be removed. In the following we will
> provide a rough estimate on termination time in a fault-prone
> scenario.
> In the following
> we assume that during a "long enough" finite good period no new
> faulty peers are added to *peerIDs*. Below we will sketch how "long
> enough" can be estimated based on the timing assumption in this
> specification.

#### **[NewFS-A-STATUS-INTERVAL]**

Let Sigma be the (upper bound on the)
time between two calls of `QueryStatus()`.

#### **[NewFS-A-GOOD-PERIOD]**

A time interval *[begin,end]* is *good period* if:

- *fmax* is the number of faulty peers in *peerIDs* at time *begin*
- *end >= begin + 2 Delta (fmax + 3)*
- no faulty peer is added before time *end*

> In the analysis below we assume that the termination condition of
> *Fastsync* is
> *height = TargetHeight* in the postcondition of
> `Execute`. Therefore, [NewFS-A-STATUS-INTERVAL] does not interfere
> with this analysis. If a correct peer reports a new height "shortly
> before termination" this leads to an additional round trip to
> request and add the new block. Then [NewFS-A-DELTA] ensures that
> *Fastsync* catches up.

Arguments:

1. If a faulty peer *p* reports a faulty block, `SequentialVerify` will
  eventually remove *p* from *peerIDs*
  
2. By `SequentialVerify`, if a faulty peer *p* reports multiple faulty
  blocks, *p* will be removed upon trying to check the block with the
  smallest height received from *p*.

3. Assume whenever a block does not have an open request, `CreateRequest` is
   called immediately, which calls `Block(n)` on a peer. Say this
   happens at time *t*. There are two cases:
  
   - by t + 2 Delta a block is added to *blockStore*
   - at t + 2 Delta `Block(n)` timed out and *n* is removed from
       peer.

4. Let *f(t)* be the number of faulty peers in *peerIDs* at time *t*;  
   *f(begin) = fmax*.

5. Let t_i be the sequence of times `OnBlockResponse(addr,b)` is
   invoked or times out with *b.Height = height + 1*.

6. By 3.,
   - (a). *t_1 <= begin + 2 Delta*
   - (b). *t_{i+1} <= t_i + 2 Delta*

7. By an inductive argument we prove for *i > 0* that

   - (a). *height(t_{i+1}) > height(t_i)*, or
   - (b). *f(t_{i+1}) < f(t_i))* and *height(t_{i+1}) = height(t_i)*  

   Argument: if the peer is faulty and does not return a block, the
   peer is removed, if it is faulty and returns a faulty block
   `SequentialVerify` removes the peer (b). If the returned block is OK,
   height is increased (a).
  
8. By 2. and 7., faulty peers can delay incrementing the height at
   most *fmax* times, where each time "costs" *2 Delta* seconds. We
   have additional *2 Delta* initial offset (3a) plus *2 Delta* to get
   all missing blocks after the last fault showed itself. (This
   assumes that an arbitrary number of blocks can be obtained and
   checked within one round-trip 2 Delta; which either needs
   conservative estimation of Delta, or a more refined analysis). Thus
   we reach the *targetHeight* and terminate by time *end*.

# References

<!--
> links to other specifications/ADRs this document refers to
---->

[[block]] Specification of the block data structure.

<!-- [[blockchain]] The specification of the Tendermint blockchain. Tags refering to this specification are labeled [TMBC-*]. -->

[block]: https://github.com/tendermint/spec/blob/d46cd7f573a2c6a2399fcab2cde981330aa63f37/spec/core/data_structures.md

<!-- [blockchain]: https://github.com/informalsystems/VDD/tree/master/blockchain/blockchain.md -->

[TMBC-HEADER-link]: #tmbc-header

[TMBC-SEQ-link]: #tmbc-seq

[TMBC-CORR-FULL-link]: #tmbc-corrfull

[TMBC-CORRECT-link]: #tmbc-correct

[TMBC-Sign-link]: #tmbc-sign

[TMBC-FaultyFull-link]: #tmbc-faultyfull

[TMBC-TIME-PARAMS-link]: #tmbc-time-params

[TMBC-SEQ-APPEND-E-link]: #tmbc-seq-append-e

[TMBC-FM-2THIRDS-link]: #tmbc-fm-2thirds

[TMBC-Auth-Byz-link]: #tmbc-auth-byz

[TMBC-INV-SIGN-link]: #tmbc-inv-sign

[TMBC-SOUND-DISTR-PossCommit--link]: #tmbc-sound-distr-posscommit

[TMBC-INV-VALID-link]: #tmbc-inv-valid

<!-- [TMBC-HEADER-link]: https://github.com/informalsystems/VDD/tree/master/blockchain/blockchain.md#tmbc-header -->

<!-- [TMBC-SEQ-link]: https://github.com/informalsystems/VDD/tree/master/blockchain/blockchain.md#tmbc-seq -->

<!-- [TMBC-CORR-FULL-link]: https://github.com/informalsystems/VDD/tree/master/blockchain/blockchain.md#tmbc-corrfull -->

<!-- [TMBC-Sign-link]: https://github.com/informalsystems/VDD/tree/master/blockchain/blockchain.md#tmbc-sign -->

<!-- [TMBC-FaultyFull-link]: https://github.com/informalsystems/VDD/tree/master/blockchain/blockchain.md#tmbc-faultyfull -->

<!-- [TMBC-TIME-PARAMS-link]: https://github.com/informalsystems/VDD/tree/master/blockchain/blockchain.md#tmbc-time-params -->

<!-- [TMBC-SEQ-APPEND-E-link]: https://github.com/informalsystems/VDD/tree/master/blockchain/blockchain.md#tmbc-seq-append-e -->

<!-- [TMBC-FM-2THIRDS-link]: https://github.com/informalsystems/VDD/tree/master/blockchain/blockchain.md#tmbc-fm-2thirds -->

<!-- [TMBC-Auth-Byz-link]: https://github.com/informalsystems/VDD/tree/master/blockchain/blockchain.md#tmbc-auth-byz -->

<!-- [TMBC-INV-SIGN-link]: https://github.com/informalsystems/VDD/tree/master/blockchain/blockchain.md#tmbc-inv-sign -->

<!-- [TMBC-SOUND-DISTR-PossCommit--link]: https://github.com/informalsystems/VDD/tree/master/blockchain/blockchain.md#tmbc-sound-distr-posscommit -->

<!-- [TMBC-INV-VALID-link]: https://github.com/informalsystems/VDD/tree/master/blockchain/blockchain.md#tmbc-inv-valid -->

[LCV-VC-LIVE-link]: https://github.com/informalsystems/VDD/tree/master/lightclient/verification.md#lcv-vc-live

[lightclient]: https://github.com/interchainio/tendermint-rs/blob/e2cb9aca0b95430fca2eac154edddc9588038982/docs/architecture/adr-002-lite-client.md

[failuredetector]: https://github.com/informalsystems/VDD/blob/master/liteclient/failuredetector.md

[fullnode]: https://github.com/tendermint/spec/blob/master/spec/blockchain/fullnode.md

[FN-LuckyCase-link]: https://github.com/tendermint/spec/blob/master/spec/blockchain/fullnode.md#fn-luckycase

[blockchain-validator-set]: https://github.com/tendermint/spec/blob/d46cd7f573a2c6a2399fcab2cde981330aa63f37/spec/core/data_structures.md#data-structures

[fullnode-data-structures]: https://github.com/tendermint/spec/blob/master/spec/blockchain/fullnode.md#data-structures

[FN-ManifestFaulty-link]: https://github.com/tendermint/spec/blob/master/spec/blockchain/fullnode.md#fn-manifestfaulty
