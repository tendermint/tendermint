---
order: 1
title: New Methods
---

# New Methods

## Description

### PrepareProposal

#### Parameters and Return values

TODO

#### When does Tendermint call it?

When a validator _p_ enters Tendermint consensus round _r_, height _h_, in which _p_ is the proposer:

1. _p_'s Tendermint collects outstanding transactions from the mempool (TODO: should we limit size & gas?).
2. _p_'s Tendermint creates a block header.
3. _p_'s Tendermint calls `PrepareProposal` with the newly created block. The call is synchronous (i.e., Tendermint's execution will block until the Application returns).
4. The Application checks the block (the header and transactions). It can also:
   * add/remove transactions
   * modify the header hashes
   * if the block is modified, the Application sets it in the return parameters
5. The Application signals _Accept_ or _Reject_ in `PrepareProposal`'s return values
   * TODO: Decide if this kind if Accept/Reject is wanted/covered by impl (maybe a panic here makes more sense?)
   * If _Reject_, the proposed block --along with any modification-- is discarded. Tendermint interprets that there is nothing to be proposed for consensus at the moment and _p_ won't send any proposal message in round _r_, height _h_.
   * If _Accept_, _p_'s Tendermint uses the modified block as _p_'s proposal in round _r_, height _h_.

### ProcessProposal

#### Parameters and Return values

TODO

#### When does Tendermint call it?

When a validator _p_ enters Tendermint consensus round _r_, height _h_, in which _q_ is the proposer (possibly _p_ = _q_):

1. _p_ sets up timer `ProposeTimeout`.
2. If _p_ is the proposer, execute steps 1-6 in _PrepareProposal_.
3. Upon reception of `Proposal` message from _q_ for round _r_, height _h_, _p_'s Tendermint calls `ProcessProposal` with the newly received proposal. The call is synchronous.
4. The Application checks/processes the proposed block, which is read-only, and returns _Accept_ or _Reject_.
   * Depending on the Application's needs, it may return from `ProcessProposal`
     * either after it has completely processed the block (the simpler case),
     * or immediately (after doing some basic checks), and process the block asynchronously. In this case the Application won't be able to reject the block, or force prevote/precommit `nil` afterwards.
5. If the returned value is
     * _Reject_, Tendermint will prevote `nil` for the proposal in round _r_, height _h_ (this can be interpreted as _valid(v)_ returning false in the white paper's pseudocode)
     * _Accept_, Tendermint will follow the normal algorithm to prevote on the proposal in _r_, height _h_

### ExtendVote

#### Parameters and Return values

TODO

#### When does Tendermint call it?

When a validator _p_ is in Tendermint consensus round _r_, height _h_, state _prevote_, and the other conditions prescribed by the consensus algorithm for sending a _Precommit_ message are fulfilled:

1. _p_'s Terndermint updates the consensus state as prescriped by the algorithm (_validValue_, _lockedValue_, etc.)
2. _p_'s Tendermint calls `ExtendVote` with the value that is about to be sent as precommit message. The call is synchronous.
3. The Application returns an array of bytes, `precommit_extension`, which is not interpreted by Tendermint.
4. _p_'s Tendermint includes `precommit_extension` as a new field in the `precommit` message.
5. _p_'s Tendermint broadcasts the `precommit` message.

N.B: In Github discussions, I saw Dev becoming convinced that extending `nil` precommits is quite useless in practice. So I'd simplify the text above to only consider non-`nil` precommits. As it is now, the text is ambiguous when referrring to the "conditions".

### VerifyVoteExtension

#### Parameters and Return values

TODO

#### When does Tendermint call it?

When a validator _p_ is in Tendermint consensus round _r_, height _h_, state _prevote_, and _p_ receives a `Precommit` message for round _r_, height _h_ from _q_:

1. _p_'s Tendermint calls `VerifyVoteExtension`
2. The Application returns Accept or Reject -- TODO: Can we really reject here? Discuss liveness problems
3. _p_'s Tendermint deems the `precommit` message invalid if the Application returned Reject

### FinalizeBlock

#### Parameters and Return values

TODO

#### When is it called?

TODO: Quite simple. Equivalent to BeginBlock, [DeliverTx]*, EndBlock, Commit (?)

TODO: To discuss: An optimization could be the App setting a config to not send the whole block, just the hash or id

### [Unclear/controversial aspects]

#### Early evidence collecting (output of ProcessProposal in Dev's pseudocode)

Need to understand the use cases, in order to come up with the properties

#### Dev's v4 changes (a.k.a. _multithreaded ABCI++_)

Comments on the "fork-join" mechanism

* So far, the preferred operation mode for ABCI has been synchronous. The most notable exception is: checkTx (deliverTx is not really async). However, there are no strong properties to be expected from checkTx, relating the transactions it checks and their validity when they make it into a block.
* The "join" part has two aims in the pseudocode (v4)
  * early collection and treatment of evidences. See above
  * influencing the pre-commit to be sent (whether _id(v)_ or `nil`)
    * TODO: Discuss strong liveness implications. Ask Josef: should I write something here and then discuss, or the other way around? (the latter seems more efficient)

#### Separation of `VerifyHeader` and `ProcessProposal`

To discuss. Starting point: it only makes sense if using the "Fork--Join" mechanism

##### [From Josef] We should understand the influence equivocation on proposals in ABCI++

TODO

#### Byzantine proposer

If Byzantine proposer proposes both A and B in the same round, today we might only receive A (proposal) and not B

#### `ProcessProposal`'s timeout (a.k.a. Zarko's Github comment)

TODO (to discuss): `PrepareProposal` must be synchronous, `ProcessProposal` may also want to fully process the block synchronously. However, they stand on the Tendermint's critical path, so the propose timeout needs to acknowledge that.

Idea: Make propose timestamp (current hardcoded to 3 secs in the implementation) part of ConsensusParams, so the App can adjust it with its knowledge of the time it takes

## Properties

[These are a sketch ATM]

### From Terndermint's point of view

`PrepareProposal`'s outcome (i.e., modified block and _Accept_ or _Reject_)

* it does not need to be deterministic

--

Discuss with Josef `PrepareProposal` in different rounds. Algo in paper allows different. Should we?

--

`ProcessProposal`'s outcome (_Accept_ or _Reject_):

* MUST be deterministic
* MUST exclusively depend on the proposal's contents, and the Application's state that resulted from the last committed block

If this doesn't hold, Tendermint might not terminate (N.B: I'm not sure this is the case -- to discuss)

--

If `ProcessProposal`'s outcome is _Reject_ for some proposed block. Tendermint guarantees that the block will not be the decision.

TODO: This has implications on the termination mentioned above.

--

The validity of every transaction in a block (from the App's point of view), as well as the hashes in its header can be guaranteed if:

* `ProcessProposal` *synchronously* handles every proposed block as though Tendermint had already decided on it.
* All the properties of `ProcessProposal` mentioned above hold.

--

What are the properties we can offer to ExtendVote/VerifyVoteExtension ?

* Can be many, a different one per round
* In the same round, they are many as well: one per validator
* TODO: ask Dev: what properties can/must the App guarantee when replying to ExtendVote (determinism?, same output?, no guarantees?)
  * The guarantees ABCI++ can offer depend on those

--

The two workflows discussed with Callum: App hash on N+1 vs App hash on N. How to capture it in ABCI++ ?

### From the Application's point of view

The following sesctions use these definitions

* We define the _common case_ as a run in which (a) the system behaves synchronously, and (b) there are no Byzantine processes. The common case captures the conditions that hold most of the time, but not always.
* We define the _suboptimal case_ as a run in which (a) the system behaves asynchronously in round 0 -- messages may be delayed, timeouts may be triggered --, and (b) it behaves synchronously in all subsequent rounds (_r>=1_), (b) there are no Byzantine processes. The _suboptimal case_ captures a possible glitch in the network, or some sudden, sporadic performance issue in some validator.
* We define the _worst case_ as [TODO]

#### `ProcessProposal`:  expectations from the Application

Given a block height _h_, process _p_'s Tendermint calls API function `ProcessProposal` every time it receives a proposed block from the proposer in round _r_, for _r_ in 0, 1, 2, ... (see above).

* In the common case, all Tendermint processes decide in round _r=0_. Let's call _v_ the block proposed by the proposer of round _r=0_, height _h_. Process _p_'s Application will receive exactly one call to `ProcessProposal` for block height _h_ containing _v_. The Application may execute _v_ as part of handling the `ProcessProposal` call, keep the resulting state, and apply it when the call to `FinalizeBlock` for _h_ confirms that _v_ is indeed the block decided in height _h_.

* In the suboptimal case, Tendermint processes may decide in round _r=0_ or in round _r=1_. Therefore, it is possible that the Application of a process, say _q_, receives two calls to `ProcessProposal`, with two different values _v0_ and _v1_ while in height _h_.
  * There is no way for the Application to "guess" whether proposed blocks _v0_ or _v1_ will be decided before `FinalizeBlock` is called
  * The Application may choose to execute either proposed block as part of handling the `ProcessProposal` call, or both. However, one of those computations will need to be discarded at decision time. If the block decided was not processed at ProcessProposal time, it will need to be executed now.

* In the worst case, TODO: unbounded number of calls to `ProcessProposal`, with unbounded number of proposed blocks. Josef: if we restrict `PrepareProposal`, at least we get a bounded number here.

#### `ExtendVote`:  expectations from the Application

TODO (in different rounds). [Finish first discussion above]
