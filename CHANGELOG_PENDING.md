# Pending

## v0.27.0

*November 29th, 2018*

Special thanks to external contributors on this release:
@danil-lashin, @srmo

Special thanks to @dlguddus for discovering a [major
issue](https://github.com/tendermint/tendermint/issues/2718#issuecomment-440888677)
in the proposer selection algorithm.

Friendly reminder, we have a [bug bounty
program](https://hackerone.com/tendermint).

This release is primarily about fixes to the proposer selection algorithm
in preparation for the [Cosmos Game of
Stakes](https://blog.cosmos.network/the-game-of-stakes-is-open-for-registration-83a404746ee6).
It also makes use of the `ConsensusParams.Validator.PubKeyTypes` to restrict the
key types that can be used by validators.

### BREAKING CHANGES:

* CLI/RPC/Config
  - [rpc] \#2932 Rename `accum` to `proposer_priority`

* Go API
  - [db] [\#2913](https://github.com/tendermint/tendermint/pull/2913)
    ReverseIterator API change: start < end, and end is exclusive.
  - [types] \#2932 Rename `Validator.Accum` to `Validator.ProposerPriority`

* Blockchain Protocol
  - [state] \#2714 Validators can now only use pubkeys allowed within
    ConsensusParams.Validator.PubKeyTypes

* P2P Protocol
  - [consensus] [\#2871](https://github.com/tendermint/tendermint/issues/2871)
    Remove *ProposalHeartbeat* message as it serves no real purpose (@srmo)
  - [state] Fixes for proposer selection:
    - \#2785 Accum for new validators is `-1.125*totalVotingPower` instead of 0
    - \#2941 val.Accum is preserved during ValidatorSet.Update to avoid being
      reset to 0

### IMPROVEMENTS:

- [state] \#2929 Minor refactor of updateState logic (@danil-lashin)

### BUG FIXES:

- [types] \#2938 Fix regression in v0.26.4 where we panic on empty
  genDoc.Validators
- [state] \#2785 Fix accum for new validators to be `-1.125*totalVotingPower`
  instead of 0, forcing them to wait before becoming the proposer. Also:
    - do not batch clip
    - keep accums averaged near 0
- [types] \#2941 Preserve val.Accum during ValidatorSet.Update to avoid it being
  reset to 0 every time a validator is updated
