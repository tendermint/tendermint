# Pending

Special thanks to external contributors on this release:

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

* P2P Protocol

FEATURES:
- [crypto/merkle] \#2298 General Merkle Proof scheme for chaining various types of Merkle trees together

IMPROVEMENTS:
- [consensus] [\#2169](https://github.com/cosmos/cosmos-sdk/issues/2169) add additional metrics
- [p2p] [\#2169](https://github.com/cosmos/cosmos-sdk/issues/2169) add additional metrics
- [config] \#2232 added ValidateBasic method, which performs basic checks

BUG FIXES:
- [autofile] \#2428 Group.RotateFile need call Flush() before rename (@goolAdapter)
- [node] \#2434 Make node respond to signal interrupts while sleeping for genesis time
- [consensus] [\#1690](https://github.com/tendermint/tendermint/issues/1690) wait for 
timeoutPrecommit before starting next round
- [evidence] \#2515 fix db iter leak (@goolAdapter)
