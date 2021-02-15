## v0.33.10


\*\*

Special thanks to external contributors on this release:

Friendly reminder, we have a [bug bounty program](https://hackerone.com/tendermint).

### BREAKING CHANGES:

- CLI/RPC/Config

- Apps

- P2P Protocol

- Go API

- Blockchain Protocol

### FEATURES:

### IMPROVEMENTS:

### BUG FIXES:

- [blockchain/v1] [\#5701](https://github.com/tendermint/tendermint/pull/5701) Handle peers without blocks (@melekes)
- [blockchain/v1] \#5711 Fix deadlock (@melekes)
- [proxy] \#5078 Fix a bug, where TM does not exit when ABCI app crashes (@melekes)
- [evidence] \#6068 Terminate broadcastEvidenceRoutine when peer is stopped (@melekes)
