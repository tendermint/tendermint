# Pending

## v0.26.0

*October 29, 2018*

Special thanks to external contributors on this release:
@bradyjoestar, @connorwstein, @goolAdapter, @HaoyangLiu,
@james-ray, @overbool, @phymbert, @Slamper, @Uzair1995

This release is primarily about adding Version fields to various data structures,
optimizing consensus messages for signing and verification in
restricted environments (like HSMs and the Ethereum Virtual Machine), and
aligning the consensus code with the [specification](https://arxiv.org/abs/1807.04938).
It also includes our first take at a generalized merkle proof system.

See the [UPGRADING.md](UPGRADING.md#v0.26.0) for details on upgrading to the new
version.

Please note that we are still making breaking changes to the protocols.
While the new Version fields should help us to keep the software backwards compatible
even while upgrading the protocols, we cannot guarantee that new releases will
be compatible with old chains just yet. Thanks for bearing with us!

Friendly reminder, we have a [bug bounty program](https://hackerone.com/tendermint).

### BREAKING CHANGES:

* CLI/RPC/Config
  * [config] [\#2232](https://github.com/tendermint/tendermint/issues/2232) timeouts as time.Duration, not ints
  * [config] [\#2505](https://github.com/tendermint/tendermint/issues/2505) Remove Mempool.RecheckEmpty (it was effectively useless anyways)
  * [config] [\#2490](https://github.com/tendermint/tendermint/issues/2490) `mempool.wal` is disabled by default
  * [privval] [\#2459](https://github.com/tendermint/tendermint/issues/2459) Split `SocketPVMsg`s implementations into Request and Response, where the Response may contain a error message (returned by the remote signer)
  * [state] [\#2644](https://github.com/tendermint/tendermint/issues/2644) Add Version field to State, breaking the format of State as
    encoded on disk.
  * [rpc] [\#2298](https://github.com/tendermint/tendermint/issues/2298) `/abci_query` takes `prove` argument instead of `trusted` and switches the default
    behaviour to `prove=false`
  * [rpc] [\#2654](https://github.com/tendermint/tendermint/issues/2654) Remove all `node_info.other.*_version` fields in `/status` and
    `/net_info`

* Apps
  * [abci] [\#2298](https://github.com/tendermint/tendermint/issues/2298) ResponseQuery.Proof is now a structured merkle.Proof, not just
    arbitrary bytes
  * [abci] [\#2644](https://github.com/tendermint/tendermint/issues/2644) Add Version to Header and shift all fields by one
  * [abci] [\#2662](https://github.com/tendermint/tendermint/issues/2662) Bump the field numbers for some `ResponseInfo` fields to make room for
      `AppVersion`

* Go API
  * [config] [\#2232](https://github.com/tendermint/tendermint/issues/2232) timeouts as time.Duration, not ints
  * [crypto/merkle & lite] [\#2298](https://github.com/tendermint/tendermint/issues/2298) Various changes to accomodate General Merkle trees
  * [crypto/merkle] [\#2595](https://github.com/tendermint/tendermint/issues/2595) Remove all Hasher objects in favor of byte slices
  * [crypto/merkle] [\#2635](https://github.com/tendermint/tendermint/issues/2635) merkle.SimpleHashFromTwoHashes is no longer exported
  * [node] [\#2479](https://github.com/tendermint/tendermint/issues/2479) Remove node.RunForever
  * [rpc/client] [\#2298](https://github.com/tendermint/tendermint/issues/2298) `ABCIQueryOptions.Trusted` -> `ABCIQueryOptions.Prove`
  * [types] [\#2298](https://github.com/tendermint/tendermint/issues/2298) Remove `Index` and `Total` fields from `TxProof`.
  * [types] [\#2598](https://github.com/tendermint/tendermint/issues/2598) `VoteTypeXxx` are now of type `SignedMsgType byte` and named `XxxType`, eg. `PrevoteType`,
    `PrecommitType`.

* Blockchain Protocol
  * [abci] [\#2636](https://github.com/tendermint/tendermint/issues/2636) Add ValidatorParams field to ConsensusParams.
  (Used to control which pubkey types validators can use, by abci type)
  * [types] Update SignBytes for `Vote`/`Proposal`/`Heartbeat`:
    * [\#2459](https://github.com/tendermint/tendermint/issues/2459) Use amino encoding instead of JSON in `SignBytes`.
    * [\#2598](https://github.com/tendermint/tendermint/issues/2598) Reorder fields and use fixed sized encoding.
    * [\#2598](https://github.com/tendermint/tendermint/issues/2598) Change `Type` field fromt `string` to `byte` and use new
      `SignedMsgType` to enumerate.
  * [types] [\#2512](https://github.com/tendermint/tendermint/issues/2512) Remove the pubkey field from the validator hash
  * [types] [\#2644](https://github.com/tendermint/tendermint/issues/2644) Add Version struct to Header
  * [types] [\#2609](https://github.com/tendermint/tendermint/issues/2609) ConsensusParams.Hash() is the hash of the amino encoded
    struct instead of the Merkle tree of the fields
  * [state] [\#2587](https://github.com/tendermint/tendermint/issues/2587) Require block.Time of the fist block to be genesis time
  * [state] [\#2644](https://github.com/tendermint/tendermint/issues/2644) Require block.Version to match state.Version
  * [types] [\#2670](https://github.com/tendermint/tendermint/issues/2670) Header.Hash() builds Merkle tree out of fields in the same
    order they appear in the header, instead of sorting by field name
  * [types] [\#2682](https://github.com/tendermint/tendermint/issues/2682) Use proto3 `varint` encoding for ints that are usually unsigned (instead of zigzag encoding).

* P2P Protocol
  * [p2p] [\#2654](https://github.com/tendermint/tendermint/issues/2654) Add `ProtocolVersion` struct with protocol versions to top of
    DefaultNodeInfo and require `ProtocolVersion.Block` to match during peer handshake

### FEATURES:
- [abci] [\#2557](https://github.com/tendermint/tendermint/issues/2557) Add `Codespace` field to `Response{CheckTx, DeliverTx, Query}`
- [abci] [\#2662](https://github.com/tendermint/tendermint/issues/2662) Add `BlockVersion` and `P2PVersion` to `RequestInfo`
- [crypto/merkle] [\#2298](https://github.com/tendermint/tendermint/issues/2298) General Merkle Proof scheme for chaining various types of Merkle trees together

### IMPROVEMENTS:
- Additional Metrics
    - [consensus] [\#2169](https://github.com/cosmos/cosmos-sdk/issues/2169)
    - [p2p] [\#2169](https://github.com/cosmos/cosmos-sdk/issues/2169)
- [config] [\#2232](https://github.com/tendermint/tendermint/issues/2232) Added ValidateBasic method, which performs basic checks
- [crypto/ed25519] [\#2558](https://github.com/tendermint/tendermint/issues/2558) Switch to use latest `golang.org/x/crypto` through our fork at
  github.com/tendermint/crypto
- [tools] [\#2238](https://github.com/tendermint/tendermint/issues/2238) Binary dependencies are now locked to a specific git commit
- [libs/log] [\#2706](https://github.com/tendermint/tendermint/issues/2706) Add year to log format

### BUG FIXES:
- [autofile] [\#2428](https://github.com/tendermint/tendermint/issues/2428) Group.RotateFile need call Flush() before rename (@goolAdapter)
- [common] [\#2533](https://github.com/tendermint/tendermint/issues/2533) Fixed a bug in the `BitArray.Or` method
- [common] [\#2506](https://github.com/tendermint/tendermint/issues/2506) Fixed a bug in the `BitArray.Sub` method (@james-ray)
- [common] [\#2534](https://github.com/tendermint/tendermint/issues/2534) Fix `BitArray.PickRandom` to choose uniformly from true bits
- [consensus] [\#1690](https://github.com/tendermint/tendermint/issues/1690) Wait for
  timeoutPrecommit before starting next round
- [consensus] [\#1745](https://github.com/tendermint/tendermint/issues/1745) Wait for
  Proposal or timeoutProposal before entering prevote
- [consensus] [\#2583](https://github.com/tendermint/tendermint/issues/2583) ensure valid 
  block property with faulty proposer
- [consensus] [\#2642](https://github.com/tendermint/tendermint/issues/2642) Only propose ValidBlock, not LockedBlock
- [consensus] [\#2642](https://github.com/tendermint/tendermint/issues/2642) Initialized ValidRound and LockedRound to -1
- [consensus] [\#1637](https://github.com/tendermint/tendermint/issues/1637) Limit the amount of evidence that can be included in a
  block
- [consensus] [\#2646](https://github.com/tendermint/tendermint/issues/2646) Simplify Proposal message (align with spec)
- [crypto] [\#2733](https://github.com/tendermint/tendermint/pull/2733) Fix general merkle keypath to start w/ last op's key
- [evidence] [\#2515](https://github.com/tendermint/tendermint/issues/2515) Fix db iter leak (@goolAdapter)
- [libs/event] [\#2518](https://github.com/tendermint/tendermint/issues/2518) Fix event concurrency flaw (@goolAdapter)
- [node] [\#2434](https://github.com/tendermint/tendermint/issues/2434) Make node respond to signal interrupts while sleeping for genesis time
- [state] [\#2616](https://github.com/tendermint/tendermint/issues/2616) Pass nil to NewValidatorSet() when genesis file's Validators field is nil
- [p2p] [\#2555](https://github.com/tendermint/tendermint/issues/2555) Fix p2p switch FlushThrottle value (@goolAdapter)
- [p2p] [\#2668](https://github.com/tendermint/tendermint/issues/2668) Reconnect to originally dialed address (not self-reported
  address) for persistent peers

