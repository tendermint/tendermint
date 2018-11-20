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
  * [state] \#2714 Validators can now only use pubkeys allowed within ConsensusParams.ValidatorParams

* P2P Protocol

### FEATURES:

### IMPROVEMENTS:

### BUG FIXES:

- [crypto/merkle] [\#2756](https://github.com/tendermint/tendermint/issues/2756) Fix crypto/merkle ProofOperators.Verify to check bounds on keypath parts.
- [mempool] fix a bug where we create a WAL despite `wal_dir` being empty
