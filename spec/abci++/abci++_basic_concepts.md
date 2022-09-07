---
order: 1
title: Overview and basic concepts
---

## Outline

- [ABCI++ vs. ABCI](#abci-vs-abci)
- [Method overview](#method-overview)
    - [Consensus/block execution methods](#consensusblock-execution-methods)
    - [Mempool methods](#mempool-methods)
    - [Info methods](#info-methods)
    - [State-sync methods](#state-sync-methods)
- [Next-block execution vs. same-block execution](#next-block-execution-vs-same-block-execution)
- [Tendermint proposal timeout](#tendermint-proposal-timeout)
- [Deterministic State-Machine Replication](#deterministic-state-machine-replication)
- [Events](#events)
- [Evidence](#evidence)
- [Errors](#errors)

# Overview and basic concepts

## ABCI++ vs. ABCI

[&uparrow; Back to Outline](#outline)

The Application's main role is to execute blocks decided (a.k.a. finalized) by consensus. The
decided blocks are the main consensus's ouput to the (replicated) Application. With ABCI, the
application only interacts with consensus at *decision* time. This restricted mode of interaction
prevents numerous features for the Application, including many scalability improvements that are
now better understood than when ABCI was first written. For example, many ideas proposed to improve
scalability can be boiled down to "make the block proposers do work, so the network does not have
to". This includes optimizations such as transaction level signature aggregation, state transition
proofs, etc. Furthermore, many new security properties cannot be achieved in the current paradigm,
as the Application cannot require validators to do more than executing the transactions contained in
finalized blocks. This includes features such as threshold cryptography, and guaranteed IBC
connection attempts.

ABCI++ addresses these limitations by allowing the application to intervene at three key places of
consensus execution: (a) at the moment a new proposal is to be created, (b) at the moment a
proposal is to be validated, and (c) at the moment a (precommit) vote is sent/received. The new
interface allows block proposers to perform application-dependent work in a block through the
`PrepareProposal` method (a); validators to perform application-dependent work and checks in a
proposed block through the `ProcessProposal` method (b); and applications to require their validators
do more than just validate blocks through the `ExtendVote` and `VerifyVoteExtension` methods (c).
Furthermore, ABCI++ coalesces {`BeginBlock`, [`DeliverTx`], `EndBlock`} into `FinalizeBlock`, as a
simplified, efficient way to deliver a decided block to the Application.

## Method overview

[&uparrow; Back to Outline](#outline)

Methods can be classified into four categories: *consensus*, *mempool*, *info*, and *state-sync*.

### Consensus/block execution methods

The first time a new blockchain is started, Tendermint calls `InitChain`. From then on, method
`FinalizeBlock` is executed upon the decision of each block, resulting in an updated Application
state. During the execution of an instance of consensus, which decides the block for a given
height, and before method `FinalizeBlock` is called, methods `PrepareProposal`, `ProcessProposal`,
`ExtendVote`, and `VerifyVoteExtension` may be called several times. See
[Tendermint's expected behavior](abci++_tmint_expected_behavior.md) for details on the possible
call sequences of these methods.

- [**InitChain:**](./abci++_methods.md#initchain) This method initializes the blockchain.
  Tendermint calls it once upon genesis.

- [**PrepareProposal:**](./abci++_methods.md#prepareproposal) It allows the block
  proposer to perform application-dependent work in a block before proposing it.
  This enables, for instance, batch optimizations to a block, which has been empirically
  demonstrated to be a key component for improved performance. Method `PrepareProposal` is called
  every time Tendermint is about to broadcast a Proposal message, but no previous proposal has
  been locked at the Tendermint level. Tendermint gathers outstanding transactions from the
  mempool, generates a block header, and uses them to create a block to propose. Then, it calls
  `RequestPrepareProposal` with the newly created proposal, called *raw proposal*. The Application
  can make changes to the raw proposal, such as modifying transactions, and returns the
  (potentially) modified proposal, called *prepared proposal* in the `ResponsePrepareProposal`
  call. The logic modifying the raw proposal can be non-deterministic.

- [**ProcessProposal:**](./abci++_methods.md#processproposal) It allows a validator to
  perform application-dependent work in a proposed block. This enables features such as immediate
  block execution, and allows the Application to reject invalid blocks.
  Tendermint calls it when it receives a proposal and the Tendermint algorithm has not locked on a
  value. The Application cannot modify the proposal at this point but can reject it if it is
  invalid. If that is the case, Tendermint will prevote `nil` on the proposal, which has
  strong liveness implications for Tendermint. As a general rule, the Application
  SHOULD accept a prepared proposal passed via `ProcessProposal`, even if a part of
  the proposal is invalid (e.g., an invalid transaction); the Application can
  ignore the invalid part of the prepared proposal at block execution time.

- [**ExtendVote:**](./abci++_methods.md#extendvote) It allows applications to force their
  validators to do more than just validate within consensus. `ExtendVote` allows applications to
  include non-deterministic data, opaque to Tendermint, to precommit messages (the final round of
  voting). The data, called *vote extension*, will be broadcast and received together with the
  vote it is extending, and will be made available to the Application in the next height,
  in the rounds where the local process is the proposer.
  Tendermint calls `ExtendVote` when it is about to send a non-`nil` precommit message.
  If the Application does not have vote extension information to provide at that time, it returns
  a 0-length byte array as its vote extension.

- [**VerifyVoteExtension:**](./abci++_methods.md#verifyvoteextension) It allows
  validators to validate the vote extension data attached to a precommit message. If the validation
  fails, the whole precommit message will be deemed invalid and ignored by Tendermint.
  This has a negative impact on Tendermint's liveness, i.e., if vote extensions repeatedly cannot be
  verified by correct validators, Tendermint may not be able to finalize a block even if sufficiently
  many (+2/3) validators send precommit votes for that block. Thus, `VerifyVoteExtension`
  should be used with special care.
  As a general rule, an Application that detects an invalid vote extension SHOULD
  accept it in `ResponseVerifyVoteExtension` and ignore it in its own logic. Tendermint calls it when
  a process receives a precommit message with a (possibly empty) vote extension.

- [**FinalizeBlock:**](./abci++_methods.md#finalizeblock) It delivers a decided block to the
  Application. The Application must execute the transactions in the block deterministically and
  update its state accordingly. Cryptographic commitments to the block and transaction results,
  returned via the corresponding parameters in `ResponseFinalizeBlock`, are included in the header
  of the next block. Tendermint calls it when a new block is decided.

- [**Commit:**](./abci++_methods.md#commit) Instructs the Application to persist its
  state. It is a fundamental part of Tendermint's crash-recovery mechanism that ensures the
  synchronization between Tendermint and the Applicatin upon recovery. Tendermint calls it just after
  having persisted the data returned by `ResponseFinalizeBlock`. The Application can now discard
  any state or data except the one resulting from executing the transactions in the decided block.

### Mempool methods

- [**CheckTx:**](./abci++_methods.md#checktx) This method allows the Application to validate
  transactions. Validation can be stateless (e.g., checking signatures ) or stateful
  (e.g., account balances). The type of validation performed is up to the application. If a
  transaction passes the validation, then Tendermint adds it to the mempool; otherwise the
  transaction is discarded.
  Tendermint calls it when it receives a new transaction either coming from an external
  user (e.g., a client) or another node. Furthermore, Tendermint can be configured to call
  re-`CheckTx` on all outstanding transactions in the mempool after calling `Commit`for a block.

### Info methods

- [**Info:**](./abci++_methods.md#info) Used to sync Tendermint with the Application during a
  handshake that happens upon recovery, or on startup when state-sync is used.

- [**Query:**](./abci++_methods.md#query) This method can be used to query the Application for
  information about the application state.

### State-sync methods

State sync allows new nodes to rapidly bootstrap by discovering, fetching, and applying
state machine (application) snapshots instead of replaying historical blocks. For more details, see the
[state sync documentation](../p2p/messages/state-sync.md).

New nodes discover and request snapshots from other nodes in the P2P network.
A Tendermint node that receives a request for snapshots from a peer will call
`ListSnapshots` on its Application. The Application returns the list of locally available
snapshots.
Note that the list does not contain the actual snapshots but metadata about them: height at which
the snapshot was taken, application-specific verification data and more (see
[snapshot data type](./abci++_methods.md#snapshot) for more details). After receiving a
list of available snapshots from a peer, the new node can offer any of the snapshots in the list to
its local Application via the `OfferSnapshot` method. The Application can check at this point the
validity of the snapshot metadata.

Snapshots may be quite large and are thus broken into smaller "chunks" that can be
assembled into the whole snapshot. Once the Application accepts a snapshot and
begins restoring it, Tendermint will fetch snapshot "chunks" from existing nodes.
The node providing "chunks" will fetch them from its local Application using
the `LoadSnapshotChunk` method.

As the new node receives "chunks" it will apply them sequentially to the local
application with `ApplySnapshotChunk`. When all chunks have been applied, the
Application's `AppHash` is retrieved via an `Info` query.
To ensure that the sync proceeded correctly, Tendermint compares the local Application's `AppHash`
to the `AppHash` stored on the blockchain (verified via
[light client verification](../light-client/verification/README.md)).

In summary:

- [**ListSnapshots:**](./abci++_methods.md#listsnapshots) Used by nodes to discover available
  snapshots on peers.

- [**OfferSnapshot:**](./abci++_methods.md#offersnapshot) When a node receives a snapshot from a
  peer, Tendermint uses this method to offer the snapshot to the Application.

- [**LoadSnapshotChunk:**](./abci++_methods.md#loadsnapshotchunk) Used by Tendermint to retrieve
  snapshot chunks from the Application to send to peers.

- [**ApplySnapshotChunk:**](./abci++_methods.md#applysnapshotchunk) Used by Tendermint to hand
  snapshot chunks to the Application.

### Other methods

Additionally, there is a [**Flush**](./abci++_methods.md#flush) method that is called on every connection,
and an [**Echo**](./abci++_methods.md#echo) method that is used for debugging.

More details on managing state across connections can be found in the section on
[Managing Application State](./abci%2B%2B_app_requirements.md#managing-the-application-state-and-related-topics).

## Next-block execution vs. same-block execution

[&uparrow; Back to Outline](#outline)

In the original ABCI protocol, the only moment when the Application had access to a
block was after it was decided. This led to a block execution model, called *next-block
execution*, where some fields hashed in a block header refer to the execution of the
previous block, namely:

- the Merkle root of the Application's state
- the transaction results
- the consensus parameter updates
- the validator updates

With ABCI++, an Application may be configured to keep using the next-block execution model, by
executing the decided block in `FinalizeBlock`. However, the new methods introduced &mdash;
`PrepareProposal` and `ProcessProposal` &mdash; disclose the entire proposed block to the
Application, allowing for its immediate exectution. An Application implementing immediate execution
may additionally wish to store certain data resulting from the block's execution in the same block
that has just been executed. This brings about a new execution model, called
*same-block execution*. An Application implementing this execution model, upon receiving a raw
proposal via `RequestPrepareProposal` and potentially modifying its transaction list, fully
executes the resulting prepared proposal as though it was the decided block (immediate execution),
and the results of the block execution are used as follows:

- The block execution may generate a set of events. The Application should store these events and
  return them back to Tendermint during the `FinalizeBlock` call if the block is finally decided.
- The Merkle root resulting from executing the prepared proposal is provided in
  `ResponsePrepareProposal` and thus refers to the **current block**. Tendermint
  will use it in the prepared proposal's header.
- Likewise, the transaction results from executing the prepared proposal are
  provided in `ResponsePrepareProposal` and refer to the transactions in the
  **current block**. Tendermint will use them to calculate the results hash
  in the prepared proposal's header.
- The consensus parameter updates and validator updates are also provided in
  `ResponsePrepareProposal` and reflect the result of the prepared proposal's
  execution. They come into force in height H+1 (as opposed to the H+2 rule
  in next-block execution model).

If the Application is configured to keep the next-block execution model, it will not
provide any data in `ResponsePrepareProposal`, other than a potentially modified
transaction list. The Application may nevertheless choose to perform immediate execution even in
next-block execution mode, however same-block execution mode *requires* immediate execution.

The long term plan is for the execution model to be set in a new boolean parameter *same_block* in
`ConsensusParams`. Once this parameter is introduced,  it **must not** be changed once the
blockchain has started, unless the Application developers *really* know what they are doing.
However, modifying `ConsensusParams` structure cannot be done lightly if we are to
preserve blockchain compatibility. Therefore we need an interim solution until
soft upgrades are specified and implemented in Tendermint. This somewhat *unsafe*
solution consists in Tendermint assuming same-block execution if the Application
fills the above mentioned fields in `ResponsePrepareProposal`.

## Tendermint proposal timeout

Immediate execution requires the Application to fully execute the prepared block
before returning from `PrepareProposal`, this means that Tendermint cannot make progress
during the block execution.
This stands on Tendermint's critical path: if the Application takes a long time
executing the block, the default value of *TimeoutPropose* might not be sufficient
to accommodate the long block execution time and non-proposer nodes might time
out and prevote `nil`. The proposal, in this case, will probably be rejected and a new round will be necessary.

The Application is the best suited to provide a value for *TimeoutPropose* so
that the block execution time upon `PrepareProposal` fits well in the propose
timeout interval. Thus, the Application can adapt the value of *TimeoutPropose* at every height via
`TimeoutParams.Propose`, contained in `ConsensusParams`.

## Deterministic State-Machine Replication

[&uparrow; Back to Outline](#outline)

ABCI++ applications must implement deterministic finite-state machines to be
securely replicated by the Tendermint consensus engine. This means block execution
must be strictly deterministic: given the same
ordered set of transactions, all nodes will compute identical responses, for all
successive `FinalizeBlock` calls. This is critical because the
responses are included in the header of the next block, either via a Merkle root
or directly, so all nodes must agree on exactly what they are.

For this reason, it is recommended that application state is not exposed to any
external user or process except via the ABCI connections to a consensus engine
like Tendermint Core. The Application must only change its state based on input
from block execution (`FinalizeBlock` calls), and not through
any other kind of request. This is the only way to ensure all nodes see the same
transactions and compute the same results.

Some Applications may choose to implement immediate execution, which entails executing the blocks
that are about to be proposed (via `PrepareProposal`), and those that the Application is asked to
validate (via `ProcessProposal`). However, the state changes caused by processing those
proposed blocks must never replace the previous state until `FinalizeBlock` confirms
the block decided.

Additionally, vote extensions or the validation thereof (via `ExtendVote` or
`VerifyVoteExtension`) must *never* have side effects on the current state.
They can only be used when their data is provided in a `RequestPrepareProposal` call.

If there is some non-determinism in the state machine, consensus will eventually
fail as nodes disagree over the correct values for the block header. The
non-determinism must be fixed and the nodes restarted.

Sources of non-determinism in applications may include:

- Hardware failures
    - Cosmic rays, overheating, etc.
- Node-dependent state
    - Random numbers
    - Time
- Underspecification
    - Library version changes
    - Race conditions
    - Floating point numbers
    - JSON or protobuf serialization
    - Iterating through hash-tables/maps/dictionaries
- External Sources
    - Filesystem
    - Network calls (eg. some external REST API service)

See [#56](https://github.com/tendermint/abci/issues/56) for the original discussion.

Note that some methods (`Query, FinalizeBlock`) return non-deterministic data in the form
of `Info` and `Log` fields. The `Log` is intended for the literal output from the Application's
logger, while the `Info` is any additional info that should be returned. These are the only fields
that are not included in block header computations, so we don't need agreement
on them. All other fields in the `Response*` must be strictly deterministic.

## Events

[&uparrow; Back to Outline](#outline)

Method `FinalizeBlock` includes an `events` field at the top level in its
`Response*`, and one `events` field per transaction included in the block.
Applications may respond to this ABCI++ method with an event list for each executed
transaction, and a general event list for the block itself.
Events allow applications to associate metadata with transactions and blocks.
Events returned via `FinalizeBlock` do not impact Tendermint consensus in any way
and instead exist to power subscriptions and queries of Tendermint state.

An `Event` contains a `type` and a list of `EventAttributes`, which are key-value
string pairs denoting metadata about what happened during the method's (or transaction's)
execution. `Event` values can be used to index transactions and blocks according to what
happened during their execution.

Each event has a `type` which is meant to categorize the event for a particular
`Response*` or `Tx`. A `Response*` or `Tx` may contain multiple events with duplicate
`type` values, where each distinct entry is meant to categorize attributes for a
particular event. Every key and value in an event's attributes must be UTF-8
encoded strings along with the event type itself.

```protobuf
message Event {
  string                  type       = 1;
  repeated EventAttribute attributes = 2;
}
```

The attributes of an `Event` consist of a `key`, a `value`, and an `index` flag. The
index flag notifies the Tendermint indexer to index the attribute. The value of
the `index` flag is non-deterministic and may vary across different nodes in the network.

```protobuf
message EventAttribute {
  bytes key   = 1;
  bytes value = 2;
  bool  index = 3;  // nondeterministic
}
```

Example:

```go
 abci.ResponseFinalizeBlock{
  // ...
 Events: []abci.Event{
  {
   Type: "validator.provisions",
   Attributes: []abci.EventAttribute{
    abci.EventAttribute{Key: []byte("address"), Value: []byte("..."), Index: true},
    abci.EventAttribute{Key: []byte("amount"), Value: []byte("..."), Index: true},
    abci.EventAttribute{Key: []byte("balance"), Value: []byte("..."), Index: true},
   },
  },
  {
   Type: "validator.provisions",
   Attributes: []abci.EventAttribute{
    abci.EventAttribute{Key: []byte("address"), Value: []byte("..."), Index: true},
    abci.EventAttribute{Key: []byte("amount"), Value: []byte("..."), Index: false},
    abci.EventAttribute{Key: []byte("balance"), Value: []byte("..."), Index: false},
   },
  },
  {
   Type: "validator.slashed",
   Attributes: []abci.EventAttribute{
    abci.EventAttribute{Key: []byte("address"), Value: []byte("..."), Index: false},
    abci.EventAttribute{Key: []byte("amount"), Value: []byte("..."), Index: true},
    abci.EventAttribute{Key: []byte("reason"), Value: []byte("..."), Index: true},
   },
  },
  // ...
 },
}
```

## Evidence

[&uparrow; Back to Outline](#outline)

Tendermint's security model relies on the use of evidences of misbehavior. An evidence is an
irrefutable proof of malicious behavior by a network participant. It is the responsibility of
Tendermint to detect such malicious behavior. When malicious behavior is detected, Tendermint
will gossip evidences of misbehavior to other nodes and commit the evidences to
the chain once they are verified by a subset of validators. These evidences will then be
passed on to the Application through ABCI++. It is the responsibility of the
Application to handle evidence of misbehavior and exercise punishment.

There are two forms of evidence: Duplicate Vote and Light Client Attack. More
information can be found in either [data structures](../core/data_structures.md)
or [accountability](../light-client/accountability/).

EvidenceType has the following protobuf format:

```protobuf
enum EvidenceType {
  UNKNOWN               = 0;
  DUPLICATE_VOTE        = 1;
  LIGHT_CLIENT_ATTACK   = 2;
}
```

## Errors

[&uparrow; Back to Outline](#outline)

The `Query`, and `CheckTx` methods include a `Code` field in their `Response*`.
Field `Code` is meant to contain an application-specific response code.
A response code of `0` indicates no error.  Any other response code
indicates to Tendermint that an error occurred.

These methods also return a `Codespace` string to Tendermint. This field is
used to disambiguate `Code` values returned by different domains of the
Application. The `Codespace` is a namespace for the `Code`.

Methods `Echo`, `Info`, and `InitChain` do not return errors.
An error in any of these methods represents a critical issue that Tendermint
has no reasonable way to handle. If there is an error in one
of these methods, the Application must crash to ensure that the error is safely
handled by an operator.

Method `FinalizeBlock` is a special case. It contains a number of
`Code` and `Codespace` fields as part of type `ExecTxResult`. Each of
these codes reports errors related to the transaction it is attached to.
However, `FinalizeBlock` does not return errors at the top level, so the
same considerations on critical issues made for `Echo`, `Info`, and
`InitChain` also apply here.

The handling of non-zero response codes by Tendermint is described below.

### `CheckTx`

When Tendermint receives a `ResponseCheckTx` with a non-zero `Code`, the associated
transaction will not be added to Tendermint's mempool or it will be removed if
it is already included.

### `ExecTxResult` (as part of `FinalizeBlock`)

The `ExecTxResult` type delivers transaction results from the Application to Tendermint. When
Tendermint receives a `ResponseFinalizeBlock` containing an `ExecTxResult` with a non-zero `Code`,
the response code is logged. Past `Code` values can be queried by clients. As the transaction was
part of a decided block, the `Code` does not influence Tendermint consensus.

### `Query`

When Tendermint receives a `ResponseQuery` with a non-zero `Code`, this code is
returned directly to the client that initiated the query.
