---
order: 4
title: Tendermint's expected behavior
---

# Tendermint's expected behavior

## Valid method call sequences

This section describes what the Application can expect from Tendermint.

The Tendermint consensus algorithm is designed to protect safety under any network conditions, as long as
less than 1/3 of validators' voting power is byzantine. Most of the time, though, the network will behave
synchronously, no process will fall behind, and there will be no byzantine process. The following describes
what will happen during a block height _h_ in these frequent, benign conditions:

* Tendermint will decide in round 0, for height _h_;
* `PrepareProposal` will be called exactly once at the proposer process of round 0, height _h_;
* `ProcessProposal` will be called exactly once at all processes, and
  will return _accept_ in its `Response*`;
* `ExtendVote` will be called exactly once at all processes;
* `VerifyVoteExtension` will be called exactly _n-1_ times at each validator process, where _n_ is
  the number of validators, and will always return _accept_ in its `Response*`;
* `FinalizeBlock` will be called exactly once at all processes, conveying the same prepared
  block that all calls to `PrepareProposal` and `ProcessProposal` had previously reported for
  height _h_; and
* `Commit` will finally be called exactly once at all processes at the end of height _h_.

However, the Application logic must be ready to cope with any possible run of Tendermint for a given
height, including bad periods (byzantine proposers, network being asynchronous).
In these cases, the sequence of calls to ABCI++ methods may not be so straighforward, but
the Application should still be able to handle them, e.g., without crashing.
The purpose of this section is to define what these sequences look like an a precise way.

As mentioned in the [Basic Concepts](./abci%2B%2B_basic_concepts.md) section, Tendermint
acts as a client of ABCI++ and the Application acts as a server. Thus, it is up to Tendermint to
determine when and in which order the different ABCI++ methods will be called. A well-written
Application design should consider _any_ of these possible sequences.

The following grammar, written in case-sensitive Augmented Backus–Naur form (ABNF, specified
in [IETF rfc7405](https://datatracker.ietf.org/doc/html/rfc7405)), specifies all possible
sequences of calls to ABCI++, taken by a correct process, across all heights from the genesis block,
including recovery runs, from the point of view of the Application.

```abnf
start               = clean-start / recovery

clean-start         = init-chain [state-sync] consensus-exec
state-sync          = *state-sync-attempt success-sync info
state-sync-attempt  = offer-snapshot *apply-chunk
success-sync        = offer-snapshot 1*apply-chunk

recovery            = info consensus-exec

consensus-exec      = (inf)consensus-height
consensus-height    = *consensus-round decide commit
consensus-round     = proposer / non-proposer

proposer            = *got-vote prepare-proposal *got-vote process-proposal [extend]
extend              = *got-vote extend-vote *got-vote
non-proposer        = *got-vote [process-proposal] [extend]

init-chain          = %s"<InitChain>"
offer-snapshot      = %s"<OfferSnapshot>"
apply-chunk         = %s"<ApplySnapshotChunk>"
info                = %s"<Info>"
prepare-proposal    = %s"<PrepareProposal>"
process-proposal    = %s"<ProcessProposal>"
extend-vote         = %s"<ExtendVote>"
got-vote            = %s"<VerifyVoteExtension>"
decide              = %s"<FinalizeBlock>"
commit              = %s"<Commit>"
```

We have kept some ABCI methods out of the grammar, in order to keep it as clear and concise as possible.
A common reason for keeping all these methods out is that they all can be called at any point in a sequence defined
by the grammar above. Other reasons depend on the method in question:

* `Echo` and `Flush` are only used for debugging purposes. Further, their handling by the Application should be trivial.
* `CheckTx` is detached from the main method call sequence that drives block execution.
* `Query` provides read-only access to the current Application state, so handling it should also be independent from
  block execution.
* Similarly, `ListSnapshots` and `LoadSnapshotChunk` provide read-only access to the Application's previously created
  snapshots (if any), and help populate the parameters of `OfferSnapshot` and `ApplySnapshotChunk` at a process performing
  state-sync while bootstrapping. Unlike `ListSnapshots` and `LoadSnapshotChunk`, both `OfferSnapshot`
  and `ApplySnapshotChunk` _are_ included in the grammar.

Finally, method `Info` is a special case. The method's purpose is three-fold, it can be used

1. as part of handling an RPC call from an external client,
2. as a handshake between Tendermint and the Application upon recovery to check whether any blocks need
   to be replayed, and
3. at the end of _state-sync_ to verify that the correct state has been reached.

We have left `Info`'s first purpose out of the grammar for the same reasons as all the others: it can happen
at any time, and has nothing to do with the block execution sequence. The second and third purposes, on the other
hand, are present in the grammar.

Let us now examine the grammar line by line, providing further details.

* When a process starts, it may do so for the first time or after a crash (it is recovering).

>```abnf
>start               = clean-start / recovery
>```

* If the process is starting from scratch, Tendermint first calls `InitChain`, then it may optionally
  start a _state-sync_ mechanism to catch up with other processes. Finally, it enters normal
  consensus execution.

>```abnf
>clean-start         = init-chain [state-sync] consensus-exec
>```

* In _state-sync_ mode, Tendermint makes one or more attempts at synchronizing the Application's state.
  At the beginning of each attempt, it offers the Application a snapshot found at another process.
  If the Application accepts the snapshot, a sequence of calls to `ApplySnapshotChunk` method follow
  to provide the Application with all the snapshots needed, in order to reconstruct the state locally.
  A successful attempt must provide at least one chunk via `ApplySnapshotChunk`.
  At the end of a successful attempt, Tendermint calls `Info` to make sure the recontructed state's
  _AppHash_ matches the one in the block header at the corresponding height.

>```abnf
>state-sync          = *state-sync-attempt success-sync info
>state-sync-attempt  = offer-snapshot *apply-chunk
>success-sync        = offer-snapshot 1*apply-chunk
>```

* In recovery mode, Tendermint first calls `Info` to know from which height it needs to replay decisions
  to the Application. After this, Tendermint enters nomal consensus execution.

>```abnf
>recovery            = info consensus-exec
>```

* The non-terminal `consensus-exec` is a key point in this grammar. It is an infinite sequence of
  consensus heights. The grammar is thus an
  [omega-grammar](https://dl.acm.org/doi/10.5555/2361476.2361481), since it produces infinite
  sequences of terminals (i.e., the API calls).

>```abnf
>consensus-exec      = (inf)consensus-height
>```

* A consensus height consists of zero or more rounds before deciding and executing via a call to
  `FinalizeBlock`, followed by a call to `Commit`. In each round, the sequence of method calls
  depends on whether the local process is the proposer or not. Note that, if a height contains zero
  rounds, this means the process is replaying an already decided value (catch-up mode).

>```abnf
>consensus-height    = *consensus-round decide commit
>consensus-round     = proposer / non-proposer
>```

* For every round, if the local process is the proposer of the current round, Tendermint starts by
  calling `PrepareProposal`, followed by `ProcessProposal`. Then, optionally, the Application is
  asked to extend its vote for that round. Calls to `VerifyVoteExtension` can come at any time: the
  local process may be slightly late in the current round, or votes may come from a future round
  of this height.

>```abnf
>proposer            = *got-vote prepare-proposal *got-vote process-proposal [extend]
>extend              = *got-vote extend-vote *got-vote
>```

* Also for every round, if the local process is _not_ the proposer of the current round, Tendermint
  will call `ProcessProposal` at most once. At most one call to `ExtendVote` may occur only after
  `ProcessProposal` is called. A number of calls to `VerifyVoteExtension` can occur in any order
  with respect to `ProcessProposal` and `ExtendVote` throughout the round. The reasons are the same
  as above, namely, the process running slightly late in the current round, or votes from future
  rounds of this height received.

>```abnf
>non-proposer        = *got-vote [process-proposal] [extend]
>```

* Finally, the grammar describes all its terminal symbols, which denote the different ABCI++ method calls that
  may appear in a sequence.

>```abnf
>init-chain          = %s"<InitChain>"
>offer-snapshot      = %s"<OfferSnapshot>"
>apply-chunk         = %s"<ApplySnapshotChunk>"
>info                = %s"<Info>"
>prepare-proposal    = %s"<PrepareProposal>"
>process-proposal    = %s"<ProcessProposal>"
>extend-vote         = %s"<ExtendVote>"
>got-vote            = %s"<VerifyVoteExtension>"
>decide              = %s"<FinalizeBlock>"
>commit              = %s"<Commit>"
>```

## Adapting existing Applications that use ABCI

In some cases, an existing Application using the legacy ABCI may need to be adapted to work with ABCI++
with as minimal changes as possible. In this case, of course, ABCI++ will not provide any advange with respect
to the existing implementation, but will keep the same guarantees already provided by ABCI.
Here is how ABCI++ methods should be implemented.

First of all, all the methods that did not change from ABCI to ABCI++, namely `Echo`, `Flush`, `Info`, `InitChain`,
`Query`, `CheckTx`, `ListSnapshots`, `LoadSnapshotChunk`, `OfferSnapshot`, and `ApplySnapshotChunk`, do not need
to undergo any changes in their implementation.

As for the new methods:

* `PrepareProposal` must create a list of [TxRecord](./abci++_methods.md#txrecord) each containing
  a transaction passed in `RequestPrepareProposal.txs`, in the same other. The field `action` must
  be set to `UNMODIFIED` for all [TxRecord](./abci++_methods.md#txrecord) elements in the list.
  The Application must check whether the size of all transactions exceeds the byte limit
  (`RequestPrepareProposal.max_tx_bytes`). If so, the Application must remove transactions at the
  end of the list until the total byte size is at or below the limit.
* `ProcessProposal` must set `ResponseProcessProposal.accept` to _true_ and return.
* `ExtendVote` is to set `ResponseExtendVote.extension` to an empty byte array and return.
* `VerifyVoteExtension` must set `ResponseVerifyVoteExtension.accept` to _true_ if the extension is
  an empty byte array and _false_ otherwise, then return.
* `FinalizeBlock` is to coalesce the implementation of methods `BeginBlock`, `DeliverTx`, and
  `EndBlock`. Legacy applications looking to reuse old code that implemented `DeliverTx` should
  wrap the legacy `DeliverTx` logic in a loop that executes one transaction iteration per
  transaction in `RequestFinalizeBlock.tx`.

Finally, `Commit`, which is kept in ABCI++, no longer returns the `AppHash`. It is now up to
`FinalizeBlock` to do so. Thus, a slight refactoring of the old `Commit` implementation will be
needed to move the return of `AppHash` to `FinalizeBlock`.
