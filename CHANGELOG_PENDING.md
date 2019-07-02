## v0.32.1

**

Special thanks to external contributors on this release:

Friendly reminder, we have a [bug bounty
program](https://hackerone.com/tendermint).

### BREAKING CHANGES:

* CLI/RPC/Config

* Apps

* Go API
  - [abci] \#2127 ABCI / mempool: Add a "Recheck Tx" indicator. Breaks the ABCI
    client interface (`abcicli.Client`) to allow for supplying the ABCI
    `types.RequestCheckTx` and `types.RequestDeliverTx` structs, and lets the
    mempool indicate to the ABCI app whether a CheckTx request is a recheck or
    not.
  - [libs] Remove unused `db/debugDB` and `common/colors.go` & `errors/errors.go` files (@marbar3778)

* Blockchain Protocol

* P2P Protocol

### FEATURES:

### IMPROVEMENTS:
  - [rpc] \#3700 Make possible to set absolute paths for TLS cert and key (@climber73)

### BUG FIXES:
