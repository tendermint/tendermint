---
order: 3
title: Application Requirements
---

# Application Requirements

This section specifies what Tendermint expects from the Application. It is structured as a set
of formal requirement that can be used for testing and verification of the Application's logic.

Let $p$ and $q$ be two different correct proposers in rounds $r_p$ and $r_q$ respectively, in height $h$.
Let $s_{p,h-1}$ be $p$'s Application's state committed for height $h-1$.
Let $v_p$ (resp. $v_q$) be the block that $p$'s (resp. $q$'s) Tendermint passes on to the Application
via `RequestPrepareProposal` as proposer of round $r_p$ (resp $r_q$), height $h$, also known as the
raw proposal.
Let $v'_p$ (resp. $v'_q$) the possibly modified block $p$'s (resp. $q$'s) Application returns via
`ResponsePrepareProposal` to Tendermint, also known as the prepared proposal.

Process $p$'s prepared proposal can differ in two different rounds where $p$ is the proposer.

* Requirement 1 [`PrepareProposal`, header-changes] When the blockchain is in same-block execution mode,
  $p$'s Application provides values for the following parameters in `ResponsePrepareProposal`:
  _AppHash_, _TxResults_, _ConsensusParams_, _ValidatorUpdates_. Provided values for
  _ConsensusParams_ and _ValidatorUpdates_ MAY be empty to denote that the Application
  wishes to keep the current values.

Parameters _AppHash_, _TxResults_, _ConsensusParams_, and _ValidatorUpdates_ are used by Tendermint to
compute various hashes in the block header that will finally be part of the proposal.

* Requirement 2 [`PrepareProposal`, no-header-changes] When the blockchain is in next-block execution
  mode, $p$'s Application does not provide values for the following parameters in `ResponsePrepareProposal`:
  _AppHash_, _TxResults_, _ConsensusParams_, _ValidatorUpdates_.

In practical terms, Requirements 1 and 2 imply that Tendermint will (a) panic if the Application is in
same-block execution mode and _does_ _not_ provide values for
_AppHash_, _TxResults_, _ConsensusParams_, and _ValidatorUpdates_, or
(b) log an error if the Application is in next-block execution mode and _does_ provide values for
_AppHash_, _TxResults_, _ConsensusParams_, or _ValidatorUpdates_ (the values provided will be ignored).

* Requirement 3 [`PrepareProposal`, timeliness] If $p$'s Application fully executes prepared blocks in
  `PrepareProposal` and the network is in a synchronous period while processes $p$ and $q$ are in $r_p$, then
  the value of *TimeoutPropose* at $q$ must be such that $q$'s propose timer does not time out
  (which would result in $q$ prevoting *nil* in $r_p$).

Full execution of blocks at `PrepareProposal` time stands on Tendermint's critical path. Thus,
Requirement 3 ensures the Application will set a value for _TimeoutPropose_ such that the time it takes
to fully execute blocks in `PrepareProposal` does not interfere with Tendermint's propose timer.

* Requirement 4 [`PrepareProposal`, `ProcessProposal`, coherence]: For any two correct processes $p$ and $q$,
  if $q$'s Tendermint calls `RequestProcessProposal` on $v'_p$,
  $q$'s Application returns Accept in `ResponseProcessProposal`.

Requirement 4 makes sure that blocks proposed by correct processes _always_ pass the correct receiving process's
`ProcessProposal` check.
On the other hand, if there is a deterministic bug in `PrepareProposal` or `ProcessProposal` (or in both),
strictly speaking, this makes all processes that hit the bug byzantine. This is a problem in practice,
as very often validators are running the Application from the same codebase, so potentially _all_ would
likely hit the bug at the same time. This would result in most (or all) processes prevoting `nil`, with the
serious consequences on Tendermint's liveness that this entails. Due to its criticality, Requirement 4 is a
target for extensive testing and automated verification.

* Requirement 5 [`ProcessProposal`, determinism-1]: `ProcessProposal` is a (deterministic) function of the current
  state and the block that is about to be applied. In other words, for any correct process $p$, and any arbitrary block $v'$,
  if $p$'s Tendermint calls `RequestProcessProposal` on $v'$ at height $h$,
  then $p$'s Application's acceptance or rejection **exclusively** depends on $v'$ and $s_{p,h-1}$.

* Requirement 6 [`ProcessProposal`, determinism-2]: For any two correct processes $p$ and $q$, and any arbitrary block $v'$,
  if $p$'s (resp. $q$'s) Tendermint calls `RequestProcessProposal` on $v'$ at height $h$,
  then $p$'s Application accepts $v'$ if and only if $q$'s Application accepts $v'$.
  Note that this requirement follows from Requirement 5 and the Agreement property of consensus.

Requirements 5 and 6 ensure that all correct processes will react in the same way to a proposed block, even
if the proposer is Byzantine. However, `ProcessProposal` may contain a bug that renders the
acceptance or rejection of the block non-deterministic, and therefore prevents processes hitting
the bug from fulfilling Requirements 5 or 6 (effectively making those processes Byzantine).
In such a scenario, Tendermint's liveness cannot be guaranteed.
Again, this is a problem in practice if most validators are running the same software, as they are likely
to hit the bug at the same point. There is currently no clear solution to help with this situation, so
the Application designers/implementors must proceed very carefully with the logic/implementation
of `ProcessProposal`. As a general rule `ProcessProposal` _should_ always accept the block.

According to the Tendermint algorithm, a correct process can broadcast at most one precommit message in round $r$, height $h$.
Since, as stated in the [Description](#description) section, `ResponseExtendVote` is only called when Tendermint
is about to broadcast a non-`nil` precommit message, a correct process can only produce one vote extension in round $r$, height $h$.
Let $e^r_p$ be the vote extension that the Application of a correct process $p$ returns via `ResponseExtendVote` in round $r$, height $h$.
Let $w^r_p$ be the proposed block that $p$'s Tendermint passes to the Application via `RequestExtendVote` in round $r$, height $h$.

* Requirement 7 [`ExtendVote`, `VerifyVoteExtension`, coherence]: For any two correct processes $p$ and $q$, if $q$ receives $e^r_p$
  from $p$ in height $h$, $q$'s Application returns Accept in `ResponseVerifyVoteExtension`.

Requirement 7 constrains the creation and handling of vote extensions in a similar way as Requirement 4
contrains the creation and handling of proposed blocks.
Requirement 7 ensures that extensions created by correct processes _always_ pass the `VerifyVoteExtension`
checks performed by correct processes receiving those extensions.
However, if there is a (deterministic) bug in `ExtendVote` or `VerifyVoteExtension` (or in both),
we will face the same liveness issues as described for Requirement 4, as Precommit messages with invalid vote
extensions will be discarded.

* Requirement 8 [`VerifyVoteExtension`, determinism-1]: `VerifyVoteExtension` is a (deterministic) function of
  the current state, the vote extension received, and the prepared proposal that the extension refers to.
  In other words, for any correct process $p$, and any arbitrary vote extension $e$, and any arbitrary
  block $w$, if $p$'s (resp. $q$'s) Tendermint calls `RequestVerifyVoteExtension` on $e$ and $w$ at height $h$,
  then $p$'s Application's acceptance or rejection **exclusively** depends on $e$, $w$ and $s_{p,h-1}$.

* Requirement 9 [`VerifyVoteExtension`, determinism-2]: For any two correct processes $p$ and $q$,
  and any arbitrary vote extension $e$, and any arbitrary block $w$,
  if $p$'s (resp. $q$'s) Tendermint calls `RequestVerifyVoteExtension` on $e$ and $w$ at height $h$,
  then $p$'s Application accepts $e$ if and only if $q$'s Application accepts $e$.
  Note that this requirement follows from Requirement 8 and the Agreement property of consensus.

Requirements 8 and 9 ensure that the validation of vote extensions will be deterministic at all
correct processes.
Requirements 8 and 9 protect against arbitrary vote extension data from Byzantine processes
similarly to Requirements 5 and 6 and proposed blocks.
Requirements 8 and 9 can be violated by a bug inducing non-determinism in
`VerifyVoteExtension`. In this case liveness can be compromised.
Extra care should be put in the implementation of `ExtendVote` and `VerifyVoteExtension` and,
as a general rule, `VerifyVoteExtension` _should_ always accept the vote extension.

* Requirement 10 [_all_, no-side-effects]: $p$'s calls to `RequestPrepareProposal`,
  `RequestProcessProposal`, `RequestExtendVote`, and `RequestVerifyVoteExtension` at height $h$ do
  not modify $s_{p,h-1}$.

* Requirement 11 [`ExtendVote`, `FinalizeBlock`, non-dependency]: for any correct process $p$,
and any vote extension $e$ that $p$ received at height $h$, the computation of
$s_{p,h}$ does not depend on $e$.

The call to correct process $p$'s `RequestFinalizeBlock` at height $h$, with block $v_{p,h}$
passed as parameter, creates state $s_{p,h}$.
Additionally,

* in next-block execution mode, $p$'s `FinalizeBlock` creates a set of transaction results $T_{p,h}$,
* in same-block execution mode, $p$'s `PrepareProposal` creates a set of transaction results $T_{p,h}$
  if $p$ was the proposer of $v_{p,h}$, otherwise `FinalizeBlock` creates $T_{p,h}$.

>**TODO** I have left out all the "events" as they don't have any impact in safety or liveness
>(same for consensus params, and validator set)

* Requirement 12 [`FinalizeBlock`, determinism-1]: For any correct process $p$,
  $s_{p,h}$ exclusively depends on $s_{p,h-1}$ and $v_{p,h}$.

* Requirement 13 [`FinalizeBlock`, determinism-2]: For any correct process $p$,
  the contents of $T_{p,h}$ exclusively depend on $s_{p,h-1}$ and $v_{p,h}$.

Note that Requirements 12 and 13, combined with Agreement property of consensus ensure
the Application state evolves consistently at all correct processes.

Finally, notice that neither `PrepareProposal` nor `ExtendVote` have determinism-related
requirements associated.
Indeed, `PrepareProposal` is not required to be deterministic:

* $v'_p$ may depend on $v_p$ and $s_{p,h-1}$, but may also depend on other values or operations.
* $v_p = v_q \nRightarrow v'_p = v'_q$.

Likewise, `ExtendVote` can also be non-deterministic:

* $e^r_p$ may depend on $w^r_p$ and $s_{p,h-1}$, but may also depend on other values or operations.
* $w^r_p = w^r_q \nRightarrow e^r_p = e^r_q$
