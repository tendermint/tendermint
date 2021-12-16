---
order: 4
title: Tendermint's expected behavior
---

# Tendermint's expected behavior

>**TODO**: Try the regex-based explanation for this section.
>
> First summarize the current text into: all good vs all bad
> Then set out the rules (regex? grammar?)

This section describes what the Application can expect from Tendermint

The following section uses these definitions:

* We define the _optimal case_ as a run in which (a) the system behaves synchronously, and (b) there are no Byzantine processes.
  The optimal case captures the conditions that hold most of the time, but not always.
* We define the _suboptimal case_ as a run in which (a) the system behaves asynchronously in round 0, height h -- messages may be delayed,
  timeouts may be triggered --, (b) it behaves synchronously in all subsequent rounds (_r>0_), and (c) there are no Byzantine processes.
  The _suboptimal case_ captures a possible glitch in the network, or some sudden, sporadic performance issue in some validator
  (network card glitch, thrashing peak, etc.).
* We define the _general case_ as a run in which (a) the system behaves asynchronously for a number of rounds,
  (b) it behaves synchronously after some time, denoted _GST_, which is unknown to the processes,
  and (c) there may be up to _f_ Byzantine processes.

### `PrepareProposal`:  Application's expectations

Given a block height _h_, process _p_'s Tendermint calls `RequestPrepareProposal` under
the following conditions:

* _p_'s Tendermint may decide at height _h_, without calling `RequestPrepareProposal`.
  In the optimal case, this will happen often, as there will only be one proposer for height _h_
  (the one for round 0).
* _p_'s Tendermint may call `RequestPrepareProposal` with a block with no transactions, if
  _ConsensusParams_ is configured to produce empty blocks when there are outstanding transactions.
  If _ConsensusParams_ is configured to avoid empty blocks, any block passed with a call to
  `RequestPrepareProposal` will contain at lesat one transaction.
* In the optimal case, _p_'s Tendermint will call `RequestPrepareProposal` at most once,
  as there is only one round.
* In the suboptimal case, _p_'s Tendermint
    * will not call `RequestPrepareProposal` for height _h_, if _p_ is neither the proposer
      of round 0, nor round 1
    * will call `RequestPrepareProposal` once for height _h_, if _p_ is either the proposer
      of round 0, or round 1 (but not both).
    * will call `RequestPrepareProposal` twice for height _h_, in the unlikely case that
      _p_ is the proposer of round 0, and round 1.
* In the general case, _p_'s Tendermint will potentially span many rounds. So, it will call
  `RequestPrepareProposal` a number of times which is impossible to predict. Thus, the
  Application will to return numerous _amended_ blocks via `ResponsePrepareProposal`.

  If the application is fully executing the blocks it returns via `ResponsePrepareProposal`,
  it should be careful about its usage of resources (memory, disk, etc.) as the number
  of `PrepareProposal` for a particular height is not bounded.

If `PrepareProposal` is called more than once for a height _h_, the Application _is_ allowed
to return different blocks each time.

### `ProcessProposal`:  Application's expectations

Given a block height _h_, process _p_'s Tendermint calls `RequestProcessProposal` depending on the case:

* In the optimal case, all Tendermint processes decide in round _r=0_. Let's call _v_ the block proposed by the proposer of round _r=0_, height _h_.
  Process _p_'s Application will receive
      * exactly one call to `RequestPrepareProposal` if _p_ is the proposer of round 0. If that is the case, _p_ will return _v_ in its call to
        `ResponsePrepareProposal`
      * exactly one call to `RequestProcessProposal` for block height _h_ containing _v_.

  In addition, the Application may execute _v_ as part of handling the `RequestProcessProposal` call, keep the resulting state as a candidate state,
  and apply it when the call to `RequestFinalizeBlock` for _h_ confirms that _v_ is indeed the block decided in height _h_.
* In the suboptimal case, Tendermint processes may decide in round _r=0_ or in round _r=1_.
  Therefore, the Application of a process _q_ may receive zero, one or two calls to `RequestProcessProposal`, depending on
  whether _q_ is the proposer of round 0 or 1 of height _h_.
  Likewise, the Application of a process _q_ may receive one or two calls to `RequestProcessProposal`
  with two different values _v0_ and _v1_ while in height _h_.
  * There is no way for the Application to predict whether proposed block _v0_ or _v1_ will be decided before `RequestFinalizeBlock` is called.
  * The Application may choose to execute either (or both) proposed block as part of handling the `RequestProcessProposal`
    call and keep the resulting state as a _candidate state_.
    At decision time (i.e. when Tendermint calls `RequestFinalizeBlock`), if the block decided corresponds to one of the candidate states,
    it can be committed, otherwise the Application will have to execute the block at this point.
* In the general case, the round in which a Tendermint processes _p_ will decide cannot be forecast.
  The number of time _p_'s Tendermint may call `RequestProcessProposal` with different proposed blocks for a given height is *unbounded*.
  As a result, the Application may need to deal with an unbounded number of different proposals for a given height,
  and also an unbounded number of candidate states if it is fully executing the proposed blocks upon `PrepareProposal` or `ProcessProposal`.
  In order protects the processes' stability (memory, CPU), the Application has to:
    * Be ready to discard candidate states if they become too many. In other words, the set of candidate states should be managed like a cache.
    * Be able to execute the blocks upon `FinalizeBlock` if the block decided was one whose candidate state was discarded.

### `ExtendVote`:  expectations from the Application

>**TODO** (in different rounds). [Finish first discussion above]

--

If `ProcessProposal`'s outcome is _Reject_ for some proposed block. Tendermint guarantees that the block will not be the decision.

--

The validity of every transaction in a block (from the App's point of view), as well as the hashes in its header can be guaranteed if:

* `ProcessProposal` *synchronously* handles every proposed block as though Tendermint had already decided on it.
* All the properties of `ProcessProposal` and `FinalizeBlock` mentioned above hold.

## [Points to discuss further]

### Byzantine proposer

>**TODO** [From Josef] We should understand the influence of equivocation on proposals in ABCI++
>
>N.B.#1: If Byzantine proposer proposes both A and B in the same round, today we might only receive A (proposal) and not B (due to p2p implementation)
>
>Sergio: some thoughts:
>
>[Thought 1] If Tendermint delivers all proposals from a Byzantine proposer _p_ in _h_ and _r_ to the App, we're vulnerable to DoS attacks,
>as _p_ can send as many as it wishes:
>
>* Assuming N.B.#1 above gets "fixed" (A and B get delivered by p2p)
>* Assuming the App fully executes the proposed block at _ProcessProposal_ time
>
>So, whenever N.B.#1 is "fixed" at p2p level, we need to ensure that only the the first proposal in round _r_, height _h_
>gets delivered to the Application. Any subsequent proposal for that round should just be used to submit evidence, but not given to the App.
>
>[Thought 2] In terms of the consensus's safety properties, as far as pure equivocation on proposals is concerned,
>I think we are OK as long as _f<n/3_.

### `ProcessProposal`'s timeout (a.k.a. Zarko's Github comment)

>**TODO** (to discuss): `PrepareProposal` is called synchronously. `ProcessProposal` may also want to fully process the block synchronously.
>However, they stand on Tendermint's critical path, so the Tendermint's Propose timeout needs to accomodate that.
>
>Idea: Make propose timestamp (currently hardcoded to 3 secs in the Tendermint Go implementation) part of ConsensusParams,
>so the App can adjust it with its knowledge of the time may take to prepare/process the proposal.

# Failure modes

>**TODO** Is it worth explaining the failure modes? Since we're going for halt, and can't configure them.

# Application modes

[This is a sketch ATM]

Mode 1: Simple mode (equivalent to ABCI).

**TODO**: Define _candidate block_

Definition: "candidate state" is the App state resulting from the optimistic exectution of a block that is not decided yet by consensus. An application managing candidate states needs to be able to discard them and recover the previous state

* PrepareProposal: Set `ResponsePrepareProposal.modified` to false and return
* ProcessProposal: keep the block data in memory (indexed by app hash) as _candidate block_ and return Accept
* ExtendVote: return empty byte array
* VerifyVoteExtension: if the byte array is empty, return Accept; else, return Reject
* Finalize block: look up the block by the app hash and fully execute it. Discard all other candidate blocks

Mode 2: Basic checks on proposals.

* PrepareProposal: Go through the transactions, apply basic app-dependent checks based on the block and last committed state. Remove/reorder invalid transactions in the block
* ProcessProposal: Same as PrepareProposal; return Reject if invalid transactions are detected. If Accept, keep the block data in memory (indexed by app hash) as _candidate block_.
* ExtendVote: return empty byte array
* VerifyVoteExtension: if the byte array is empty, return Accept; else, return Reject
* Finalize block: look up the block by the app hash and fully execute it. Discard all other candidate blocks

Mode 3: Full check of proposals. Optimistic (or immediate) execution. No candidate state management.

* PrepareProposal: fully execute the block, but discard the resulting state. Remove/reorder invalid transactions in the block.
* ProcessProposal: Same as PrepareProposal. Return Reject if invalid transactions are detected. If Accept, keep the block data in memory (indexed by app hash) as _candidate block_.
* ExtendVote: return empty byte array
* VerifyVoteExtension: if the byte array is empty, return Accept; else, return Reject
* Finalize block: look up the block by the app hash and fully execute it. Discard all other candidate blocks

This mode guarantees that no invalid transactions will ever make it into a committed block

Mode 4: Mode 3 + candidate state management.

* PrepareProposal: First Remove/reorder invalid transactions in the block. If _v_ is not in the set of candidate states, fully execute the block, add the resulting state as candidate state for value _v_, height _h_.
* ProcessProposal: Same as PrepareProposal. Return Reject if invalid transactions are detected
* ExtendVote: return empty byte array
* VerifyVoteExtension: if the byte array is empty, return Accept; else, return Reject
* Finalize block: commit candidate state corresponding to _v_, discard all other candidate states.

Mode 5: Mode 4 + AppHash for heigh _h_ is in _h_'s header

>**TODO**  Mode 6: With vote extensions (?)

>**TODO**: Explain the two workflows discussed with Callum: App hash on N+1 vs App hash on N. How to capture it in ABCI++ ?
