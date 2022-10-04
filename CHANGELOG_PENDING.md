# Unreleased Changes

## v0.34.22

### BREAKING CHANGES

- CLI/RPC/Config

- Apps

- P2P Protocol

- Go API

- Blockchain Protocol

### FEATURES

- [rpc] support https inside websocket (@RiccardoM, @cmwaters)

### IMPROVEMENTS
<<<<<<< HEAD
=======
- [crypto] \#9250 Update to use btcec v2 and the latest btcutil. (@wcsiu)

- [cli] \#9171 add `--hard` flag to rollback command (and a boolean to the `RollbackState` method). This will rollback
  state and remove the last block. This command can be triggered multiple times. The application must also rollback
  state to the same height. (@tsutsu, @cmwaters)
- [proto] \#9356 Migrate from `gogo/protobuf` to `cosmos/gogoproto` (@julienrbrt)
- [rpc] \#9276 Added `header` and `header_by_hash` queries to the RPC client (@samricotta)
- [abci] \#5706 Added `AbciVersion` to `RequestInfo` allowing applications to check ABCI version when connecting to Tendermint. (@marbar3778)
- [node] \#6059 Validate and complete genesis doc before saving to state store (@silasdavis)

- [crypto/ed25519] \#5632 Adopt zip215 `ed25519` verification. (@marbar3778)
- [crypto/ed25519] \#6526 Use [curve25519-voi](https://github.com/oasisprotocol/curve25519-voi) for `ed25519` signing and verification. (@Yawning)
- [crypto/sr25519] \#6526 Use [curve25519-voi](https://github.com/oasisprotocol/curve25519-voi) for `sr25519` signing and verification. (@Yawning)
- [crypto] \#6120 Implement batch verification interface for ed25519 and sr25519. (@marbar3778 & @Yawning)
- [types] \#6120 use batch verification for verifying commits signatures. (@marbar3778 & @cmwaters & @Yawning)
    - If the key type supports the batch verification API it will try to batch verify. If the verification fails we will single verify each signature.
- [state] \#9505 Added logic so when pruning, the evidence period is taken into consideration and only deletes unecessary data (@samricotta)
>>>>>>> abbeb919d (Use evidence period when pruning (#9505))

### BUG FIXES

- [config] \#9483 Calling `tendermint init` would incorrectly leave out the new
  `[storage]` section delimiter in the generated configuration file - this has
  now been fixed

