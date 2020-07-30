## v0.33.8


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

- [consensus] \#5143 Only call `privValidator.GetPubKey` once per block (@melekes)

### BUG FIXES:

- [consensus] [\#4895](https://github.com/tendermint/tendermint/pull/4895) Cache the address of the validator to reduce querying a remote KMS (@joe-bowman)
