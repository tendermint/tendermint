# Pending

## v0.26.1

*TBA*

Special thanks to external contributors on this release:

Friendly reminder, we have a [bug bounty program](https://hackerone.com/tendermint).

### BREAKING CHANGES:

* CLI/RPC/Config

* Apps

* Go API

* Blockchain Protocol

* P2P Protocol

### FEATURES:

- [eventbus] create new event types for NewRound and CompleteProposal. add proposed block info to CompleteProposal event and proposer info to NewRound event

### IMPROVEMENTS:

### BUG FIXES:

- [crypto/merkle] [\#2756](https://github.com/tendermint/tendermint/issues/2756) Fix crypto/merkle ProofOperators.Verify to check bounds on keypath parts.
- [mempool] fix a bug where we create a WAL despite `wal_dir` being empty
- [p2p] \#2771 Fix `peer-id` label name in prometheus metrics
- [autofile] [\#2703] do not panic when checking Head size
