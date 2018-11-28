# Pending

## v0.27.0

*TBD*

Special thanks to external contributors on this release:

Friendly reminder, we have a [bug bounty
program](https://hackerone.com/tendermint).

### BREAKING CHANGES:

* CLI/RPC/Config

* Apps

* Go API

- [db] [\#2913](https://github.com/tendermint/tendermint/pull/2913) ReverseIterator API change -- start < end, and end is exclusive.

* Blockchain Protocol
  * [state] \#2714 Validators can now only use pubkeys allowed within ConsensusParams.ValidatorParams

* P2P Protocol

### FEATURES:

### IMPROVEMENTS:
- [consensus] [\#2871](https://github.com/tendermint/tendermint/issues/2871) Remove *ProposalHeartbeat* infrastructure as it serves no real purpose

### BUG FIXES:
- [types] \#2938 Fix regression in v0.26.4 where we panic on empty
  genDoc.Validators
