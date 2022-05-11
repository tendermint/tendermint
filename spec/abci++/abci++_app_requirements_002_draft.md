---
order: 3
title: Application Requirements
---

# Application Requirements

This section specifies what Tendermint expects from the Application. It is structured as a set
of formal requirements that can be used for testing and verification of the Application's logic.

Let *p* and *q* be two different correct proposers in rounds *r<sub>p</sub>* and *r<sub>q</sub>*
respectively, in height *h*.
Let *s<sub>p,h-1</sub>* be *p*'s Application's state committed for height *h-1*.
Let *v<sub>p</sub>* (resp. *v<sub>q</sub>*) be the block that *p*'s (resp. *q*'s) Tendermint passes
on to the Application
via `RequestPrepareProposal` as proposer of round *r<sub>p</sub>* (resp *r<sub>q</sub>*), height *h*,
also known as the raw proposal.
Let *v'<sub>p</sub>* (resp. *v'<sub>q</sub>*) the possibly modified block *p*'s (resp. *q*'s) Application
returns via `ResponsePrepareProposal` to Tendermint, also known as the prepared proposal.

Process *p*'s prepared proposal can differ in two different rounds where *p* is the proposer.

* Requirement 1 [`PrepareProposal`, header-changes]: When the blockchain is in same-block execution mode,
  *p*'s Application provides values for the following parameters in `ResponsePrepareProposal`:
  `AppHash`, `TxResults`, `ConsensusParams`, `ValidatorUpdates`. Provided values for
  `ConsensusParams` and `ValidatorUpdates` MAY be empty to denote that the Application
  wishes to keep the current values.

Parameters `AppHash`, `TxResults`, `ConsensusParams`, and `ValidatorUpdates` are used by Tendermint to
compute various hashes in the block header that will finally be part of the proposal.

* Requirement 2 [`PrepareProposal`, no-header-changes]: When the blockchain is in next-block execution
  mode, *p*'s Application does not provide values for the following parameters in `ResponsePrepareProposal`:
  `AppHash`, `TxResults`, `ConsensusParams`, `ValidatorUpdates`.

In practical terms, Requirements 1 and 2 imply that Tendermint will (a) panic if the Application is in
same-block execution mode and *does not* provide values for
`AppHash`, `TxResults`, `ConsensusParams`, and `ValidatorUpdates`, or
(b) log an error if the Application is in next-block execution mode and *does* provide values for
`AppHash`, `TxResults`, `ConsensusParams`, or `ValidatorUpdates` (the values provided will be ignored).

* Requirement 3 [`PrepareProposal`, timeliness]: If *p*'s Application fully executes prepared blocks in
  `PrepareProposal` and the network is in a synchronous period while processes *p* and *q* are in *r<sub>p</sub>*,
  then the value of *TimeoutPropose* at *q* must be such that *q*'s propose timer does not time out
  (which would result in *q* prevoting `nil` in *r<sub>p</sub>*).

Full execution of blocks at `PrepareProposal` time stands on Tendermint's critical path. Thus,
Requirement 3 ensures the Application will set a value for `TimeoutPropose` such that the time it takes
to fully execute blocks in `PrepareProposal` does not interfere with Tendermint's propose timer.

* Requirement 4 [`PrepareProposal`, tx-size]: When *p*'s Application calls `ResponsePrepareProposal`, the
  total size in bytes of the transactions returned does not exceed `RequestPrepareProposal.max_tx_bytes`.

Busy blockchains might seek to maximize the amount of transactions included in each block. Under those conditions,
Tendermint might choose to increase the transactions passed to the Application via `RequestPrepareProposal.txs`
beyond the `RequestPrepareProposal.max_tx_bytes` limit. The idea is that, if the Application drops some of
those transactions, it can still return a transaction list whose byte size is as close to
`RequestPrepareProposal.max_tx_bytes` as possible. Thus, Requirement 4 ensures that the size in bytes of the
transaction list returned by the application will never cause the resulting block to go beyond its byte size
limit.

* Requirement 5 [`PrepareProposal`, `ProcessProposal`, coherence]: For any two correct processes *p* and *q*,
  if *q*'s Tendermint calls `RequestProcessProposal` on *v'<sub>p</sub>*,
  *q*'s Application returns Accept in `ResponseProcessProposal`.

Requirement 5 makes sure that blocks proposed by correct processes *always* pass the correct receiving process's
`ProcessProposal` check.
On the other hand, if there is a deterministic bug in `PrepareProposal` or `ProcessProposal` (or in both),
strictly speaking, this makes all processes that hit the bug byzantine. This is a problem in practice,
as very often validators are running the Application from the same codebase, so potentially *all* would
likely hit the bug at the same time. This would result in most (or all) processes prevoting `nil`, with the
serious consequences on Tendermint's liveness that this entails. Due to its criticality, Requirement 5 is a
target for extensive testing and automated verification.

* Requirement 6 [`ProcessProposal`, determinism-1]: `ProcessProposal` is a (deterministic) function of the current
  state and the block that is about to be applied. In other words, for any correct process *p*, and any arbitrary block *v'*,
  if *p*'s Tendermint calls `RequestProcessProposal` on *v'* at height *h*,
  then *p*'s Application's acceptance or rejection **exclusively** depends on *v'* and *s<sub>p,h-1</sub>*.

* Requirement 7 [`ProcessProposal`, determinism-2]: For any two correct processes *p* and *q*, and any arbitrary
  block *v'*,
  if *p*'s (resp. *q*'s) Tendermint calls `RequestProcessProposal` on *v'* at height *h*,
  then *p*'s Application accepts *v'* if and only if *q*'s Application accepts *v'*.
  Note that this requirement follows from Requirement 6 and the Agreement property of consensus.

Requirements 6 and 7 ensure that all correct processes will react in the same way to a proposed block, even
if the proposer is Byzantine. However, `ProcessProposal` may contain a bug that renders the
acceptance or rejection of the block non-deterministic, and therefore prevents processes hitting
the bug from fulfilling Requirements 6 or 7 (effectively making those processes Byzantine).
In such a scenario, Tendermint's liveness cannot be guaranteed.
Again, this is a problem in practice if most validators are running the same software, as they are likely
to hit the bug at the same point. There is currently no clear solution to help with this situation, so
the Application designers/implementors must proceed very carefully with the logic/implementation
of `ProcessProposal`. As a general rule `ProcessProposal` SHOULD always accept the block.

According to the Tendermint algorithm, a correct process can broadcast at most one precommit
message in round *r*, height *h*.
Since, as stated in the [Methods](./abci++_methods_002_draft.md#extendvote) section, `ResponseExtendVote`
is only called when Tendermint
is about to broadcast a non-`nil` precommit message, a correct process can only produce one vote extension
in round *r*, height *h*.
Let *e<sup>r</sup><sub>p</sub>* be the vote extension that the Application of a correct process *p* returns via
`ResponseExtendVote` in round *r*, height *h*.
Let *w<sup>r</sup><sub>p</sub>* be the proposed block that *p*'s Tendermint passes to the Application via `RequestExtendVote`
in round *r*, height *h*.

* Requirement 8 [`ExtendVote`, `VerifyVoteExtension`, coherence]: For any two correct processes *p* and *q*, if *q*
receives *e<sup>r</sup><sub>p</sub>*
  from *p* in height *h*, *q*'s Application returns Accept in `ResponseVerifyVoteExtension`.

Requirement 8 constrains the creation and handling of vote extensions in a similar way as Requirement 5
constrains the creation and handling of proposed blocks.
Requirement 8 ensures that extensions created by correct processes *always* pass the `VerifyVoteExtension`
checks performed by correct processes receiving those extensions.
However, if there is a (deterministic) bug in `ExtendVote` or `VerifyVoteExtension` (or in both),
we will face the same liveness issues as described for Requirement 5, as Precommit messages with invalid vote
extensions will be discarded.

* Requirement 9 [`VerifyVoteExtension`, determinism-1]: `VerifyVoteExtension` is a (deterministic) function of
  the current state, the vote extension received, and the prepared proposal that the extension refers to.
  In other words, for any correct process *p*, and any arbitrary vote extension *e*, and any arbitrary
  block *w*, if *p*'s (resp. *q*'s) Tendermint calls `RequestVerifyVoteExtension` on *e* and *w* at height *h*,
  then *p*'s Application's acceptance or rejection **exclusively** depends on *e*, *w* and *s<sub>p,h-1</sub>*.

* Requirement 10 [`VerifyVoteExtension`, determinism-2]: For any two correct processes *p* and *q*,
  and any arbitrary vote extension *e*, and any arbitrary block *w*,
  if *p*'s (resp. *q*'s) Tendermint calls `RequestVerifyVoteExtension` on *e* and *w* at height *h*,
  then *p*'s Application accepts *e* if and only if *q*'s Application accepts *e*.
  Note that this requirement follows from Requirement 9 and the Agreement property of consensus.

Requirements 9 and 10 ensure that the validation of vote extensions will be deterministic at all
correct processes.
Requirements 9 and 10 protect against arbitrary vote extension data from Byzantine processes,
in a similar way as Requirements 6 and 7 protect against arbitrary proposed blocks.
Requirements 9 and 10 can be violated by a bug inducing non-determinism in
`VerifyVoteExtension`. In this case liveness can be compromised.
Extra care should be put in the implementation of `ExtendVote` and `VerifyVoteExtension`.
As a general rule, `VerifyVoteExtension` SHOULD always accept the vote extension.

* Requirement 11 [*all*, no-side-effects]: *p*'s calls to `RequestPrepareProposal`,
  `RequestProcessProposal`, `RequestExtendVote`, and `RequestVerifyVoteExtension` at height *h* do
  not modify *s<sub>p,h-1</sub>*.

* Requirement 12 [`ExtendVote`, `FinalizeBlock`, non-dependency]: for any correct process *p*,
and any vote extension *e* that *p* received at height *h*, the computation of
*s<sub>p,h</sub>* does not depend on *e*.

The call to correct process *p*'s `RequestFinalizeBlock` at height *h*, with block *v<sub>p,h</sub>*
passed as parameter, creates state *s<sub>p,h</sub>*.
Additionally,

* in next-block execution mode, *p*'s `FinalizeBlock` creates a set of transaction results *T<sub>p,h</sub>*,
* in same-block execution mode, *p*'s `PrepareProposal` creates a set of transaction results *T<sub>p,h</sub>*
  if *p* was the proposer of *v<sub>p,h</sub>*. If *p* was not the proposer of *v<sub>p,h</sub>*,
  `ProcessProposal` creates *T<sub>p,h</sub>*. `FinalizeBlock` MAY re-create *T<sub>p,h</sub>* if it was
  removed from memory during the execution of height *h*.

* Requirement 13 [`FinalizeBlock`, determinism-1]: For any correct process *p*,
  *s<sub>p,h</sub>* exclusively depends on *s<sub>p,h-1</sub>* and *v<sub>p,h</sub>*.

* Requirement 14 [`FinalizeBlock`, determinism-2]: For any correct process *p*,
  the contents of *T<sub>p,h</sub>* exclusively depend on *s<sub>p,h-1</sub>* and *v<sub>p,h</sub>*.

Note that Requirements 13 and 14, combined with Agreement property of consensus ensure
state machine replication, i.e., the Application state evolves consistently at all correct processes.

Finally, notice that neither `PrepareProposal` nor `ExtendVote` have determinism-related
requirements associated.
Indeed, `PrepareProposal` is not required to be deterministic:

* *v'<sub>p</sub>* may depend on *v<sub>p</sub>* and *s<sub>p,h-1</sub>*, but may also depend on other values or operations.
* *v<sub>p</sub> = v<sub>q</sub> &#8655; v'<sub>p</sub> = v'<sub>q</sub>*.

Likewise, `ExtendVote` can also be non-deterministic:

* *e<sup>r</sup><sub>p</sub>* may depend on *w<sup>r</sup><sub>p</sub>* and *s<sub>p,h-1</sub>*,
  but may also depend on other values or operations.
* *w<sup>r</sup><sub>p</sub> = w<sup>r</sup><sub>q</sub> &#8655;
  e<sup>r</sup><sub>p</sub> = e<sup>r</sup><sub>q</sub>*
