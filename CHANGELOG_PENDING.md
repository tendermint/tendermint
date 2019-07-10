## v0.32.1

\*\*

Special thanks to external contributors on this release:

Friendly reminder, we have a [bug bounty
program](https://hackerone.com/tendermint).

### BREAKING CHANGES:

- CLI/RPC/Config

- Apps

- Go API

  - [abci] \#2127 ABCI / mempool: Add a "Recheck Tx" indicator. Breaks the ABCI
    client interface (`abcicli.Client`) to allow for supplying the ABCI
    `types.RequestCheckTx` and `types.RequestDeliverTx` structs, and lets the
    mempool indicate to the ABCI app whether a CheckTx request is a recheck or
    not.
  - [libs] Remove unused `db/debugDB` and `common/colors.go` & `errors/errors.go` files (@marbar3778)
  - [libs] \#2432 Remove unused `common/heap.go` file (@marbar3778)

- Blockchain Protocol

- P2P Protocol

### FEATURES:

- [node] Refactor `NewNode` to use functional options to make it more flexible
  and extensible in the future.
- [node][\#3730](https://github.com/tendermint/tendermint/pull/3730) Add `CustomReactors` option to `NewNode` allowing caller to pass
  custom reactors to run inside Tendermint node (@ParthDesai)

### IMPROVEMENTS:

- [rpc] \#3700 Make possible to set absolute paths for TLS cert and key (@climber73)

### BUG FIXES:

- [p2p] \#3338 Prevent "sent next PEX request too soon" errors by not calling
  ensurePeers outside of ensurePeersRoutine
- [behaviour] Return correct reason in MessageOutOfOrder (@jim380)
- [config] \#3723 Add consensus_params to testnet config generation; document time_iota_ms (@ashleyvega)
