# Pending

Special thanks to external contributors on this release:
@goolAdapter, @bradyjoestar

BREAKING CHANGES:

* CLI/RPC/Config
  * [config] \#2232 timeouts as time.Duration, not ints
  * [config] \#2505 Remove Mempool.RecheckEmpty (it was effectively useless anyways)
  * [config] `mempool.wal` is disabled by default
  * [rpc] \#2298 `/abci_query` takes `prove` argument instead of `trusted` and switches the default
    behaviour to `prove=false`
  * [privval] \#2459 Split `SocketPVMsg`s implementations into Request and Response, where the Response may contain a error message (returned by the remote signer)
  * [state] \#2644 Add Version field to State, breaking the format of State as
    encoded on disk.
  * [rpc] \#2654 Remove all `node_info.other.*_version` fields in `/status` and
    `/net_info`

* Apps
  * [abci] \#2298 ResponseQuery.Proof is now a structured merkle.Proof, not just
    arbitrary bytes
  * [abci] \#2644 Add Version to Header and shift all fields by one
  * [abci] \#2662 Bump the field numbers for some `ResponseInfo` fields to make room for
      `AppVersion`

* Go API
  * [node] Remove node.RunForever
  * [config] \#2232 timeouts as time.Duration, not ints
  * [rpc/client] \#2298 `ABCIQueryOptions.Trusted` -> `ABCIQueryOptions.Prove`
  * [types] \#2298 Remove `Index` and `Total` fields from `TxProof`.
  * [crypto/merkle & lite] \#2298 Various changes to accomodate General Merkle trees
  * [crypto/merkle] \#2595 Remove all Hasher objects in favor of byte slices
  * [crypto/merkle] \#2635 merkle.SimpleHashFromTwoHashes is no longer exported
  * [types] \#2598 `VoteTypeXxx` are now of type `SignedMsgType byte` and named `XxxType`, eg. `PrevoteType`,
    `PrecommitType`.

* Blockchain Protocol
  * [types] Update SignBytes for `Vote`/`Proposal`/`Heartbeat`:
    * \#2459 Use amino encoding instead of JSON in `SignBytes`.
    * \#2598 Reorder fields and use fixed sized encoding.
    * \#2598 Change `Type` field fromt `string` to `byte` and use new
      `SignedMsgType` to enumerate.
  * [types] \#2512 Remove the pubkey field from the validator hash
  * [types] \#2644 Add Version struct to Header
  * [state] \#2587 Require block.Time of the fist block to be genesis time
  * [state] \#2644 Require block.Version to match state.Version

* P2P Protocol
  * [p2p] \#2654 Add `ProtocolVersion` struct with protocol versions to top of
    DefaultNodeInfo and require `ProtocolVersion.Block` to match during peer handshake


FEATURES:
- [crypto/merkle] \#2298 General Merkle Proof scheme for chaining various types of Merkle trees together
- [abci] \#2557 Add `Codespace` field to `Response{CheckTx, DeliverTx, Query}`
- [abci] \#2662 Add `BlockVersion` and `P2PVersion` to `RequestInfo`

IMPROVEMENTS:
- Additional Metrics
    - [consensus] [\#2169](https://github.com/cosmos/cosmos-sdk/issues/2169)
    - [p2p] [\#2169](https://github.com/cosmos/cosmos-sdk/issues/2169)
- [config] \#2232 Added ValidateBasic method, which performs basic checks
- [crypto/ed25519] \#2558 Switch to use latest `golang.org/x/crypto` through our fork at
  github.com/tendermint/crypto
- [tools] \#2238 Binary dependencies are now locked to a specific git commit
- [crypto] \#2099 make crypto random use chacha, and have forward secrecy of generated randomness

BUG FIXES:
- [autofile] \#2428 Group.RotateFile need call Flush() before rename (@goolAdapter)
- [node] \#2434 Make node respond to signal interrupts while sleeping for genesis time
- [consensus] [\#1690](https://github.com/tendermint/tendermint/issues/1690) wait for
timeoutPrecommit before starting next round
- [consensus] [\#1745](https://github.com/tendermint/tendermint/issues/1745) wait for
Proposal or timeoutProposal before entering prevote
- [evidence] \#2515 fix db iter leak (@goolAdapter)
- [common/bit_array] Fixed a bug in the `Or` function
- [common/bit_array] Fixed a bug in the `Sub` function (@james-ray)
- [common] \#2534 Make bit array's PickRandom choose uniformly from true bits
- [consensus] \#1637 Limit the amount of evidence that can be included in a
  block
- [p2p] \#2555 fix p2p switch FlushThrottle value (@goolAdapter)
- [libs/event] \#2518 fix event concurrency flaw (@goolAdapter)
- [state] \#2616 Pass nil to NewValidatorSet() when genesis file's Validators field is nil
