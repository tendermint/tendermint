# Pending

## v0.27.0

*TBD*

Special thanks to external contributors on this release:

Friendly reminder, we have a [bug bounty
program](https://hackerone.com/tendermint).

### BREAKING CHANGES:

* CLI/RPC/Config
  - [rpc] \#2932 Rename `accum` to `proposer_priority`

* Apps

* Go API
  - [db] [\#2913](https://github.com/tendermint/tendermint/pull/2913)
    ReverseIterator API change -- start < end, and end is exclusive.
  - [types] \#2932 Rename `Validator.Accum` to `Validator.ProposerPriority`

* Blockchain Protocol
  - [state] \#2714 Validators can now only use pubkeys allowed within
    ConsensusParams.ValidatorParams

* P2P Protocol
  - [consensus] [\#2871](https://github.com/tendermint/tendermint/issues/2871)
    Remove *ProposalHeartbeat* message as it serves no real purpose
  - [state] Fixes for proposer selection:
    - \#2785 Accum for new validators is `-1.125*totalVotingPower` instead of 0
    - \#2941 val.Accum is preserved during ValidatorSet.Update to avoid being
      reset to 0

### FEATURES:

### IMPROVEMENTS:

### BUG FIXES:
- [types] \#2938 Fix regression in v0.26.4 where we panic on empty
  genDoc.Validators
- [state] \#2785 Fix accum for new validators to be `-1.125*totalVotingPower`
  instead of 0, forcing them to wait before becoming the proposer. Also:
    - do not batch clip
    - keep accums averaged near 0
- [types] \#2941 Preserve val.Accum during ValidatorSet.Update to avoid it being
  reset to 0 every time a validator is updated
