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

* Apps
  * [abci] \#2298 ResponseQuery.Proof is now a structured merkle.Proof, not just
    arbitrary bytes

* Go API
  * [node] Remove node.RunForever
  * [config] \#2232 timeouts as time.Duration, not ints
  * [rpc/client] \#2298 `ABCIQueryOptions.Trusted` -> `ABCIQueryOptions.Prove`
  * [types] \#2298 Remove `Index` and `Total` fields from `TxProof`.
  * [crypto/merkle & lite] \#2298 Various changes to accomodate General Merkle trees

* Blockchain Protocol
  * [types] \#2459 `Vote`/`Proposal`/`Heartbeat` use amino encoding instead of JSON in `SignBytes`.
  * [types] \#2512 Remove the pubkey field from the validator hash

* P2P Protocol

FEATURES:
- [crypto/merkle] \#2298 General Merkle Proof scheme for chaining various types of Merkle trees together
- [abci] \#2557 Add `Codespace` field to `Response{CheckTx, DeliverTx, Query}`

IMPROVEMENTS:
- [consensus] [\#2169](https://github.com/cosmos/cosmos-sdk/issues/2169) add additional metrics
- [p2p] [\#2169](https://github.com/cosmos/cosmos-sdk/issues/2169) add additional metrics
- [config] \#2232 added ValidateBasic method, which performs basic checks
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
- [common/bit_array] Fixed a bug in the `Sub` function (@bradyjoestar)
- [common] \#2534 make bit array's PickRandom choose uniformly from true bits
- [p2p] \#2555 fix p2p switch FlushThrottle value (@goolAdapter)
- [libs/event] \#2518 fix event concurrency flaw (@goolAdapter)
