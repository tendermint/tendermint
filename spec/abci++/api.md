# Protocol Documentation
<a name="top"></a>

## Table of Contents

- [tendermint/abci/types.proto](#tendermint_abci_types-proto)
    - [CommitInfo](#tendermint-abci-CommitInfo)
    - [Event](#tendermint-abci-Event)
    - [EventAttribute](#tendermint-abci-EventAttribute)
    - [ExecTxResult](#tendermint-abci-ExecTxResult)
    - [ExtendVoteExtension](#tendermint-abci-ExtendVoteExtension)
    - [ExtendedCommitInfo](#tendermint-abci-ExtendedCommitInfo)
    - [ExtendedVoteInfo](#tendermint-abci-ExtendedVoteInfo)
    - [Misbehavior](#tendermint-abci-Misbehavior)
    - [QuorumHashUpdate](#tendermint-abci-QuorumHashUpdate)
    - [Request](#tendermint-abci-Request)
    - [RequestApplySnapshotChunk](#tendermint-abci-RequestApplySnapshotChunk)
    - [RequestCheckTx](#tendermint-abci-RequestCheckTx)
    - [RequestCommit](#tendermint-abci-RequestCommit)
    - [RequestEcho](#tendermint-abci-RequestEcho)
    - [RequestExtendVote](#tendermint-abci-RequestExtendVote)
    - [RequestFinalizeBlock](#tendermint-abci-RequestFinalizeBlock)
    - [RequestFlush](#tendermint-abci-RequestFlush)
    - [RequestInfo](#tendermint-abci-RequestInfo)
    - [RequestInitChain](#tendermint-abci-RequestInitChain)
    - [RequestListSnapshots](#tendermint-abci-RequestListSnapshots)
    - [RequestLoadSnapshotChunk](#tendermint-abci-RequestLoadSnapshotChunk)
    - [RequestOfferSnapshot](#tendermint-abci-RequestOfferSnapshot)
    - [RequestPrepareProposal](#tendermint-abci-RequestPrepareProposal)
    - [RequestProcessProposal](#tendermint-abci-RequestProcessProposal)
    - [RequestQuery](#tendermint-abci-RequestQuery)
    - [RequestVerifyVoteExtension](#tendermint-abci-RequestVerifyVoteExtension)
    - [Response](#tendermint-abci-Response)
    - [ResponseApplySnapshotChunk](#tendermint-abci-ResponseApplySnapshotChunk)
    - [ResponseCheckTx](#tendermint-abci-ResponseCheckTx)
    - [ResponseCommit](#tendermint-abci-ResponseCommit)
    - [ResponseEcho](#tendermint-abci-ResponseEcho)
    - [ResponseException](#tendermint-abci-ResponseException)
    - [ResponseExtendVote](#tendermint-abci-ResponseExtendVote)
    - [ResponseFinalizeBlock](#tendermint-abci-ResponseFinalizeBlock)
    - [ResponseFlush](#tendermint-abci-ResponseFlush)
    - [ResponseInfo](#tendermint-abci-ResponseInfo)
    - [ResponseInitChain](#tendermint-abci-ResponseInitChain)
    - [ResponseListSnapshots](#tendermint-abci-ResponseListSnapshots)
    - [ResponseLoadSnapshotChunk](#tendermint-abci-ResponseLoadSnapshotChunk)
    - [ResponseOfferSnapshot](#tendermint-abci-ResponseOfferSnapshot)
    - [ResponsePrepareProposal](#tendermint-abci-ResponsePrepareProposal)
    - [ResponseProcessProposal](#tendermint-abci-ResponseProcessProposal)
    - [ResponseQuery](#tendermint-abci-ResponseQuery)
    - [ResponseVerifyVoteExtension](#tendermint-abci-ResponseVerifyVoteExtension)
    - [Snapshot](#tendermint-abci-Snapshot)
    - [ThresholdPublicKeyUpdate](#tendermint-abci-ThresholdPublicKeyUpdate)
    - [TxRecord](#tendermint-abci-TxRecord)
    - [TxResult](#tendermint-abci-TxResult)
    - [Validator](#tendermint-abci-Validator)
    - [ValidatorSetUpdate](#tendermint-abci-ValidatorSetUpdate)
    - [ValidatorUpdate](#tendermint-abci-ValidatorUpdate)
    - [VoteInfo](#tendermint-abci-VoteInfo)
  
    - [CheckTxType](#tendermint-abci-CheckTxType)
    - [MisbehaviorType](#tendermint-abci-MisbehaviorType)
    - [ResponseApplySnapshotChunk.Result](#tendermint-abci-ResponseApplySnapshotChunk-Result)
    - [ResponseOfferSnapshot.Result](#tendermint-abci-ResponseOfferSnapshot-Result)
    - [ResponseProcessProposal.ProposalStatus](#tendermint-abci-ResponseProcessProposal-ProposalStatus)
    - [ResponseVerifyVoteExtension.VerifyStatus](#tendermint-abci-ResponseVerifyVoteExtension-VerifyStatus)
    - [TxRecord.TxAction](#tendermint-abci-TxRecord-TxAction)
  
    - [ABCIApplication](#tendermint-abci-ABCIApplication)
  
- [Scalar Value Types](#scalar-value-types)



<a name="tendermint_abci_types-proto"></a>
<p align="right"><a href="#top">Top</a></p>

## tendermint/abci/types.proto



<a name="tendermint-abci-CommitInfo"></a>

### CommitInfo



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| round | [int32](#int32) |  |  |
| quorum_hash | [bytes](#bytes) |  |  |
| block_signature | [bytes](#bytes) |  |  |
| state_signature | [bytes](#bytes) |  |  |
| threshold_vote_extensions | [tendermint.types.VoteExtension](#tendermint-types-VoteExtension) | repeated |  |






<a name="tendermint-abci-Event"></a>

### Event
Event allows application developers to attach additional information to
ResponseCheckTx, ResponsePrepareProposal, ResponseProcessProposal
and ResponseFinalizeBlock.

Later, transactions may be queried using these events.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| type | [string](#string) |  |  |
| attributes | [EventAttribute](#tendermint-abci-EventAttribute) | repeated |  |






<a name="tendermint-abci-EventAttribute"></a>

### EventAttribute
EventAttribute is a single key-value pair, associated with an event.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| key | [string](#string) |  |  |
| value | [string](#string) |  |  |
| index | [bool](#bool) |  | nondeterministic |






<a name="tendermint-abci-ExecTxResult"></a>

### ExecTxResult
ExecTxResult contains results of executing one individual transaction.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| code | [uint32](#uint32) |  |  |
| data | [bytes](#bytes) |  |  |
| log | [string](#string) |  | nondeterministic |
| info | [string](#string) |  | nondeterministic |
| gas_wanted | [int64](#int64) |  |  |
| gas_used | [int64](#int64) |  |  |
| events | [Event](#tendermint-abci-Event) | repeated | nondeterministic |
| codespace | [string](#string) |  |  |






<a name="tendermint-abci-ExtendVoteExtension"></a>

### ExtendVoteExtension



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| type | [tendermint.types.VoteExtensionType](#tendermint-types-VoteExtensionType) |  |  |
| extension | [bytes](#bytes) |  |  |






<a name="tendermint-abci-ExtendedCommitInfo"></a>

### ExtendedCommitInfo
ExtendedCommitInfo is similar to CommitInfo except that it is only used in
the PrepareProposal request such that Tendermint can provide vote extensions
to the application.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| round | [int32](#int32) |  | The round at which the block proposer decided in the previous height. |
| quorum_hash | [bytes](#bytes) |  | List of validators&#39; addresses in the last validator set with their voting information, including vote extensions. |
| block_signature | [bytes](#bytes) |  |  |
| state_signature | [bytes](#bytes) |  |  |
| threshold_vote_extensions | [tendermint.types.VoteExtension](#tendermint-types-VoteExtension) | repeated |  |






<a name="tendermint-abci-ExtendedVoteInfo"></a>

### ExtendedVoteInfo
ExtendedVoteInfo


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| validator | [Validator](#tendermint-abci-Validator) |  | The validator that sent the vote. |
| signed_last_block | [bool](#bool) |  | Indicates whether the validator signed the last block, allowing for rewards based on validator availability. |
| vote_extension | [bytes](#bytes) |  | Non-deterministic extension provided by the sending validator&#39;s application. |






<a name="tendermint-abci-Misbehavior"></a>

### Misbehavior



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| type | [MisbehaviorType](#tendermint-abci-MisbehaviorType) |  |  |
| validator | [Validator](#tendermint-abci-Validator) |  | The offending validator |
| height | [int64](#int64) |  | The height when the offense occurred |
| time | [google.protobuf.Timestamp](#google-protobuf-Timestamp) |  | The corresponding time where the offense occurred |
| total_voting_power | [int64](#int64) |  | Total voting power of the validator set in case the ABCI application does not store historical validators. https://github.com/tendermint/tendermint/issues/4581 |






<a name="tendermint-abci-QuorumHashUpdate"></a>

### QuorumHashUpdate



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| quorum_hash | [bytes](#bytes) |  |  |






<a name="tendermint-abci-Request"></a>

### Request



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| echo | [RequestEcho](#tendermint-abci-RequestEcho) |  |  |
| flush | [RequestFlush](#tendermint-abci-RequestFlush) |  |  |
| info | [RequestInfo](#tendermint-abci-RequestInfo) |  |  |
| init_chain | [RequestInitChain](#tendermint-abci-RequestInitChain) |  |  |
| query | [RequestQuery](#tendermint-abci-RequestQuery) |  |  |
| check_tx | [RequestCheckTx](#tendermint-abci-RequestCheckTx) |  |  |
| commit | [RequestCommit](#tendermint-abci-RequestCommit) |  |  |
| list_snapshots | [RequestListSnapshots](#tendermint-abci-RequestListSnapshots) |  |  |
| offer_snapshot | [RequestOfferSnapshot](#tendermint-abci-RequestOfferSnapshot) |  |  |
| load_snapshot_chunk | [RequestLoadSnapshotChunk](#tendermint-abci-RequestLoadSnapshotChunk) |  |  |
| apply_snapshot_chunk | [RequestApplySnapshotChunk](#tendermint-abci-RequestApplySnapshotChunk) |  |  |
| prepare_proposal | [RequestPrepareProposal](#tendermint-abci-RequestPrepareProposal) |  |  |
| process_proposal | [RequestProcessProposal](#tendermint-abci-RequestProcessProposal) |  |  |
| extend_vote | [RequestExtendVote](#tendermint-abci-RequestExtendVote) |  |  |
| verify_vote_extension | [RequestVerifyVoteExtension](#tendermint-abci-RequestVerifyVoteExtension) |  |  |
| finalize_block | [RequestFinalizeBlock](#tendermint-abci-RequestFinalizeBlock) |  |  |






<a name="tendermint-abci-RequestApplySnapshotChunk"></a>

### RequestApplySnapshotChunk
Applies a snapshot chunk


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| index | [uint32](#uint32) |  |  |
| chunk | [bytes](#bytes) |  |  |
| sender | [string](#string) |  |  |






<a name="tendermint-abci-RequestCheckTx"></a>

### RequestCheckTx



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| tx | [bytes](#bytes) |  |  |
| type | [CheckTxType](#tendermint-abci-CheckTxType) |  |  |






<a name="tendermint-abci-RequestCommit"></a>

### RequestCommit







<a name="tendermint-abci-RequestEcho"></a>

### RequestEcho



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| message | [string](#string) |  | A string to echo back |






<a name="tendermint-abci-RequestExtendVote"></a>

### RequestExtendVote
Extends a vote with application-side injection


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| hash | [bytes](#bytes) |  |  |
| height | [int64](#int64) |  |  |






<a name="tendermint-abci-RequestFinalizeBlock"></a>

### RequestFinalizeBlock



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| txs | [bytes](#bytes) | repeated | List of transactions committed as part of the block. |
| decided_last_commit | [CommitInfo](#tendermint-abci-CommitInfo) |  | Info about the last commit, obtained from the block that was just decided. |
| misbehavior | [Misbehavior](#tendermint-abci-Misbehavior) | repeated | List of information about validators that acted incorrectly. |
| hash | [bytes](#bytes) |  | The block header&#39;s hash. Present for convenience (can be derived from the block header). |
| height | [int64](#int64) |  | The height of the finalized block. |
| time | [google.protobuf.Timestamp](#google-protobuf-Timestamp) |  | Timestamp included in the finalized block. |
| next_validators_hash | [bytes](#bytes) |  | Merkle root of the next validator set. |
| core_chain_locked_height | [uint32](#uint32) |  | Core chain lock height to be used when signing this block. |
| proposer_pro_tx_hash | [bytes](#bytes) |  | ProTXHash of the original proposer of the block. |
| proposed_app_version | [uint64](#uint64) |  | Proposer&#39;s latest available app protocol version. |
| version | [tendermint.version.Consensus](#tendermint-version-Consensus) |  | App and block version used to generate the block. |
| app_hash | [bytes](#bytes) |  | The Merkle root hash of the application state after committing the block. |






<a name="tendermint-abci-RequestFlush"></a>

### RequestFlush







<a name="tendermint-abci-RequestInfo"></a>

### RequestInfo



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| version | [string](#string) |  |  |
| block_version | [uint64](#uint64) |  |  |
| p2p_version | [uint64](#uint64) |  |  |
| abci_version | [string](#string) |  |  |






<a name="tendermint-abci-RequestInitChain"></a>

### RequestInitChain



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| time | [google.protobuf.Timestamp](#google-protobuf-Timestamp) |  | Genesis time |
| chain_id | [string](#string) |  | ID of the blockchain. |
| consensus_params | [tendermint.types.ConsensusParams](#tendermint-types-ConsensusParams) |  | Initial consensus-critical parameters. |
| validator_set | [ValidatorSetUpdate](#tendermint-abci-ValidatorSetUpdate) |  | Initial genesis validators, sorted by voting power. |
| app_state_bytes | [bytes](#bytes) |  | Serialized initial application state. JSON bytes. |
| initial_height | [int64](#int64) |  | Height of the initial block (typically `1`). |
| initial_core_height | [uint32](#uint32) |  | Initial core chain lock height. |






<a name="tendermint-abci-RequestListSnapshots"></a>

### RequestListSnapshots
lists available snapshots






<a name="tendermint-abci-RequestLoadSnapshotChunk"></a>

### RequestLoadSnapshotChunk
loads a snapshot chunk


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| height | [uint64](#uint64) |  |  |
| format | [uint32](#uint32) |  |  |
| chunk | [uint32](#uint32) |  |  |






<a name="tendermint-abci-RequestOfferSnapshot"></a>

### RequestOfferSnapshot
offers a snapshot to the application


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| snapshot | [Snapshot](#tendermint-abci-Snapshot) |  | snapshot offered by peers |
| app_hash | [bytes](#bytes) |  | light client-verified app hash for snapshot height |






<a name="tendermint-abci-RequestPrepareProposal"></a>

### RequestPrepareProposal



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| max_tx_bytes | [int64](#int64) |  | Currently configured maximum size in bytes taken by the modified transactions. The modified transactions cannot exceed this size. |
| txs | [bytes](#bytes) | repeated | Preliminary list of transactions that have been picked as part of the block to propose. Sent to the app for possible modifications. |
| local_last_commit | [ExtendedCommitInfo](#tendermint-abci-ExtendedCommitInfo) |  | Info about the last commit, obtained locally from Tendermint&#39;s data structures. |
| misbehavior | [Misbehavior](#tendermint-abci-Misbehavior) | repeated | List of information about validators that acted incorrectly. |
| height | [int64](#int64) |  | The height of the block that will be proposed. |
| time | [google.protobuf.Timestamp](#google-protobuf-Timestamp) |  | Timestamp of the block that that will be proposed. |
| next_validators_hash | [bytes](#bytes) |  | Merkle root of the next validator set. |
| core_chain_locked_height | [uint32](#uint32) |  | Core chain lock height to be used when signing this block. |
| proposer_pro_tx_hash | [bytes](#bytes) |  | ProTXHash of the original proposer of the block. |
| proposed_app_version | [uint64](#uint64) |  | Proposer&#39;s latest available app protocol version. |
| version | [tendermint.version.Consensus](#tendermint-version-Consensus) |  | App and block version used to generate the block. |






<a name="tendermint-abci-RequestProcessProposal"></a>

### RequestProcessProposal



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| txs | [bytes](#bytes) | repeated | List of transactions that have been picked as part of the proposed |
| proposed_last_commit | [CommitInfo](#tendermint-abci-CommitInfo) |  | Info about the last commit, obtained from the information in the proposed block. |
| misbehavior | [Misbehavior](#tendermint-abci-Misbehavior) | repeated | List of information about validators that acted incorrectly. |
| hash | [bytes](#bytes) |  | The block header&#39;s hash of the proposed block. |
| height | [int64](#int64) |  | The height of the proposed block. |
| time | [google.protobuf.Timestamp](#google-protobuf-Timestamp) |  | Timestamp included in the proposed block. |
| next_validators_hash | [bytes](#bytes) |  | Merkle root of the next validator set. |
| core_chain_locked_height | [uint32](#uint32) |  | Core chain lock height to be used when signing this block. |
| proposer_pro_tx_hash | [bytes](#bytes) |  | ProTXHash of the original proposer of the block. |
| proposed_app_version | [uint64](#uint64) |  | Proposer&#39;s latest available app protocol version. |
| version | [tendermint.version.Consensus](#tendermint-version-Consensus) |  | App and block version used to generate the block. |






<a name="tendermint-abci-RequestQuery"></a>

### RequestQuery



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| data | [bytes](#bytes) |  |  |
| path | [string](#string) |  |  |
| height | [int64](#int64) |  |  |
| prove | [bool](#bool) |  |  |






<a name="tendermint-abci-RequestVerifyVoteExtension"></a>

### RequestVerifyVoteExtension
Verify the vote extension


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| hash | [bytes](#bytes) |  |  |
| validator_pro_tx_hash | [bytes](#bytes) |  |  |
| height | [int64](#int64) |  |  |
| vote_extensions | [ExtendVoteExtension](#tendermint-abci-ExtendVoteExtension) | repeated |  |






<a name="tendermint-abci-Response"></a>

### Response



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| exception | [ResponseException](#tendermint-abci-ResponseException) |  |  |
| echo | [ResponseEcho](#tendermint-abci-ResponseEcho) |  |  |
| flush | [ResponseFlush](#tendermint-abci-ResponseFlush) |  |  |
| info | [ResponseInfo](#tendermint-abci-ResponseInfo) |  |  |
| init_chain | [ResponseInitChain](#tendermint-abci-ResponseInitChain) |  |  |
| query | [ResponseQuery](#tendermint-abci-ResponseQuery) |  |  |
| check_tx | [ResponseCheckTx](#tendermint-abci-ResponseCheckTx) |  |  |
| commit | [ResponseCommit](#tendermint-abci-ResponseCommit) |  |  |
| list_snapshots | [ResponseListSnapshots](#tendermint-abci-ResponseListSnapshots) |  |  |
| offer_snapshot | [ResponseOfferSnapshot](#tendermint-abci-ResponseOfferSnapshot) |  |  |
| load_snapshot_chunk | [ResponseLoadSnapshotChunk](#tendermint-abci-ResponseLoadSnapshotChunk) |  |  |
| apply_snapshot_chunk | [ResponseApplySnapshotChunk](#tendermint-abci-ResponseApplySnapshotChunk) |  |  |
| prepare_proposal | [ResponsePrepareProposal](#tendermint-abci-ResponsePrepareProposal) |  |  |
| process_proposal | [ResponseProcessProposal](#tendermint-abci-ResponseProcessProposal) |  |  |
| extend_vote | [ResponseExtendVote](#tendermint-abci-ResponseExtendVote) |  |  |
| verify_vote_extension | [ResponseVerifyVoteExtension](#tendermint-abci-ResponseVerifyVoteExtension) |  |  |
| finalize_block | [ResponseFinalizeBlock](#tendermint-abci-ResponseFinalizeBlock) |  |  |






<a name="tendermint-abci-ResponseApplySnapshotChunk"></a>

### ResponseApplySnapshotChunk



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| result | [ResponseApplySnapshotChunk.Result](#tendermint-abci-ResponseApplySnapshotChunk-Result) |  |  |
| refetch_chunks | [uint32](#uint32) | repeated | Chunks to refetch and reapply |
| reject_senders | [string](#string) | repeated | Chunk senders to reject and ban |






<a name="tendermint-abci-ResponseCheckTx"></a>

### ResponseCheckTx



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| code | [uint32](#uint32) |  |  |
| data | [bytes](#bytes) |  |  |
| gas_wanted | [int64](#int64) |  |  |
| codespace | [string](#string) |  |  |
| sender | [string](#string) |  |  |
| priority | [int64](#int64) |  |  |
| mempool_error | [string](#string) |  | ABCI applications creating a ResponseCheckTX should not set mempool_error. |






<a name="tendermint-abci-ResponseCommit"></a>

### ResponseCommit



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| retain_height | [int64](#int64) |  |  |






<a name="tendermint-abci-ResponseEcho"></a>

### ResponseEcho



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| message | [string](#string) |  |  |






<a name="tendermint-abci-ResponseException"></a>

### ResponseException
nondeterministic


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| error | [string](#string) |  |  |






<a name="tendermint-abci-ResponseExtendVote"></a>

### ResponseExtendVote



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| vote_extensions | [ExtendVoteExtension](#tendermint-abci-ExtendVoteExtension) | repeated |  |






<a name="tendermint-abci-ResponseFinalizeBlock"></a>

### ResponseFinalizeBlock
In same-block execution mode, Tendermint will log an error and ignore values for ResponseFinalizeBlock.app_hash,
ResponseFinalizeBlock.tx_results, ResponseFinalizeBlock.validator_updates, and ResponsePrepareProposal.consensus_param_updates,
as those must have been provided by PrepareProposal.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| events | [Event](#tendermint-abci-Event) | repeated | Type &amp; Key-Value events for indexing |
| retain_height | [int64](#int64) |  | Blocks below this height may be removed. Defaults to `0` (retain all). |






<a name="tendermint-abci-ResponseFlush"></a>

### ResponseFlush







<a name="tendermint-abci-ResponseInfo"></a>

### ResponseInfo



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| data | [string](#string) |  |  |
| version | [string](#string) |  | this is the software version of the application. TODO: remove? |
| app_version | [uint64](#uint64) |  |  |
| last_block_height | [int64](#int64) |  |  |
| last_block_app_hash | [bytes](#bytes) |  |  |






<a name="tendermint-abci-ResponseInitChain"></a>

### ResponseInitChain



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| consensus_params | [tendermint.types.ConsensusParams](#tendermint-types-ConsensusParams) |  | Initial consensus-critical parameters (optional). |
| app_hash | [bytes](#bytes) |  | Initial application hash. |
| validator_set_update | [ValidatorSetUpdate](#tendermint-abci-ValidatorSetUpdate) |  | Initial validator set (optional). |
| next_core_chain_lock_update | [tendermint.types.CoreChainLock](#tendermint-types-CoreChainLock) |  | Initial core chain lock update. |
| initial_core_height | [uint32](#uint32) |  | Initial height of core lock. |






<a name="tendermint-abci-ResponseListSnapshots"></a>

### ResponseListSnapshots



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| snapshots | [Snapshot](#tendermint-abci-Snapshot) | repeated |  |






<a name="tendermint-abci-ResponseLoadSnapshotChunk"></a>

### ResponseLoadSnapshotChunk



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| chunk | [bytes](#bytes) |  |  |






<a name="tendermint-abci-ResponseOfferSnapshot"></a>

### ResponseOfferSnapshot



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| result | [ResponseOfferSnapshot.Result](#tendermint-abci-ResponseOfferSnapshot-Result) |  |  |






<a name="tendermint-abci-ResponsePrepareProposal"></a>

### ResponsePrepareProposal



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| tx_records | [TxRecord](#tendermint-abci-TxRecord) | repeated | Possibly modified list of transactions that have been picked as part of the proposed block. |
| app_hash | [bytes](#bytes) |  | The Merkle root hash of the application state. |
| tx_results | [ExecTxResult](#tendermint-abci-ExecTxResult) | repeated | List of structures containing the data resulting from executing the transactions. |
| consensus_param_updates | [tendermint.types.ConsensusParams](#tendermint-types-ConsensusParams) |  | Changes to consensus-critical gas, size, and other parameters that will be applied at next height. |
| core_chain_lock_update | [tendermint.types.CoreChainLock](#tendermint-types-CoreChainLock) |  | Core chain lock that will be used for generated block. |
| validator_set_update | [ValidatorSetUpdate](#tendermint-abci-ValidatorSetUpdate) |  | Changes to validator set that will be applied at next height. |






<a name="tendermint-abci-ResponseProcessProposal"></a>

### ResponseProcessProposal



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| status | [ResponseProcessProposal.ProposalStatus](#tendermint-abci-ResponseProcessProposal-ProposalStatus) |  | `enum` that signals if the application finds the proposal valid. |
| app_hash | [bytes](#bytes) |  | The Merkle root hash of the application state. |
| tx_results | [ExecTxResult](#tendermint-abci-ExecTxResult) | repeated | List of structures containing the data resulting from executing the transactions. |
| consensus_param_updates | [tendermint.types.ConsensusParams](#tendermint-types-ConsensusParams) |  | Changes to consensus-critical gas, size, and other parameters. |
| core_chain_lock_update | [tendermint.types.CoreChainLock](#tendermint-types-CoreChainLock) |  | Core chain lock that will be used for generated block. |
| validator_set_update | [ValidatorSetUpdate](#tendermint-abci-ValidatorSetUpdate) |  | Changes to validator set (set voting power to 0 to remove). |






<a name="tendermint-abci-ResponseQuery"></a>

### ResponseQuery



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| code | [uint32](#uint32) |  |  |
| log | [string](#string) |  | bytes data = 2; // use &#34;value&#34; instead.

nondeterministic |
| info | [string](#string) |  | nondeterministic |
| index | [int64](#int64) |  |  |
| key | [bytes](#bytes) |  |  |
| value | [bytes](#bytes) |  |  |
| proof_ops | [tendermint.crypto.ProofOps](#tendermint-crypto-ProofOps) |  |  |
| height | [int64](#int64) |  |  |
| codespace | [string](#string) |  |  |






<a name="tendermint-abci-ResponseVerifyVoteExtension"></a>

### ResponseVerifyVoteExtension



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| status | [ResponseVerifyVoteExtension.VerifyStatus](#tendermint-abci-ResponseVerifyVoteExtension-VerifyStatus) |  |  |






<a name="tendermint-abci-Snapshot"></a>

### Snapshot



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| height | [uint64](#uint64) |  | The height at which the snapshot was taken |
| format | [uint32](#uint32) |  | The application-specific snapshot format |
| chunks | [uint32](#uint32) |  | Number of chunks in the snapshot |
| hash | [bytes](#bytes) |  | Arbitrary snapshot hash, equal only if identical |
| metadata | [bytes](#bytes) |  | Arbitrary application metadata |
| core_chain_locked_height | [uint32](#uint32) |  | The core chain locked height |






<a name="tendermint-abci-ThresholdPublicKeyUpdate"></a>

### ThresholdPublicKeyUpdate



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| threshold_public_key | [tendermint.crypto.PublicKey](#tendermint-crypto-PublicKey) |  |  |






<a name="tendermint-abci-TxRecord"></a>

### TxRecord



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| action | [TxRecord.TxAction](#tendermint-abci-TxRecord-TxAction) |  |  |
| tx | [bytes](#bytes) |  |  |






<a name="tendermint-abci-TxResult"></a>

### TxResult
TxResult contains results of executing the transaction.

One usage is indexing transaction results.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| height | [int64](#int64) |  |  |
| index | [uint32](#uint32) |  |  |
| tx | [bytes](#bytes) |  |  |
| result | [ExecTxResult](#tendermint-abci-ExecTxResult) |  |  |






<a name="tendermint-abci-Validator"></a>

### Validator
Validator


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| power | [int64](#int64) |  | bytes address = 1; // The first 20 bytes of SHA256(public key) PubKey pub_key = 2 [(gogoproto.nullable)=false];

The voting power |
| pro_tx_hash | [bytes](#bytes) |  |  |






<a name="tendermint-abci-ValidatorSetUpdate"></a>

### ValidatorSetUpdate



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| validator_updates | [ValidatorUpdate](#tendermint-abci-ValidatorUpdate) | repeated |  |
| threshold_public_key | [tendermint.crypto.PublicKey](#tendermint-crypto-PublicKey) |  |  |
| quorum_hash | [bytes](#bytes) |  |  |






<a name="tendermint-abci-ValidatorUpdate"></a>

### ValidatorUpdate
ValidatorUpdate


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| pub_key | [tendermint.crypto.PublicKey](#tendermint-crypto-PublicKey) |  |  |
| power | [int64](#int64) |  |  |
| pro_tx_hash | [bytes](#bytes) |  |  |
| node_address | [string](#string) |  | address of the Validator, correct URI |






<a name="tendermint-abci-VoteInfo"></a>

### VoteInfo
VoteInfo


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| validator | [Validator](#tendermint-abci-Validator) |  |  |
| signed_last_block | [bool](#bool) |  |  |





 


<a name="tendermint-abci-CheckTxType"></a>

### CheckTxType


| Name | Number | Description |
| ---- | ------ | ----------- |
| NEW | 0 |  |
| RECHECK | 1 |  |



<a name="tendermint-abci-MisbehaviorType"></a>

### MisbehaviorType


| Name | Number | Description |
| ---- | ------ | ----------- |
| UNKNOWN | 0 |  |
| DUPLICATE_VOTE | 1 |  |
| LIGHT_CLIENT_ATTACK | 2 |  |



<a name="tendermint-abci-ResponseApplySnapshotChunk-Result"></a>

### ResponseApplySnapshotChunk.Result


| Name | Number | Description |
| ---- | ------ | ----------- |
| UNKNOWN | 0 | Unknown result, abort all snapshot restoration |
| ACCEPT | 1 | Chunk successfully accepted |
| ABORT | 2 | Abort all snapshot restoration |
| RETRY | 3 | Retry chunk (combine with refetch and reject) |
| RETRY_SNAPSHOT | 4 | Retry snapshot (combine with refetch and reject) |
| REJECT_SNAPSHOT | 5 | Reject this snapshot, try others |



<a name="tendermint-abci-ResponseOfferSnapshot-Result"></a>

### ResponseOfferSnapshot.Result


| Name | Number | Description |
| ---- | ------ | ----------- |
| UNKNOWN | 0 | Unknown result, abort all snapshot restoration |
| ACCEPT | 1 | Snapshot accepted, apply chunks |
| ABORT | 2 | Abort all snapshot restoration |
| REJECT | 3 | Reject this specific snapshot, try others |
| REJECT_FORMAT | 4 | Reject all snapshots of this format, try others |
| REJECT_SENDER | 5 | Reject all snapshots from the sender(s), try others |



<a name="tendermint-abci-ResponseProcessProposal-ProposalStatus"></a>

### ResponseProcessProposal.ProposalStatus


| Name | Number | Description |
| ---- | ------ | ----------- |
| UNKNOWN | 0 | Unspecified error occurred |
| ACCEPT | 1 | Proposal accepted |
| REJECT | 2 | Proposal is not valid; prevoting `nil` |



<a name="tendermint-abci-ResponseVerifyVoteExtension-VerifyStatus"></a>

### ResponseVerifyVoteExtension.VerifyStatus


| Name | Number | Description |
| ---- | ------ | ----------- |
| UNKNOWN | 0 |  |
| ACCEPT | 1 |  |
| REJECT | 2 |  |



<a name="tendermint-abci-TxRecord-TxAction"></a>

### TxRecord.TxAction
TxAction contains App-provided information on what to do with a transaction that is part of a raw proposal

| Name | Number | Description |
| ---- | ------ | ----------- |
| UNKNOWN | 0 | Unknown action |
| UNMODIFIED | 1 | The Application did not modify this transaction. |
| ADDED | 2 | The Application added this transaction. |
| REMOVED | 3 | The Application wants this transaction removed from the proposal and the mempool. |


 

 


<a name="tendermint-abci-ABCIApplication"></a>

### ABCIApplication


| Method Name | Request Type | Response Type | Description |
| ----------- | ------------ | ------------- | ------------|
| Echo | [RequestEcho](#tendermint-abci-RequestEcho) | [ResponseEcho](#tendermint-abci-ResponseEcho) | Echo a string to test an abci client/server implementation |
| Flush | [RequestFlush](#tendermint-abci-RequestFlush) | [ResponseFlush](#tendermint-abci-ResponseFlush) |  |
| Info | [RequestInfo](#tendermint-abci-RequestInfo) | [ResponseInfo](#tendermint-abci-ResponseInfo) |  |
| CheckTx | [RequestCheckTx](#tendermint-abci-RequestCheckTx) | [ResponseCheckTx](#tendermint-abci-ResponseCheckTx) |  |
| Query | [RequestQuery](#tendermint-abci-RequestQuery) | [ResponseQuery](#tendermint-abci-ResponseQuery) |  |
| Commit | [RequestCommit](#tendermint-abci-RequestCommit) | [ResponseCommit](#tendermint-abci-ResponseCommit) |  |
| InitChain | [RequestInitChain](#tendermint-abci-RequestInitChain) | [ResponseInitChain](#tendermint-abci-ResponseInitChain) |  |
| ListSnapshots | [RequestListSnapshots](#tendermint-abci-RequestListSnapshots) | [ResponseListSnapshots](#tendermint-abci-ResponseListSnapshots) |  |
| OfferSnapshot | [RequestOfferSnapshot](#tendermint-abci-RequestOfferSnapshot) | [ResponseOfferSnapshot](#tendermint-abci-ResponseOfferSnapshot) |  |
| LoadSnapshotChunk | [RequestLoadSnapshotChunk](#tendermint-abci-RequestLoadSnapshotChunk) | [ResponseLoadSnapshotChunk](#tendermint-abci-ResponseLoadSnapshotChunk) |  |
| ApplySnapshotChunk | [RequestApplySnapshotChunk](#tendermint-abci-RequestApplySnapshotChunk) | [ResponseApplySnapshotChunk](#tendermint-abci-ResponseApplySnapshotChunk) |  |
| PrepareProposal | [RequestPrepareProposal](#tendermint-abci-RequestPrepareProposal) | [ResponsePrepareProposal](#tendermint-abci-ResponsePrepareProposal) |  |
| ProcessProposal | [RequestProcessProposal](#tendermint-abci-RequestProcessProposal) | [ResponseProcessProposal](#tendermint-abci-ResponseProcessProposal) |  |
| ExtendVote | [RequestExtendVote](#tendermint-abci-RequestExtendVote) | [ResponseExtendVote](#tendermint-abci-ResponseExtendVote) |  |
| VerifyVoteExtension | [RequestVerifyVoteExtension](#tendermint-abci-RequestVerifyVoteExtension) | [ResponseVerifyVoteExtension](#tendermint-abci-ResponseVerifyVoteExtension) |  |
| FinalizeBlock | [RequestFinalizeBlock](#tendermint-abci-RequestFinalizeBlock) | [ResponseFinalizeBlock](#tendermint-abci-ResponseFinalizeBlock) |  |

 



## Scalar Value Types

| .proto Type | Notes | C++ | Java | Python | Go | C# | PHP | Ruby |
| ----------- | ----- | --- | ---- | ------ | -- | -- | --- | ---- |
| <a name="double" /> double |  | double | double | float | float64 | double | float | Float |
| <a name="float" /> float |  | float | float | float | float32 | float | float | Float |
| <a name="int32" /> int32 | Uses variable-length encoding. Inefficient for encoding negative numbers – if your field is likely to have negative values, use sint32 instead. | int32 | int | int | int32 | int | integer | Bignum or Fixnum (as required) |
| <a name="int64" /> int64 | Uses variable-length encoding. Inefficient for encoding negative numbers – if your field is likely to have negative values, use sint64 instead. | int64 | long | int/long | int64 | long | integer/string | Bignum |
| <a name="uint32" /> uint32 | Uses variable-length encoding. | uint32 | int | int/long | uint32 | uint | integer | Bignum or Fixnum (as required) |
| <a name="uint64" /> uint64 | Uses variable-length encoding. | uint64 | long | int/long | uint64 | ulong | integer/string | Bignum or Fixnum (as required) |
| <a name="sint32" /> sint32 | Uses variable-length encoding. Signed int value. These more efficiently encode negative numbers than regular int32s. | int32 | int | int | int32 | int | integer | Bignum or Fixnum (as required) |
| <a name="sint64" /> sint64 | Uses variable-length encoding. Signed int value. These more efficiently encode negative numbers than regular int64s. | int64 | long | int/long | int64 | long | integer/string | Bignum |
| <a name="fixed32" /> fixed32 | Always four bytes. More efficient than uint32 if values are often greater than 2^28. | uint32 | int | int | uint32 | uint | integer | Bignum or Fixnum (as required) |
| <a name="fixed64" /> fixed64 | Always eight bytes. More efficient than uint64 if values are often greater than 2^56. | uint64 | long | int/long | uint64 | ulong | integer/string | Bignum |
| <a name="sfixed32" /> sfixed32 | Always four bytes. | int32 | int | int | int32 | int | integer | Bignum or Fixnum (as required) |
| <a name="sfixed64" /> sfixed64 | Always eight bytes. | int64 | long | int/long | int64 | long | integer/string | Bignum |
| <a name="bool" /> bool |  | bool | boolean | boolean | bool | bool | boolean | TrueClass/FalseClass |
| <a name="string" /> string | A string must always contain UTF-8 encoded or 7-bit ASCII text. | string | String | str/unicode | string | string | string | String (UTF-8) |
| <a name="bytes" /> bytes | May contain any arbitrary sequence of bytes. | string | ByteString | str | []byte | ByteString | string | String (ASCII-8BIT) |

