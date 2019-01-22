## v0.29.0

*January 21, 2019*

Special thanks to external contributors on this release:
@bradyjoestar, @kunaldhariwal, @gauthamzz, @hrharder

This release is primarily about making some breaking changes to
the Block protocol version before Cosmos launch, and to fixing more issues
in the proposer selection algorithm discovered on Cosmos testnets.

The Block protocol changes include using a standard Merkle tree format (RFC 6962),
fixing some inconsistencies between field orders in Vote and Proposal structs,
and constraining the hash of the ConsensusParams to include only a few fields.

The proposer selection algorithm saw significant progress,
including a [formal proof by @cwgoes for the base-case in Idris](https://github.com/cwgoes/tm-proposer-idris)
and a [much more detailed specification (still in progress) by
@ancazamfir](https://github.com/tendermint/tendermint/pull/3140).

Fixes to the proposer selection algorithm include normalizing the proposer
priorities to mitigate the effects of large changes to the validator set.
That said, we just discovered [another bug](https://github.com/tendermint/tendermint/issues/3181),
which will be fixed in the next breaking release.

While we are trying to stabilize the Block protocol to preserve compatibility
with old chains, there may be some final changes yet to come before Cosmos
launch as we continue to audit and test the software.

Friendly reminder, we have a [bug bounty
program](https://hackerone.com/tendermint).

### BREAKING CHANGES:

* CLI/RPC/Config

* Apps
- [state] \#3049 Total voting power of the validator set is upper bounded by
    `MaxInt64 / 8`. Apps must ensure they do not return changes to the validator
    set that cause this maximum to be exceeded.

* Go API
- [node] \#3082 MetricsProvider now requires you to pass a chain ID
- [types] \#2713 Rename `TxProof.LeafHash` to `TxProof.Leaf`
- [crypto/merkle] \#2713 `SimpleProof.Verify` takes a `leaf` instead of a
  `leafHash` and performs the hashing itself

* Blockchain Protocol
  * [crypto/merkle] \#2713 Merkle trees now match the RFC 6962 specification
  * [types] \#3078 Re-order Timestamp and BlockID in CanonicalVote so it's
    consistent with CanonicalProposal (BlockID comes
    first)
  * [types] \#3165 Hash of ConsensusParams only includes BlockSize.MaxBytes and
    BlockSize.MaxGas

* P2P Protocol
  - [consensus] \#3049 Normalize priorities to not exceed `2*TotalVotingPower` to mitigate unfair proposer selection
    heavily preferring earlier joined validators in the case of an early bonded large validator unbonding

### FEATURES:

### IMPROVEMENTS:
- [rpc] \#3065 Return maxPerPage (100), not defaultPerPage (30) if `per_page` is greater than the max 100.
- [instrumentation] \#3082 Add `chain_id` label for all metrics

### BUG FIXES:
- [crypto] \#3164 Update `btcd` fork for rare signRFC6979 bug
- [lite] \#3171 Fix verifying large validator set changes
- [log] \#3125 Fix year format
- [mempool] \#3168 Limit tx size to fit in the max reactor msg size
- [scripts] \#3147 Fix json2wal for large block parts (@bradyjoestar)
