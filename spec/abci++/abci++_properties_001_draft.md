---
order: 1
title: New Methods
---

# New Methods

## Description

### PrepareProposal

#### Parameters and Types

>**TODO**: Hyperlinks for ConsensusParams, LastCommitInfo, Evidence, Event, and ValidatorUpdate are
broken because they are defined in abci.md (and not copied over for the moment).

* **Request**:

    | Name                    | Type                                        | Description                                                                                                      | Field Number |
    |-------------------------|---------------------------------------------|------------------------------------------------------------------------------------------------------------------|--------------|
    | hash                    | bytes                                       | The block header's hash of the block to propose. Present for convenience (can be derived from the block header). | 1            |
    | header                  | [Header](../core/data_structures.md#header) | The header of the block to propose.                                                                              | 2            |
    | tx                      | repeated bytes                              | Preliminary list of transactions that have been picked as part of the block to propose.                          | 3            |
    | byzantine_validators    | repeated [Evidence](#evidence)              | List of evidence of validators that acted maliciously.                                                           | 4            |
    | last_commit_info        | [LastCommitInfo](#lastcommitinfo)           | Info about the last commit, including the round, the validator list, and which ones signed the last block.       | 5            |

>**TODO**: DISCUSS: We need to make clear whether a proposer is also running the logic of a non-proposer node (in particular "ProcessProposal")
From the App's perspective, they'll probably skip ProcessProposal

* **Response**:

    | Name                    | Type                                             | Description                                                                                 | Field Number |
    |-------------------------|--------------------------------------------------|---------------------------------------------------------------------------------------------|--------------|
    | modified                | bool                                             | The Application sets it to true to denote that it made changes                           | 1            |
    | tx                      | repeated [TransactionRecord](#transactionrecord) | Possibly modified list of transactions that have been picked as part of the proposed block. | 2            |
    | data                    | bytes                                            | The Merkle root hash of the application state.                                              | 3            |
    | validator_updates       | repeated [ValidatorUpdate](#validatorupdate)     | Changes to validator set (set voting power to 0 to remove).                                 | 4            |
    | consensus_param_updates | [ConsensusParams](#consensusparams)              | Changes to consensus-critical gas, size, and other parameters.                              | 5            |

* **Usage**:
    * Contains a preliminary block to be proposed, which the Application can modify.
    * The parameters and types of `RequestPrepareProposal` are the same as `RequestProcessProposal`
      and `RequestFinalizeBlock`.
    * The header contains the height, timestamp, and more - it exactly matches the
      Tendermint block header.
    * The Application can modify the parameters received in `RequestPrepareProposal` before sending
    them in `ResponsePrepareProposal`. In that case, `ResponsePrepareProposal.modified` is set to true.
    * In same-block execution mode, the Application can (and should) modify `ResponsePrepareProposal.data`,
      `ResponsePrepareProposal.validator_updates`, and `ResponsePrepareProposal.consensus_param_updates`.
    * In next-block execution mode, the Application can only modify `ResponsePrepareProposal.tx`, Tendermint
      will ignore any modification to the other fields.
    * If `ResponsePrepareProposal.modified` is false, then Tendermint will ignore the rest of
      parameters in `ResponsePrepareProposal`.
    * As a result of executing the block to propose, the Application may produce header events or transaction events.
      The Application must keep those events until a block is decided and then pass them on to Tendermint via
      `ResponseFinalizeBlock`.
    * Likewise, the Application must keep all responses to executing transactions until it can call `ResponseFinalizeBlock`.
    * The Application can change the transaction list via `ResponsePrepareProposal.tx`.
      See [TransactionRecord](#transactionrecord) for further information on how to use it. Some notes:
        * To remove a transaction from the proposed block the Application _marks_ the transaction as
          "REMOVE". It does not remove it from the list.
        * Removing a transaction from the list means it is too early to propose that transaction,
          so it will be excluded from the proposal but will stay in the mempool for later proposals.
          The Application should be extra-careful with this feature to avoid leaks in the mempool.
        * The `new_hashes` field, besides helping with mempool maintenance, helps Tendermint handle
          queries such as "what happened with this Tx?", by answering "it was modified into these ones".
        * The Application _can_ reorder the transactions in the list.
    * As a sanity check, Tendermint will check the returned parameters for validity if the Application modified them.
      In particular, `ResponsePrepareProposal.tx` will be deemed invalid if
        * There is a duplicate transaction in the list.
        * The `new_hashes` field contains a dangling reference to a non-existing transaction.
        * A new or modified transaction is marked as "UNMODIFIED" or "REMOVED".
        * An unmodified transaction is marked as "ADDED".
        * A transaction is marked as "UNKNOWN".
    * If Tendermint's sanity checks on the parameters of `ResponsePrepareProposal` fails, then it will drop the proposal
      and proceed to the next round (thus simulating a network loss/delay of the proposal).
        * **TODO**: [From discussion with William] Another possibility here is to panic. What do folks think we should do here?
    * The implementation of `PrepareProposal` can be non-deterministic.

#### When does Tendermint call it?

When a validator _p_ enters Tendermint consensus round _r_, height _h_, in which _p_ is the proposer,
and _p_'s _validValue_ is `nil`:

1. _p_'s Tendermint collects outstanding transactions from the mempool
    * The transactions will be collected in order of priority
    * Let $C$ the list of currently collected transactions
    * The collection stops when any of the following conditions are met
        * the mempool is empty
        * the total size of transactions $\in C$ is greater than or equal to `consensusParams.block.max_bytes`
        * the sum of `GasWanted` field of transactions $\in C$ is greater than or equal to
          `consensusParams.block.max_gas`
    * _p_'s Tendermint creates a block header.
2. _p_'s Tendermint calls `RequestPrepareProposal` with the newly generated block.
   The call is synchronous: Tendermint's execution will block until the Application returns from the call.
3. The Application checks the block (header, transactions, commit info, evidences). Besides,
    * in "same-block execution" mode, the Application can (and should) provide `ResponsePrepareProposal.data`,
      `ResponsePrepareProposal.validator_updates`, or
      `ResponsePrepareProposal.consensus_param_updates`.
    * in "next-block execution" mode, _p_'s Tendermint will ignore the values for `ResponsePrepareProposal.data`,
      `ResponsePrepareProposal.validator_updates`, and `ResponsePrepareProposal.consensus_param_updates`.
    * in both modes, the Application can manipulate transactions
        * leave transactions untouched - `TxAction = UNMODIFIED`
        * add new transactions (not previously in the mempool) - `TxAction = ADDED`
        * remove transactions (invalid) from the proposal and from the mempool - `TxAction = REMOVED`
        * remove transactions from the proposal but not from the mempool (effectively _delaying_ them) - the
          Application removes the transaction from the list
        * modify transactions (e.g. aggregate them) - `TxAction = ADDED` followed by `TxAction = REMOVED`
        * reorder transactions - the Application reorders transactions in the list
4. If the block is modified, the Application sets `ResponsePrepareProposal.modified` to true,
   and includes the modified block in the return parameters (see the rules in section _Usage_).
   The Application returns from the call.
5. _p_'s Tendermint uses the (possibly) modified block as _p_'s proposal in round _r_, height _h_.

Note that, if _p_ has a non-`nil` _validValue_, Tendermint will use it as proposal and will not call `RequestPrepareProposal`.

### ProcessProposal

#### Parameters and Types

* **Request**:

    | Name                 | Type                                        | Description                                                                                                      | Field Number |
    |----------------------|---------------------------------------------|------------------------------------------------------------------------------------------------------------------|--------------|
    | hash                 | bytes                                       | The block header's hash of the proposed block. Present for convenience (can be derived from the block header).   | 1            |
    | header               | [Header](../core/data_structures.md#header) | The proposed block's header.                                                                                     | 2            |
    | tx                   | repeated bytes                              | List of transactions that have been picked as part of the proposed block.                                        | 3            |
    | byzantine_validators | repeated [Evidence](#evidence)              | List of evidence of validators that acted maliciously.                                                           | 4            |
    | last_commit_info     | [LastCommitInfo](#lastcommitinfo)           | Info about the last commit, including the round , the validator list, and which ones signed the last block.      | 5            |

* **Response**:

    | Name   | Type | Description                                      | Field Number |
    |--------|------|--------------------------------------------------|--------------|
    | accept | bool | If false, the received block failed verification | 1            |

* **Usage**:
    * Contains a full proposed block.
        * The parameters and types of `RequestProcessProposal` are the same as `RequestPrepareProposal`
          and `RequestFinalizeBlock`.
        * In "same-block execution" mode, the Application will fully execute the block
          as though it was handling `RequestFinalizeBlock`.
          However, any resulting state changes must be kept as _canditade state_,
          and the Application should be ready to
          backtrack/discard it in case the decided block is different.
    * The header contains the height, timestamp, and more - it exactly matches the
      Tendermint block header.
    * If `ResponseProcessProposal.accept` is _false_, Tendermint assumes the proposal received
      is not valid.
    * The implementation of `ProcessProposal` MUST be deterministic. Moreover, the value of
      `ResponseProcessProposal.accept` MUST *exclusively* depend on the parameters passed in
      the call to `RequestProcessProposal`, and the last committed Application state
      (see [Properties](#properties) section below).
    * Moreover, application implementors SHOULD always set `ResponseProcessProposal.accept` to _true_,
      unless they _really_ know what the potential liveness implications of returning _false_ are.

>**TODO**: should `ResponseProcessProposal.accept` be of type `Result` rather than `bool`? (so we are able to extend the possible values in the future?)

#### When does Tendermint call it?

When a validator _p_ enters Tendermint consensus round _r_, height _h_, in which _q_ is the proposer (possibly _p_ = _q_):

1. _p_ sets up timer `ProposeTimeout`.
2. If _p_ is the proposer, _p_ executes steps 1-6 in _PrepareProposal_ (see above).
3. Upon reception of Proposal message (which contains the header) for round _r_, height _h_ from _q_, _p_'s Tendermint verifies the block header.
4. Upon reception of Proposal message, along with all the block parts, for round _r_, height _h_ from _q_, _p_'s Tendermint follows its algorithm
   to check whether it should prevote for the block just received, or `nil`
5. If Tendermint should prevote for the block just received
    1. Tendermint calls `RequestProcessProposal` with the block. The call is synchronous.
    2. The Application checks/processes the proposed block, which is read-only, and returns true (_accept_) or false (_reject_) in `ResponseProcessProposal.accept`.
       * The Application, depending on its needs, may call `ResponseProcessProposal`
         * either after it has completely processed the block (the simpler case),
         * or immediately (after doing some basic checks), and process the block asynchronously. In this case the Application will
           not be able to reject the block, or force prevote/precommit `nil` afterwards.
    3. If the returned value is
         * _accept_, Tendermint prevotes on this proposal for round _r_, height _h_.
         * _reject_, Tendermint prevotes `nil`.

### ExtendVote

#### Parameters and Types

* **Request**:

    | Name   | Type  | Description                                                           | Field Number |
    |--------|-------|-----------------------------------------------------------------------|--------------|
    | hash   | bytes | The hash of the proposed block that the vote extension is to refer to | 1            |
    | height | int64 | Height of the proposed block (for sanity check).                      | 2            |

* **Response**:

    | Name      | Type  | Description                                                         | Field Number |
    |-----------|-------|---------------------------------------------------------------------|--------------|
    | extension | bytes | Optional information that will be attached to the Precommit message | 1            |

* **Usage**:
    * `RequestExtendVote.hash` corresponds to the hash of a proposed block that was made available to the application
      in a previous call to `ProcessProposal` for the current height.
    * `ResponseExtendVote.extension` will always be attached to a non-`nil` Precommit message. If Tendermint is to
      precommit `nil`, it will not call `RequestExtendVote`.
    * The Application logic that creates the extension can be non-deterministic.

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

* **Request**:

    | Name      | Type  | Description                                                                              | Field Number |
    |-----------|-------|------------------------------------------------------------------------------------------|--------------|
    | extension | bytes | Sender Application's vote extension to be validated                                      | 1            |
    | hash      | bytes | The hash of the propsed block that the vote extension refers to                          | 2            |
    | address   | bytes | [Address](../core/data_structures.md#address) of the validator that signed the extension | 3            |
    | height    | int64 | Height of the block  (for sanity check).                                                 | 4            |

* **Response**:

    | Name   | Type | Description                                           | Field Number |
    |--------|------|-------------------------------------------------------|--------------|
    | accept | bool | If false, Application is rejecting the vote extension | 1            |

* **Usage**:
    * If `ResponseVerifyVoteExtension.accept` is _false_, Tendermint will reject the whole received vote.
      See the [Properties](#properties) section to understand the potential liveness implications of this.
    * The implementation of `VerifyVoteExtension` MUST be deterministic. Moreover, the value of
      `ResponseVerifyVoteExtension.accept` MUST *exclusively* depend on the parameters passed in
      the call to `RequestVerifyVoteExtension`, and the last committed Application state
      (see [Properties](#properties) section below).
    * Moreover, application implementors SHOULD always set `ResponseVerifyVoteExtension.accept` to _true_,
      unless they _really_ know what the potential liveness implications of returning _false_ are.


#### When does Tendermint call it?

When a validator _p_ is in Tendermint consensus round _r_, height _h_, state _prevote_ (**TODO** discuss: I think I must remove the state
from this condition, but not sure), and _p_ receives a Precommit message for round _r_, height _h_ from _q_:

1. _p_'s Tendermint calls `RequestVerifyVoteExtension`.
2. The Application returns _accept_ or _reject_ via `ResponseVerifyVoteExtension.accept`.
3. If the Application returns
   * _accept_, _p_'s Tendermint will keep the received vote, together with its corresponding
     vote extension in its internal data structures. It will be used to:
       * calculate field _LastCommitHash_ in the header of the block proposed for height _h + 1_
         (in the rounds where _p_ will be proposer).
       * populate _LastCommitInfo_ in calls to `RequestPrepareProposal`, `RequestProcessProposal`,
         and `RequestFinalizeBlock` in height _h + 1_.
   * _reject_, _p_'s Tendermint will deem the Precommit message invalid and discard it.

### FinalizeBlock

#### Parameters and Types

* **Request**:

    | Name                 | Type                                        | Description                                                                                                       | Field Number |
    |----------------------|---------------------------------------------|-------------------------------------------------------------------------------------------------------------------|--------------|
    | hash                 | bytes                                       | The block header's hash. Present for convenience (can be derived from the block header).                          | 1            |
    | header               | [Header](../core/data_structures.md#header) | The block header.                                                                                                 | 2            |
    | tx                   | repeated bytes                              | List of transactions committed as part of the block.                                                              | 3            |
    | byzantine_validators | repeated [Evidence](#evidence)              | List of evidence of validators that acted maliciously.                                                            | 4            |
    | last_commit_info     | [LastCommitInfo](#lastcommitinfo)           | Info about the last commit, including the round, and the list of validators and which ones signed the last block. | 5            |

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
    * Contains a newly decided block.
    * This method is equivalent to the call sequence `BeginBlock`, [`DeliverTx`],
      `EndBlock`, `Commit` in the previous version of ABCI.
    * The header contains the height, timestamp, and more - it exactly matches the
      Tendermint block header.
    * The Application can use `RequestFinalizeBlock.last_commit_info` and `RequestFinalizeBlock.byzantine_validators`
      to determine rewards and punishments for the validators.
    * The application must execute the transactions in full, in the order they appear in `RequestFinalizeBlock.tx`,
      before returning control to Tendermint. Alternatively, it can commit the candidate state corresponding to the same block
      previously executed via `PrepareProposal` or `ProcessProposal`.
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
    * `ResponseFinalizeBlock.app_data` is included
        * [in next-block execution mode] as the `Header.AppHash` in the next block.
        * [in same-block execution mode] as the `Header.AppHash` in the current block. In this case,
          `PrepareProposal` is required to fully execute the block and set the App hash before
          returning the proposed block to Tendermint.
        * `ResponseFinalizeBlock.app_data` may also be empty or hard-coded, but MUST be
          **deterministic** - it must not be a function of anything that did not come from the parameters
          of `RequestFinalizeBlock` and the previous committed state.
    * Later calls to `Query` can return proofs about the application state anchored
      in this Merkle root hash.
    * Use `retain_height` with caution! If all nodes in the network remove historical
      blocks then this data is permanently lost, and no new nodes will be able to
      join the network and bootstrap. Historical blocks may also be required for
      other purposes, e.g. auditing, replay of non-persisted heights, light client
      verification, and so on.
    * Just as `ProcessProposal`, the implementation of `FinalizeBlock` MUST be deterministic, since it is
      making the Application's state evolve in the context of state machine replication.
    * Currently, Tendermint will fill up all fields in `RequestFinalizeBlock`, even if they were
      already passed on to the Application via `RequestPrepareProposal` or `RequestProcessProposal`.
      If the Application is in same-block execution mode, it applies the right candidate state here
      (rather than executing the whole block). In this case the Application disregards all parameters in
      `RequestFinalizeBlock` except `RequestFinalizeBlock.hash`.

#### When is it called?

When a validator _p_ is in Tendermint consensus height _h_, and _p_ receives

* the Proposal message with block _v_ for a round _r_, along with all its block parts, from _q_,
  which is the proposer of round _r_, height _h_,
* `Precommit` messages from _2f + 1_ validators' voting power for round _r_, height _h_,
  precommitting the same block _id(v)_,

then _p_'s Tendermint decides block _v_ and finalizes consensus for height _h_ in the following way

1. _p_'s Tendermint persists _v_ as decision for height _h_.
2. _p_'s Tendermint locks the mempool -- no calls to checkTx on new transactions.
3. _p_'s Tendermint calls `RequestFinalizeBlock` with _id(v)_. The call is synchronous.
4. _p_'s Application processes block _v_, received in a previous call to `RequestProcessProposal`.
5. _p_'s Application commits and persists the state resulting from processing the block.
6. _p_'s Application calculates and returns the _AppHash_, along with an array of arrays of bytes representing the output of each of the transactions
7. _p_'s Tendermint hashes the array of transaction outputs and stores it in _ResultHash_
8. _p_'s Tendermint persists _AppHash_ and _ResultHash_
9. _p_'s Tendermint unlocks the mempool -- newly received transactions can now be checked.
10. _p_'s starts consensus for a new height _h+1_, round 0

### [Points to discuss further]

#### Byzantine proposer

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

#### `ProcessProposal`'s timeout (a.k.a. Zarko's Github comment)

>**TODO** (to discuss): `PrepareProposal` is called synchronously. `ProcessProposal` may also want to fully process the block synchronously.
>However, they stand on Tendermint's critical path, so the Tendermint's Propose timeout needs to accomodate that.
>
>Idea: Make propose timestamp (currently hardcoded to 3 secs in the Tendermint Go implementation) part of ConsensusParams,
>so the App can adjust it with its knowledge of the time may take to prepare/process the proposal.

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

### TxAction

```proto
  enum TxAction {
    UNKNOWN       = 0;  // Unknown action
    UNMODIFIED    = 1;  // The Application did not modify this transaction. Ignore new_hashes field
    ADDED         = 2;  // The Application added this transaction. Ignore new_hashes field
    REMOVED       = 3;  // The Application wants this transaction removed from the proposal and the mempool. Use new_hashes field if the transaction was modified
  }
```

* **Usage**:
    * If `Action` is UNKNOWN, a problem happened in the Application. Tendermint will ignore this transaction. **TODO** should we panic?
    * If `Action` is UNMODIFIED, Tendermint includes the transaction in the proposal. Nothing to do on the mempool. Field `new_hashes` is ignored.
    * If `Action` is ADDED, Tendermint includes the transaction in the proposal. The transaction is also added to the mempool and gossipped. Field `new_hashes` is ignored.
    * If `Action` is REMOVED, Tendermint excludes the transaction from the proposal. The transaction is also removed from the mempool if it exists,
      similar to `CheckTx` returning _false_. Tendermint can use field `new_hashes` to help client trace transactions that have been modified into other transactions.

### TransactionRecord

* **Fields**:

    | Name       | Type                  | Description                                                      | Field Number |
    |------------|-----------------------|------------------------------------------------------------------|--------------|
    | action     | [TxAction](#txaction) | What should Tendermint do with this transaction?                 | 1            |
    | tx         | bytes                 | Transaction contents                                             | 2            |
    | new_hashes | repeated bytes        | List of hashes of successor transactions                         | 3            |

* **Usage**:
    * As `new_hashes` is a list, `TransactionRecord` allows to trace many-to-many modifications. Some examples:
        * Transaction $t1$ modified into $t2$ is represented with these records
            * $t2$ "ADDED"
            * $t1$ "REMOVED"; `new_hashes` contains [$id(t2)$]
        * Transaction $t1$ modified into $t2$ and $t3$ is represented with these `TransactionRecord` records
            * $t2$ "ADDED"
            * $t3$ "ADDED"
            * $t1$ "REMOVED"; `new_hashes` contains [$id(t2)$, $id(t3)$]
        * Transactions $t1$ and $t2$ aggregated into $t3$ is represented with these `TransactionRecord` records
            * $t3$ "ADDED"
            * $t1$ "REMOVED"; `new_hashes` contains [$id(t3)$]
            * $t2$ "REMOVED"; `new_hashes` contains [$id(t3)$]
        * Transactions $t1$ and $t2$ combined into $t3$ and $t4$ is represented with these `TransactionRecord` records
            * $t3$ "ADDED"
            * $t4$ "ADDED"
            * $t1$ "REMOVED" and `new_hashes` containing [$id(t3)$, $id(t4)$]
            * $t2$ "REMOVED" and `new_hashes` containing [$id(t3)$, $id(t4)$]

## Properties

### What Tendermint expects from the Application

Let $p$ and $q$ be two different correct proposers in rounds $r_p$ and $r_q$ respectively, in height $h$.
Let $s_{p,h-1}$ be $p$'s Application's state committed for height $h-1$.
Let $v_p$ (resp. $v_q$) be the block that $p$'s (resp. $q$'s) Tendermint passes on to the Application via `RequestPrepareProposal`
as proposer of round $r_p$ (resp $r_q$), height $h$.
Let $v'_p$ (resp. $v'_q$) the possibly modified block $p$'s (resp. $q$'s) Application returns via `ResponsePrepareProposal` to Tendermint.

The value proposed by $p$ can differ in two different rounds where $p$ is the proposer.

* Property 1 [`PrepareProposal`, header-changes] When the blockchain is in same-block execution mode,
  $p$'s Application can change the following parameters in `ResponsePrepareProposal`:
  _AppHash_, _ConsensusParams_, _ValidatorUpdates_.

Parameters _AppHash_, _ConsensusParams_, and _ValidatorUpdates_ are used by Tendermint to compute various hashes in
the block header that will finally be part of the proposal.

* Property 2 [`PrepareProposal`, no-header-changes] When the blockchain is in next-block execution
  mode, $p$'s Application cannot make any changes to the following parameters in `ResponsePrepareProposal`:
  _AppHash_, _ConsensusParams_, _ValidatorUpdates_.

In practical terms, Property 2 implies that Tendermint will ignore those parameters in `ResponsePrepareProposal`.

* Property 3 [`PrepareProposal`, `ProcessProposal`, coherence]: For any two correct processes $p$ and $q$,
  if $q$'s Tendermint calls `RequestProcessProposal` on $v'_p$,
  $q$'s Application returns Accept in `ResponseProcessProposal`.

Property 3 makes sure that blocks proposed by correct processes _always_ pass the correct receiving process's
`ProcessProposal` check.
On the other hand, if there is a deterministic bug in `PrepareProposal` or `ProcessProposal` (or in both),
strictly speaking, this makes all processes that hit the bug byzantine. This is a problem in practice,
as very often validators are running the Application from the same codebase, so potentially _all_ would
likely hit the bug at the same time. This would result in most (or all) processes prevoting `nil`, with the
serious consequences on Tendermint's liveness that this entails. Due to its criticality, Property 3 is a target for extensive testing and automated verification.

* Property 4 [`ProcessProposal`, determinism-1]: `ProcessProposal` is a (deterministic) function of the current state and the block that is about to be applied. In other words, for any correct process $p$, and any arbitrary block $v'$,
  if $p$'s Tendermint calls `RequestProcessProposal` on $v'$ at height $h$,
  then $p$'s Application's acceptance or rejection **exclusively** depends on $v'$ and $s_{p,h-1}$.

* Property 5 [`ProcessProposal`, determinism-2]: For any two correct processes $p$ and $q$, and any arbitrary block $v'$,
  if $p$'s (resp. $q$'s) Tendermint calls `RequestProcessProposal` on $v'$ at height $h$,
  then $p$'s Application accepts $v'$ if and only if $q$'s Application accepts $v'$.
  Note that this Property follows from Property 4 and the Agreement property of consensus.

Properties 4 and 5 ensure that all correct processes will react in the same way to a proposed block, even
if the proposer is Byzantine. However, `ProcessProposal` may contain a bug that renders the
acceptance or rejection of the block non-deterministic, and therefore prevents processes hitting
the bug from fulfilling Properties 4 or 5 (effectively making those processes Byzantine).
In such a scenario, Tendermint's liveness cannot be guaranteed.
Again, this is a problem in practice if most validators are running the same software, as they are likely
to hit the bug at the same point. There is currently no clear solution to help with this situation, so
the Application designers/implementors must proceed very carefully with the logic/implementation
of `ProcessProposal`. As a general rule `ProcessProposal` _should_ always accept the block.

According to the Tendermint algorithm, a correct process can broadcast at most one precommit message in round $r$, height $h$.
Since, as stated in the [Description](#description) section, `ResponseExtendVote` is only called when Tendermint
is about to broadcast a non-`nil` precommit message, a correct process can only produce one vote extension in round $r$, height $h$.
Let $e^r_p$ the vote extension that the Application of a correct process $p$ returns via `ResponseExtendVote` in round $r$, height $h$.
Let $w^r_p$ the proposed block that $p$'s Tendermint passes to the Application via `RequestExtendVote` in round $r$, height $h$.

* Property 6 [`ExtendVote`, `VerifyVoteExtension`, coherence]: For any two correct processes $p$ and $q$, if $q$ receives $e^r_p$
  from $p$ in height $h$, $q$'s Application returns Accept in `ResponseVerifyVoteExtension`.

Property 6 constrains the creation and handling of vote extensions in a similar way as Property 3
contrains the creation and handling of proposed blocks.
Property 6 ensures that extensions created by correct processes _always_ pass the `VerifyVoteExtension`
checks performed by correct processes receiving those extensions.
However, if there is a (deterministic) bug in `ExtendVote` or `VerifyVoteExtension` (or in both),
we will face the same liveness issues as described for Property 3, as Precommit messages with invalid vote
extensions will be discarded.

* Property 7 [`VerifyVoteExtension`, determinism-1]: For any correct process $p$,
  and any arbitrary vote extension $e$, and any arbitrary block $w$,
  if $p$'s (resp. $q$'s) Tendermint calls `RequestVerifyVoteExtension` on $e$ and $w$ at height $h$,
  then $p$'s Application's acceptance or rejection exclusively depends on $e$, $w$ and $s_{p,h-1}$.

* Property 8 [`VerifyVoteExtension`, determinism-2]: For any two correct processes $p$ and $q$,
  and any arbitrary vote extension $e$, and any arbitrary block $w$,
  if $p$'s (resp. $q$'s) Tendermint calls `RequestVerifyVoteExtension` on $e$ and $w$ at height $h$,
  then $p$'s Application accepts $e$ if and only if $q$'s Application accepts $e$.
  Note that this property follows from Property 7 and the Agreement property of consensus.

Properties 7 and 8 ensure that the validation of vote extensions will be deterministic at all
correct processes.
Properties 7 and 8 protect against arbitrary vote extension data from Byzantine processes
similarly to Properties 4 and 5 and proposed blocks.
Properties 7 and 8 can be violated by a bug inducing non-determinism in `ExtendVote` or
`VerifyVoteExtension`. In this case liveness can be compromised.
Extra care should be put in the implementation of `ExtendVote` and `VerifyVoteExtension` and,
as a general rule, `VerifyVoteExtension` _should_ always accept the vote extension.

* Property 9 [_all_, read-only]: $p$'s calls to `RequestPrepareProposal`, `RequestProcessProposal`,
  `RequestExtendVote`, and `RequestVerifyVoteExtension` at height $h$ do not modify $s_{p,h-1}$.

* Property 10 [`ExtendVote`, `FinalizeBlock`, non-dependency]: for any correct process $p$,
and any vote extension $e$ that $q$ received at height $h$, the computation of
$s{p,h}$ does not depend on $e$.

The call to correct process $p$'s `RequestFinalizeBlock` at height $h$, with block $v_{p,h}$
passed as parameter, creates state $s_{p,h}$.
Additionally, $p$'s `FinalizeBlock` creates a set of transaction results $T_{p,h}$.

>**TODO** I have left out all the "events" as they don't have any impact in safety or liveness
>(same for consensus params, and validator set)

* Property 11 [`FinalizeBlock`, determinism-1]: For any correct process $p$,
  the contents of $s_{p,h}$ exclusively depend on $s_{p,h-1}$ and $v_{p,h}$.

* Property 12 [`FinalizeBlock`, determinism-2]: For any correct process $p$,
  the contents of $T_{p,h}$ exclusively depend on $s_{p,h-1}$ and $v_{p,h}$.

Note that Properties 11 and 12, combined with Agreement property of consensus ensure
the Application state evolves consistently at all correct processes.

Finally, notice that neither `PrepareProposal` nor `ExtendVote` have determinism-related properties associated.
Indeed, `PrepareProposal` is not required to be deterministic:

* $v'_p$ may depend on $v_p$ and $s_{p,h-1}$, but may also depend on other values or operations.
* $v_p = v_q \nRightarrow v'_p = v'_q$.

Likewise, `ExtendVote` can also be non-deterministic:

* $e^r_p$ may depend on $w^r_p$ and $s_{p,h-1}$, but may also depend on other values or operations.
* $w^r_p = w^r_q \nRightarrow e^r_p = e^r_q$

### What the Application can expect from Tendermint

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

#### `PrepareProposal`:  Application's expectations

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

#### `ProcessProposal`:  Application's expectations

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

#### `ExtendVote`:  expectations from the Application

>**TODO** (in different rounds). [Finish first discussion above]

--

If `ProcessProposal`'s outcome is _Reject_ for some proposed block. Tendermint guarantees that the block will not be the decision.

--

The validity of every transaction in a block (from the App's point of view), as well as the hashes in its header can be guaranteed if:

* `ProcessProposal` *synchronously* handles every proposed block as though Tendermint had already decided on it.
* All the properties of `ProcessProposal` and `FinalizeBlock` mentioned above hold.

## Failure modes

>**TODO** Is it worth explaining the failure modes? Since we're going for halt, and can't configure them.

## Application modes

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
