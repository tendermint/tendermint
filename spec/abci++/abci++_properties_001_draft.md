---
order: 1
title: New Methods
---

# New Methods

## Description

### PrepareProposal

#### Parameters and Types

>**TODO**: Hyperlinks for ConsensusParams, LastCommitInfo, Evidence, Event, and ValidatorUpdate are broken because they are defined in abci.md (and not copied over for the moment).

* **Request**:

    | Name                    | Type                                        | Description                                                                                                             | Field Number |
    |-------------------------|---------------------------------------------|-------------------------------------------------------------------------------------------------------------------------|--------------|
    | (*)hash                 | bytes                                       | The hash of the block to propose. This can be derived from the block header.                                            | 1            |
    | header                  | [Header](../core/data_structures.md#header) | The header of the block to propose.                                                                                     | 2            |
    | (*)last_commit_info     | [LastCommitInfo](#lastcommitinfo)           | Info about the last commit, including the round, the validator list, and which ones signed the last block. | 3            |
    | (*)byzantine_validators | repeated [Evidence](#evidence)              | List of evidence of validators that acted maliciously.                                                                  | 4            |
    | tx                      | repeated bytes                              | Preliminary list of transactions that have been picked as part of the block to propose.                                 | 5            |
    | height                  | int64                                       | Height of the block to propose (for sanity check).                                                                      | 6            |

* **Response**:

    | Name                    | Type                                        | Description                                                                                                             | Field Number |
    |-------------------------|---------------------------------------------|-------------------------------------------------------------------------------------------------------------------------|--------------|
    | modified                | bool                                        | The Application sets it to true to denote it did not make changes                                                       | 1            |
    | (*)hash                 | bytes                                       | The hash of the block to propose. This can be derived from the block header.                                            | 2            |
    | header                  | [Header](../core/data_structures.md#header) | The header of the block to propose.                                                                                     | 3            |
    | (*)last_commit_info     | [LastCommitInfo](#lastcommitinfo)           | Info about the last commit, including the round, the validator list, and which ones signed the last block.              | 4            |
    | (*)byzantine_validators | repeated [Evidence](#evidence)              | List of evidence of validators that acted maliciously.                                                                  | 5            |
    | tx                      | repeated bytes                              | Possibly modified list of transactions that have been picked as part of the proposed block.                             | 6            |
    | (*)header_events        | repeated [Event](#events)                   | Type & Key-Value events for indexing header information                                                                 | 7            |
    | (*)tx_events            | repeated [Event](#events)                   | Type & Key-Value events for indexing transactions                                                                       | 8            |

* **Usage**:
    * The Application can modify the parameters received in `RequestPrepareProposal` before sending
    them in `ResponsePrepareProposal`. In that case, `ResponsePrepareProposal.modified` is set to true.
    * If `ResponsePrepareProposal.modified` is false, then Tendermint should ignore the rest of
    parameters in `ResponsePrepareProposal`.
    * As a sanity check, Tendermint will check the returned parameters for validity if the Application modified it.

>**TODO**: All fields marked with an asterisk (*) are not 100% needed for PrepareProposal to work. Their presence depends on the functionality we want for PrepareProposal (i.e., what can the App modify?)

>**BEGIN TODO**
>
>Big question: How are we going to ensure a sane management of the mempool if the App can change (add, remove, modify, reorder) transactions. Some ideas:
>
>1. If Tendermint always does ReCheckTx on transactions in the mempool after every commit, then the App can ensure that mempool is properly managed
>  * _(pro)_ This doesn't need any extra logic or params
>  * _(con)_ It may be inefficient for some Apps
>2. Each returned transaction is attached a new enum, `Action`, and an extra `Hash` field:
 > * `Action` = Unmodified, Hash = `nil`. The Application didn't touch this transaction. Nothing to do on mempool
 > * `Action` = Added, Hash = `nil`. The Application added this new transaction to the list. Tendermint should hash it and check (for sanity) if it is already in the mempool, if not add it to the mempool
 > * `Action` = Removed, Hash = `nil`. The Application removed this transaction from the list, denoting it is not valid. Tendermint should remove it from the mempool (equivalent to ReCheckTx returning false). Note that the App is **not** removing the transaction, but **marking** it as invalid
 > * `Action` = Modified, Hash = `old_hash`. The Application modified the transaction and is indicating the TX's old hash (i.e., before the changes). Tendermint should remove the old transaction (based on old_hash) from the mempool, and add the modified one (if not there already)
 > * The Application SHOULD NOT remove transactions from the list it received in the Request, however, it can reorder them
 > * Note that this mechanism supports transaction reordering
 > * "Modified" is not strictly necessary. It can be simulated with "Removed", then "Added". This would render the "Hash" field unnecessary
 > * _(pro)_ Allows App to efficiently manage mempool
 > * _(con)_ More complex, less fool-proof for the App
>3. Identifying transactions with IDs in both directions and using those IDs instead of hashes. The mempool management would be similar to point 2.
>  * Same pros and cons as 2.
>  * _(pro w.r.t 2)_ IDs look to me a semantically clearer way to identify transactions
>  * _(con w.r.t 2)_ IDs need to be introduced... unclear if changes may reach further into Tendermint's code
>4. Other ideas?
>
>**END TODO**

>**TODO**: Since we are likely to keep CheckTx, does it make sense to have tx_events here? (If we allow for new TXs to be added here, then we might want to have their corresponding events)

#### When does Tendermint call it?

When a validator _p_ enters Tendermint consensus round _r_, height _h_, in which _p_ is the proposer:

1. _p_'s Tendermint collects outstanding transactions from the mempool
   * The transactions will be collected in order of priority
   * Let $C$ the list of currently collected transactions
   * The collection stops when any of the following conditions no longer holds
     * the mempool is not empty
     * the total size of transactions $\in C$ is less than `consensusParams.block.max_bytes`
     * the sum of `GasWanted` field of transactions $\in C$ is less than `consensusParams.block.max_gas`
2. _p_'s Tendermint creates a block header.
3. _p_'s Tendermint calls `RequestPrepareProposal` with the newly created block.
   The call is synchronous: Tendermint's execution will block until the Application returns from the call.
4. The Application checks the block (header, transactions, height). It can also:
   * modify the order of transactions
   * remove transactions
   * aggregate transactions (?)
   * add new transactions (not previously in the mempool)
   * **TODO**: Define how the mempool is maintained (see discussion above)
   * modify the block header (?)
     * **TODO**: what can and cannot be modified in the header?
     * **TODO**: include logic of `VerifyVoteExtension` here?
   * modify `last_commit_info_`, `byzantine_validators` (?)
5. If the block is modified, the Application sets `ResponsePrepareProposal.modified` to true,
   and includes the modified block in the return parameters (see the rules in section _Usage_).
   The Application returns from the call.
6. _p_'s Tendermint uses the (possibly) modified block as _p_'s proposal in round _r_, height _h_.

### ProcessProposal

#### Parameters and Types

* **Request**:

    | Name                 | Type                                        | Description                                                                                                                 | Field Number |
    |----------------------|---------------------------------------------|-----------------------------------------------------------------------------------------------------------------------------|--------------|
    | hash                 | bytes                                       | The proposed block's hash. This can be derived from the block header.                                                       | 1            |
    | header               | [Header](../core/data_structures.md#header) | The proposed block's header.                                                                                                | 2            |
    | last_commit_info     | [LastCommitInfo](#lastcommitinfo)           | Info about the last commit, including the round (**TODO**: !!!!), the validator list, and which ones signed the last block. | 3            |
    | byzantine_validators | repeated [Evidence](#evidence)              | List of evidence of validators that acted maliciously.                                                                      | 4            |
    | tx                   | repeated bytes                              | List of transactions that have been picked as part of the proposed block.                                                   | 5            |
    | height               | int64                                       | Height of the block just proposed (for sanity check).                                                                       | 6            |

* **Response**:

    | Name   | Type | Description                                                      | Field Number |
    |--------|------|------------------------------------------------------------------|--------------|
    | accept | bool | If false, instruct Tendermint to prevote $\bot$ for this proposal | 1            |

* **Usage**:
    * Contains a full proposed block.
      * The parameters and types of `RequestProcessProposal` are the same as `RequestFinalizeBlock`.
      * The App may decide to (optimistically) execute it as though it was handling `RequestFinalizeBlock`.
        However, any changes to the state must be kept as _canditade state_, and the Application should be ready to
        backtrack/discard it in case the decided block is different.
    * If `ResponseProcessProposal.accept` is true, Tendermint should prevote according to the normal procedure; else, it should prevote $\bot$

>**TODO**: Dev's pseudo-code also includes evidences in the Response message. However, I still can't see the advantage/utility, since evidences need to be committed in order to be heeded AFAIU.

>**TODO**: should `ResponseProcessProposal.accept` be of type `Result` rather than `bool`? (so we are able to extend the possible values in the future?)

#### When does Tendermint call it?

When a validator _p_ enters Tendermint consensus round _r_, height _h_, in which _q_ is the proposer (possibly _p_ = _q_):

1. _p_ sets up timer `ProposeTimeout`.
2. If _p_ is the proposer, _p_ executes steps 1-6 in _PrepareProposal_ (see above).
3. Upon reception of Proposal message (which contains the header) for round _r_, height _h_ from _q_, _p_'s Tendermint verifies the block header.
4. Upon reception of Proposal message , along with all the block parts, for round _r_, height _h_ from _q_, _p_'s Tendermint calls `RequestProcessProposal`
   with the newly received proposal. The call is synchronous.
5. The Application checks/processes the proposed block, which is read-only, and returns true (_accept_) or false (_reject_) in `ResponseProcessProposal.accept`.
   * The Application, depending on its needs, may call `ResponseProcessProposal`
     * either after it has completely processed the block (the simpler case),
     * or immediately (after doing some basic checks), and process the block asynchronously. In this case the Application will
       not be able to reject the block, or force prevote/precommit $\bot$ afterwards.
6. If the returned value is
     * _accept_, Tendermint will follow the normal algorithm to prevote on this proposal for round _r_, height _h_.
     * _reject_, Tendermint will prevote $\bot$ for the proposal in round _r_, height _h_, which is a special value that processes can provote and precommit for,
       and also decide. Processes can still vote `nil` according to the Tendermint algorithm. The role of $\bot$ is to protect Tendermint's liveness in cases
       where the Application has a problem in its implementation of _ProcessProposal_ that, albeit deterministically, rejects too many proposals.

>**TODO**: To discuss with Josef. If we decide $\bot$, we manage to keep consensus going, but we will be producing (public) empty blocks until the Application's logic gets fixed...

### ExtendVote

#### Parameters and Types

* **Request**:

    | Name   | Type  | Description                                                           | Field Number |
    |--------|-------|-----------------------------------------------------------------------|--------------|
    | hash   | bytes | The hash of the proposed block that the vote extension is to refer to | 1            |
    | height | int64 | Height of the block just proposed (for sanity check).                 | 2            |

* **Response**:

    | Name      | Type  | Description                                                         | Field Number |
    |-----------|-------|---------------------------------------------------------------------|--------------|
    | extension | bytes | Optional information that will be attached to the Precommit message | 1            |

* **Usage**:
    * `RequestExtendVote.hash` corresponds to the hash of a proposed block that was made available to the application
      in a previous call to `ProcessProposal` for the current height.
    * `ResponseExtendVote.extension` will always be attached to a non-`nil` Precommit message. If Tendermint is to
      precommit `nil`, it will not call `RequestExtendVote`.

>**BEGIN TODO**
>
>The Request call also includes round and height in Dev's pseudo-code:
>
>   * Height, although clearly unnecessary (unless we want to extend votes from previous heights that arrive late), is included
>     to allow the App for sanity checks. Should we keep it?. This also applies to `RequestPrepareProposal`, `RequestProcessProposal`, and `VerifyVoteExtension`.
>   * Round may make sense, but why would the App be interested in it? Rounds might be different for different
>     validators (see also the TODO in FinalizeBlock).
>
>**END TODO**

#### When does Tendermint call it?

When a validator _p_ is in Tendermint consensus state _prevote_ of round _r_, height _h_, in which _q_ is the proposer; and _p_ has received

* the Proposal message _v_ for round _r_, height _h_, along with all the block parts, from _q_,
* `Prevote` messages from _2f + 1_ validators' voting power for round _r_, height _h_, prevoting for the same block _id(v)_,

then _p_'s Tendermint locks _v_  and sends a Precommit message in the following way

1. _p_'s Tendermint sets _lockedValue_ and _validValue_ to _v_, and sets _lockedRound_ and _validRound_ to _r_
2. _p_'s Tendermint calls `RequestExtendVote` with _id(v)_ (`RequestExtendVote.hash`). The call is synchronous.
3. The Application returns an array of bytes, `ResponseExtendVote.extension`, which is not interpreted by Tendermint.
4. _p_'s Tendermint includes `ResponseExtendVote.extension` as a new field in the Precommit message.
5. _p_'s Tendermint signs and broadcasts the Precommit message.

In the cases when _p_'s Tendermint is to broadcast `precommit nil` messages (either _2f+1_ `prevote nil` messages received, or _timeoutPrevote_ triggered), _p_'s Tendermint does **not** call `RequestExtendVote` and will include an empty byte array as vote extension in the `precommit nil` message.

### VerifyVoteExtension

#### Parameters and Types

>**TODO** (still very controversial). Current status of discussions:

* **Request**:

    | Name      | Type  | Description                                           | Field Number |
    |-----------|-------|-------------------------------------------------------|--------------|
    | extension | bytes | Sender Application's vote information to be validated | 1            |
    | height    | int64 | Height of the block  (for sanity check).              | 2            |

* **Response**:

    | Name   | Type | Description                                           | Field Number |
    |--------|------|-------------------------------------------------------|--------------|
    | accept | bool | If false, Application is rejecting the vote extension | 1            |

* **Usage**: **TODO**: BIG question: should `ResponseVerifyVoteExtension.accept` influence Tendermint's acceptance of the Precommit message it came in? (see below)

>**TODO** We probably need a new paramater in **Request**, _hash_, with the hash of the proposed block the extension refers to. Please see Property 7 below for more info on this.

>**TODO** IMPORTANT. We need to describe what Tendermint does with the extensions (includes them in the next proposal's header?).
We need to describe it here in detail (referencing the data structures concerned).

#### When does Tendermint call it?

When a validator _p_ is in Tendermint consensus round _r_, height _h_, state _prevote_ (**TODO** discuss: I think I must remove the state
from this condition, but not sure), and _p_ receives a Precommit message for round _r_, height _h_ from _q_:

1. _p_'s Tendermint calls `RequestVerifyVoteExtension`.
2. The Application returns _accept_ or _reject_.
3. _p_'s Tendermint deems the Precommit message invalid if the Application returned _reject_.

>**BEGIN TODO**
>
>Current status of discussion of "Application's Reject forcing consensus to reject/ignore a (non-nil) precommit value":
>
>* If App can force reject --> important liveness implications to consensus
>* From discussion with Callum: App rejects vote extension only if
>   * bug in sender's App (or, more generally, sender's App is Byzantine)
>   * vote extension rejection based on App's specific semantics seems out of the question for everyone
>   * vote extension must be deterministic, and depend **exclusively** on
>     * last committed App state
>     * contents of vote extension
>* The big question here is: is there a reasonable enough way from engineers to detect any non-determinism that may creep in?
>  * It'll be more complicated than detecting non-determinism when executing a block (detected by app-hash, result-hash, etc)
>  * In this case, if there is non determinism, the liveness of consensus is compromised, so it will be very hard to troubleshoot!
>  * BTW, the same reasoning can be applied to `ResponseProcessProposal.accept`
>
>**END TODO**

### FinalizeBlock

#### Parameters and Types

* **Request**:

    | Name                 | Type                                        | Description                                                                                                       | Field Number |
    |----------------------|---------------------------------------------|-------------------------------------------------------------------------------------------------------------------|--------------|
    | hash                 | bytes                                       | The block's hash. This can be derived from the block header.                                                      | 1            |
    | header               | [Header](../core/data_structures.md#header) | The block header.                                                                                                 | 2            |
    | last_commit_info     | [LastCommitInfo](#lastcommitinfo)           | Info about the last commit, including the round, and the list of validators and which ones signed the last block. | 3            |
    | byzantine_validators | repeated [Evidence](#evidence)              | List of evidence of validators that acted maliciously.                                                            | 4            |
    | tx                   | repeated bytes                              | List of transactions committed as part of the block.                                                              | 5            |
    | height               | int64                                       | Height of the block just executed.                                                                                | 6            |

>**TODO**: Discuss if we really want to pass whole block here, or only its hash (since the block was part of a previous `RequestProcessProposal`).
We could _require_ that App to keep all blocks received in `ProcessProposal`, so it wouldn't be needed to pass them here...
even if the App only executes the decided block (no optimistic/immediate execution).

* **Response**:

    | Name                    | Type                                         | Description                                                                     | Field Number |
    |-------------------------|----------------------------------------------|---------------------------------------------------------------------------------|--------------|
    | header_events           | repeated [Event](#events)                    | Type & Key-Value events for indexing                                            | 1            |
    | tx_result               | repeated [DeliverTxResult](#delivertxresult) | List of structures containing the data resulting from executing the transaction | 2            |
    | validator_updates       | repeated [ValidatorUpdate](#validatorupdate) | Changes to validator set (set voting power to 0 to remove).                     | 3            |
    | consensus_param_updates | [ConsensusParams](#consensusparams)          | Changes to consensus-critical gas, size, and other parameters.                  | 4            |
    | block_events            | repeated [Event](#events)                    | Type & Key-Value events for indexing                                            | 5            |
    | app_data                | bytes                                        | The Merkle root hash of the application state.                                  | 6            |
    | retain_height           | int64                                        | Blocks below this height may be removed. Defaults to `0` (retain all).          | 7            |

* **Usage**:
    * Contains a new block. (**TODO**: or the new block's hash?)
    * This method is equivalent to the call sequence `BeginBlock`, [`DeliverTx`],
      `EndBlock`, `Commit` in the previous version of ABCI.
    * The header contains the height, timestamp, and more - it exactly matches the
      Tendermint block header. **TODO**: Any extensions brought by ABCI++ ? (I think so, but which ones exactly?)
    * The Application can use `RequestFinalizeBlock.last_commit_info` and `RequestFinalizeBlock.byzantine_validators`
      to determine rewards and punishments for the validators.
    * The application must execute the transactions in full, in the order they appear in `RequestFinalizeBlock.tx`,
      before returning control to Tendermint. Alternatively, it can commit the candidate state corresponding to the block
      previously executed via `ProcessProposal`.
    * `ResponseFinalizeBlock.tx_result[i].Code == 0` only if the _i_-th transaction is fully valid.
    * Optional `ResponseFinalizeBlock.validator_updates` triggered by block `H`. These updates affect validation
      for blocks `H+1`, `H+2`, and `H+3`. Heights following a validator update are affected in the following way:
        * `H+1`: `NextValidatorsHash` includes the new `validator_updates` value.
        * `H+2`: The validator set change takes effect and `ValidatorsHash` is updated.
        * `H+3`: `last_commit_info` is changed to include the altered validator set.
    * `ResponseFinalizeBlock.consensus_param_updates` returned for block `H` apply to the consensus
      params for block `H+1`. For more information on the consensus parameters,
      see the [application spec entry on consensus parameters](../abci/apps.md#consensus-parameters).
    * Application is expected to persist its state at the end of this call, before calling `ResponseFinalizeBlock`.
    * `ResponseFinalizeBlock.app_data` contains an (optional) Merkle root hash of the application state.
    * `ResponseFinalizeBlock.app_data` is included as the `Header.AppHash` in the next block. It may be empty.
    * Later calls to `Query` can return proofs about the application state anchored
      in this Merkle root hash.
    * Note developers can return whatever they want here (could be nothing, or a
      constant string, etc.), so long as it is **deterministic** - it must not be a
      function of anything that did not come from the parameters of `RequestFinalizeBlock`
      and the previous committed state.
    * Use `retain_height` with caution! If all nodes in the network remove historical
      blocks then this data is permanently lost, and no new nodes will be able to
      join the network and bootstrap. Historical blocks may also be required for
      other purposes, e.g. auditing, replay of non-persisted heights, light client
      verification, and so on.

>**TODO**: Round is contained in last_commit_info. My current understanding is that it contains the round in which the
proposer of height H+1 decided in H. Note that different proposers may decide in different rounds. So, I would like to
understand how the Application uses that data.

#### When is it called?

When a validator _p_ is in Tendermint consensus height _h_, and _p_ receives

* the Proposal message with block _v_ for a round _r_, along with all its block parts, from _q_, which is the proposer of round _r_, height _h_,
* `Precommit` messages from _2f + 1_ validators' voting power for round _r_, height _h_, precommitting the same block _id(v)_,

then _p_'s Tendermint decides block _v_ and finalizes consensus for height _h_ in the following way

1. _p_ panics if the following does not hold

  * _p_ has prevoted or precommitted for $\bot$ in round _r_, height _h_, if and only if $v = \bot$
2. _p_'s Tendermint persists _v_ as decision for height _h_.
3. _p_'s Tendermint locks the mempool -- no calls to checkTx on new transactions.
4. _p_'s Tendermint calls `RequestFinalizeBlock` with _id(v)_. The call is synchronous.
5. _p_'s Application processes block _v_, received in a previous call to `RequestProcessProposal`.
6. _p_'s Application commits and persists the state resulting from processing the block.
7. _p_'s Application calculates and returns the _AppHash_, along with an array of arrays of bytes representing the output of each of the transactions
8. _p_'s Tendermint hashes the array of transaction outputs and stores it in _ResultHash_
9. _p_'s Tendermint persists _AppHash_ and _ResultHash_
10. _p_'s Tendermint unlocks the mempool -- newly received transactions can now be checked.
11. _p_'s starts consensus for a new height _h+1_, round 0

Step 1 above helps protect agains non-deterministic implementations of _ProcessPropoasl_ and _VerifyVoteExtension_.

### [Points unclear/to discuss further]

#### Dev's v4 changes (a.k.a. _multithreaded ABCI++_)

Comments on the "fork-join" mechanism

* So far, the operation mode for ABCI has been mainly synchronous. The most notable exception is: checkTx (deliverTx is not really async).
  Calls to checkTx can afford to be async because checkTx provides no strong properties: checkTx returning Accept does not guarantee the Tx's validity when it makes it into a block.
* AFAIU: The "join" part has two aims in the pseudocode (v4)
  * early collection and treatment of evidences. See comments above
  * influencing the precommit to be sent (whether _id(v)_ or `nil`)

>**BEGIN TODO**
>
>* Several issues we need to fully understand
>   * Changing ABCI from sync to async may have unintended side effects on both sides. We are introducing concurrency in the API that affects the logic of consensus
>   * If App is late in providing the result, Tendermint may advance to r=1, thus forking a new ProcessProposal in parallel --> even more time to complete --> vicious circle
>   * But, the main issue we'd need to overcome is the possibility that the value rejected by _ProcessProposal_ gets locked by $f + 1$ correct processes in that round
>     * In this case (according to Tendermint algorithm's proof) no other value can be decided. Liveness breaks down!
>     * About introducing $\bot$ here to fix liveness, I don't think it works (unlike at prevote step) as $\bot$ is considered "just another" value you can decide on,
>       so it must follow the same rules to get locked and then decided (but the rejected value is already locked!)
>
>**END TODO**

#### Separation of `VerifyHeader` and `ProcessProposal`

>**TODO** My interpretation of Dev's pseudo code is that it only makes sense if using the "Fork--Join" mechanism. So this discussion should be carried out after the one above

#### Byzantine proposer

[From Josef] We should understand the influence of equivocation on proposals in ABCI++

N.B: If Byzantine proposer proposes both A and B in the same round, today we might only receive A (proposal) and not B

Sergio: preliminary thoughts:

[Thought 1] If we deliver all proposals from a Byzantine proposer _p_ in _h_ and _r_ we're vulnerable to DoS, as _p_ can send as many as it wishes

* Assuming here an App that fully executes the proposed block at _ProcessProposal_ time
* Assuming the N.B. above gets "fixed" (A and B get delivered by p2p)

So probably we are better off if we do not "fix" the N.B.

* or a way to fix it would be to only "use" the first proposal in consensus, and use the others just to submit evidence

[Thought 2] _ProcessProposal_ is exposed to arbitrary input, yet it should still behave deterministically.

* In ABCI, this is also the case upon (BeginBlock, [DeliverTx], EndBlock, Commit). The difference is that, at the end of the block we have the "protection" of AppHash, ResultsHash to guard against non-determinism.
* Here, arbitrary proposals that "uncover" indeterminism issues in the _ProcessProposal_ code compromise consenus's liveness
* Any ideas what we can do here? (I have the impression $\bot$ helps here only partially)

[Thought 3] In terms of the consensus's safety properties, as far as pure equivocation on proposals is concerned, I think we are OK since Tendermint builds on a Byzantine failure model.

#### `ProcessProposal`'s timeout (a.k.a. Zarko's Github comment)

>**TODO** (to discuss): `PrepareProposal` is called synchronously. `ProcessProposal` may also want to fully process the block synchronously. However, they stand on Tendermint's critical path, so the propose timeout needs to accomodate that.

Idea: Make propose timestamp (currently hardcoded to 3 secs in the Tendermint Go implementation) part of ConsensusParams, so the App can adjust it with its knowledge of the time may take to prepare/process the proposal.

## Data types

### DeliverTxResult

* **Fields**:

    | Name       | Type                      | Description                                                           | Field Number |
    |------------|---------------------------|-----------------------------------------------------------------------|--------------|
    | code       | uint32                    | Response code.                                                        | 1            |
    | data       | bytes                     | Result bytes, if any.                                                 | 2            |
    | log        | string                    | The output of the application's logger. **May be non-deterministic.** | 3            |
    | info       | string                    | Additional information. **May be non-deterministic.**                 | 4            |
    | gas_wanted | int64                     | Amount of gas requested for transaction.                              | 5            |
    | gas_used   | int64                     | Amount of gas consumed by transaction.                                | 6            |
    | tx_events  | repeated [Event](#events) | Type & Key-Value events for indexing transactions (e.g. by account).  | 7            |
    | codespace  | string                    | Namespace for the `code`.                                             | 8            |

## Properties

### What Tendermint expects from the Application

Let $p$ and $q$ be two different correct proposers in rounds $r_p$ and $r_q$ respectively, in height $h$.
Let $s_{h-1}$ be the Application's state committed for height $h-1$.
Let $v_p$ (resp. $v_q$) be the block that $p$'s (resp. $q$'s) Tendermint passes to the Application via `RequestPrepareProposal`
as proposer of round $r_p$ (resp $r_q$), height $h$.
Let $v'_p$ (resp. $v'_q$) the possibly modified block $p$'s (resp. $q$'s) Application returns via `ResponsePrepareProposal` to Tendermint.

> **TODO** Should we add a restriction so that $v'_p$ should be the same in all rounds, in height $h$, for which $p$ is proposer?
This would help the App manage the number of candidate states, although it is not a total solution,
as Byzantine could still send different proposals for different rounds (well, it can get caught and slashed)

* Property 1 [`PrepareProposal`, header-changes] **TODO** Which parts of the header can the App touch?

* Property 2 [`PrepareProposal`, `ProcessProposal`, coherence]: For any two correct processes $p$ and $q$,
  if $q$'s Tendermint calls `RequestProcessProposal` on $v'_p$,
  $q$'s Application returns Accept in `ResponseProcessProposal`.

Property 2 makes sure that blocks proposed by correct processes will always pass the receiving validator's `ProcessProposal` check.
On the other hand, a violation of Property 2 (e.g., a bug in `PrepareProposal`, or in `ProcessProposal`, or in both) may force some
(or even all) correct processes to prevote `nil`.
This has serious consequences on Tendermint's liveness and, therefore, we introduce a new mechanism here: in addition to voting `nil`
or _id(v)_, a process can also vote $\bot$, which represents and empty block.
A correct process $p$ will prevote $\bot$ if $p$'s `ProcessProposal` implementation rejects the given block.

As $\bot$ is a value introduced at proposal time, it will follow the Tendermint consensus just as any other proposed block.
However, it has the advantage that Tendermint does not get stuck in increasing rounds of a particular height,
should there be validators with more an 1/3 of the total voting power whose `ProcessProposal`
implementation wrongly rejects most (or all) proposed values.

* Property 3 [`ProcessProposal`, determinism-2]: For any correct process $p$, and any arbitrary block $v'$,
  if $p$'s Tendermint calls `RequestProcessProposal` on $v'$ in height $h$,
  then $p$'s Application's acceptance or rejection exclusively depends on $v'$ and $s_{h-1}$.

* Property 4 [`ProcessProposal`, determinism-1]: For any two correct processes $p$ and $q$, and any arbitrary block $v'$,
  if $p$'s (resp. $q$'s) Tendermint calls `RequestProcessProposal` on $v'$,
  then $p$'s Application accepts $v'$ if and only if $q$'s Application accepts $v'$.
  Note that this Property follows from Property 3 and the Agreement property of consensus.

>**TODO** If Property 4 does not hold, liveness issues. $\bot$ doesn't really help. What can we do?

According to the Tendermint algorithm, a correct process can only broadcast one precommit message in round $r$, height $h$.
Since, as stated in the [Description](#description) section, `ResponseExtendVote` is only called when Tendermint
is about to broadcast a non-`nil` precommit message, a correct process can only produce one vote extension in round $r$, height $h$.
Let $e^r_p$ the vote extension that the Application of a correct process $p$ returns via `ResponseExtendVote` in round $r$, height $h$.

>**TODO**: Question: Is it a good idea to extend the signature of `RequestVerifyVoteExtension`
with the block hash, just as `RequestExtendVote`? (I think the App cannot infer it by itself because the proposer may be byzantine).

* Property 5 [`ExtendVote`, `VerifyVoteExtension`, coherence]: For any two correct processes $p$ and $q$, if $q$ receives $e^r_p$
  from $p$ in height $h$, $q$'s Application returns Accept in `ResponseVerifyVoteExtension`.

* Property 6 [`VerifyVoteExtension`, determinism]: For any two correct processes $p$ and $q$, and any arbitrary vote extension $e$,
  if $p$'s (resp. $q$'s) Tendermint calls `RequestVerifyVoteExtension` on $e$,
  then $p$'s Application accepts $e$ if and only if $q$'s Application accepts $e$.

>**TODO** If Property 6 does not hold, liveness depends on whether App rejection forces Tendermint to reject the precommit message or not.
Proposal: flag the rejected extension (while accepting the precommit message), and keep the flag together with the extension.

>**TODO** Property 6 does not guarantee that all correct processes will end up with the same extension received from the same process,
as a Byzantine process could send different (valid) extensions to different processes.
Is this a big deal? Do we need an extra property?

* Property 7 [_all_, read-only]: The calls to `RequestPrepareProposal`, `RequestProcessProposal`, `RequestExtendVote`,
  and `RequestVerifyVoteExtension` in height $h$ do not modify $s_{h-1}$.

>**TODO** We need properties for `FinalizeBlock`, but postponing ATM as it is well understood (basically, same a ABCI).

Finally, notice that `PrepareProposal` does not have determinism-related properties associated.
Indeed, `PrepareProposal` is not required to be deterministic:

* $v_p = v_q \nRightarrow v'_p = v'_q$.
* $v'_p$ may depend on $v_p$ and $s_{h-1}$, but may also depend on other values or operations.

### What the Application can expect from Tendermint

The following sections use these definitions:

* We define the _common case_ as a run in which (a) the system behaves synchronously, and (b) there are no Byzantine processes.
  The common case captures the conditions that hold most of the time, but not always.
* We define the _suboptimal case_ as a run in which (a) the system behaves asynchronously in round 0, height h -- messages may be delayed,
  timeouts may be triggered --, (b) it behaves synchronously in all subsequent rounds (_r>=1_), and (c) there are no Byzantine processes.
  The _suboptimal case_ captures a possible glitch in the network, or some sudden, sporadic performance issue in some validator
  (network card glitch, thrashing peak, etc.).
* We define the _general case_ as a run in which (a) the system behaves asynchronously for a number of rounds,
  (b) it behaves synchronously after some time, denoted _GST_, which is unknown to the processes,
  and (c) there may be up to _f_ Byzantine processes.

#### `ProcessProposal`:  expectations from the Application

Given a block height _h_, process _p_'s Tendermint calls `RequestProcessProposal` every time it receives a proposed block from the
proposer in round _r_, for _r_ in 0, 1, 2, ...

* In the common case, all Tendermint processes decide in round _r=0_. Let's call _v_ the block proposed by the proposer of round _r=0_, height _h_.
  Process _p_'s Application will receive exactly one call to `RequestProcessProposal` for block height _h_ containing _v_.
  The Application may execute _v_ as part of handling the `RequestProcessProposal` call, keep the resulting state as a candidate state,
  and apply it when the call to `RequestFinalizeBlock` for _h_ confirms that _v_ is indeed the block decided in height _h_.
* In the suboptimal case, Tendermint processes may decide in round _r=0_ or in round _r=1_.
  Therefore, the Application of a process _q_ may receive one or two calls to `RequestProcessProposal`,
  with two different values _v0_ and _v1_ while in height _h_.
  * There is no way for the Application to "guess" whether proposed block _v0_ or _v1_ will be decided before `RequestFinalizeBlock` is called.
  * The Application may choose to execute either (or both) proposed block as part of handling the `RequestProcessProposal`
    call and keep the resulting state as candidate state.
    At decision time, if the block decided corresponds to one of the candidate states, it can be committed,
    otherwise the Application will have to execute the block at this point.
* In the general case, the round in which a Tendermint processes _p_ will decide cannot be forecast. _p_'s Tendermint may call `RequestProcessProposal` in each round,
  and each call may convey a different proposal. As a result, the Application may need to deal with an unbounded number of different proposals for a given height,
  and also an unbounded number of candidate states if it is executing the proposed blocks optimistically (a.k.a. immediate execution).
  * **TODO**: Discuss with Josef: if we restrict `PrepareProposal` to one value per _p_ per height, at least we get a bounded number of proposed blocks
    in the worst case (assuming we detect and filter out Byzantine equivocation).

#### `ExtendVote`:  expectations from the Application

>**TODO** (in different rounds). [Finish first discussion above]

--

If `ProcessProposal`'s outcome is _Reject_ for some proposed block. Tendermint guarantees that the block will not be the decision.

>**TODO**: This has liveness implications (see above).

--

The validity of every transaction in a block (from the App's point of view), as well as the hashes in its header can be guaranteed if:

* `ProcessProposal` *synchronously* handles every proposed block as though Tendermint had already decided on it.
* All the properties of `ProcessProposal` and `FinalizeBlock` mentioned above hold.

## Application modes

[This is a sketch ATM]

Mode 1: Simple mode (equivalent to ABCI).

TODO: Define _candidate block_

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
