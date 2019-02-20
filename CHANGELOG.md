# Changelog

## v0.30.1

*February 20th, 2019*

This release fixes a consensus halt and a DataCorruptionError after restart
discovered in `game_of_stakes_6`. It also fixes a security issue in the p2p
handshake by authenticating the NetAddress.ID of the peer we're dialing.

### IMPROVEMENTS:

* [config] [\#3291](https://github.com/tendermint/tendermint/issues/3291) Make
  config.ResetTestRootWithChainID() create concurrency-safe test directories.

### BUG FIXES:

* [consensus] [\#3295](https://github.com/tendermint/tendermint/issues/3295)
  Flush WAL on stop to prevent data corruption during graceful shutdown.
* [consensus] [\#3302](https://github.com/tendermint/tendermint/issues/3302)
  Fix possible halt by resetting TriggeredTimeoutPrecommit before starting next height.
* [rpc] [\#3251](https://github.com/tendermint/tendermint/issues/3251) Fix
  `/net_info#peers#remote_ip` format. New format spec:
  * dotted decimal ("192.0.2.1"), if ip is an IPv4 or IP4-mapped IPv6 address
  * IPv6 ("2001:db8::1"), if ip is a valid IPv6 address
* [cmd] [\#3314](https://github.com/tendermint/tendermint/issues/3314) Return
  an error on `show_validator` when the private validator file does not exist.
* [p2p] [\#3010](https://github.com/tendermint/tendermint/issues/3010#issuecomment-464287627)
  Authenticate a peer against its NetAddress.ID when dialing.

## v0.30.0

*February 8th, 2019*

This release fixes yet another issue with the proposer selection algorithm.
We hope it's the last one, but we won't be surprised if it's not.
We plan to one day expose the selection algorithm more directly to
the application ([\#3285](https://github.com/tendermint/tendermint/issues/3285)), and even to support randomness ([\#763](https://github.com/tendermint/tendermint/issues/763)).
For more, see issues marked
[proposer-selection](https://github.com/tendermint/tendermint/labels/proposer-selection).

This release also includes a fix to prevent Tendermint from including the same
piece of evidence in more than one block. This issue was reported by @chengwenxi in our
[bug bounty program](https://hackerone.com/tendermint).

### BREAKING CHANGES:

* Apps
  - [state] [\#3222](https://github.com/tendermint/tendermint/issues/3222)
    Duplicate updates for the same validator are forbidden. Apps must ensure
    that a given `ResponseEndBlock.ValidatorUpdates` contains only one entry per pubkey.

* Go API
  - [types] [\#3222](https://github.com/tendermint/tendermint/issues/3222)
    Remove `Add` and `Update` methods from `ValidatorSet` in favor of new
    `UpdateWithChangeSet`. This allows updates to be applied as a set, instead of
    one at a time.

* Block Protocol
  - [state] [\#3286](https://github.com/tendermint/tendermint/issues/3286) Blocks that include already committed evidence are invalid.

* P2P Protocol
  - [consensus] [\#3222](https://github.com/tendermint/tendermint/issues/3222)
    Validator updates are applied as a set, instead of one at a time, thus
    impacting the proposer priority calculation. This ensures that the proposer
    selection algorithm does not depend on the order of updates in
    `ResponseEndBlock.ValidatorUpdates`.

### IMPROVEMENTS:
- [crypto] [\#3279](https://github.com/tendermint/tendermint/issues/3279) Use `btcec.S256().N` directly instead of hard coding a copy.

### BUG FIXES:
- [state] [\#3222](https://github.com/tendermint/tendermint/issues/3222) Fix validator set updates so they are applied as a set, rather
  than one at a time. This makes the proposer selection algorithm independent of
  the order of updates in `ResponseEndBlock.ValidatorUpdates`.
- [evidence] [\#3286](https://github.com/tendermint/tendermint/issues/3286) Don't add committed evidence to evidence pool.

## v0.29.2

*February 7th, 2019*

Special thanks to external contributors on this release:
@ackratos, @rickyyangz

**Note**: This release contains security sensitive patches in the `p2p` and
`crypto` packages:
- p2p:
  - Partial fix for MITM attacks on the p2p connection. MITM conditions may
    still exist. See [\#3010](https://github.com/tendermint/tendermint/issues/3010).
- crypto:
  - Eliminate our fork of `btcd` and use the `btcd/btcec` library directly for
    native secp256k1 signing. Note we still modify the signature encoding to
    prevent malleability.
  - Support the libsecp256k1 library via CGo through the `go-ethereum/crypto/secp256k1` package.
  - Eliminate MixEntropy functions

### BREAKING CHANGES:

* Go API
  - [crypto] [\#3278](https://github.com/tendermint/tendermint/issues/3278) Remove
    MixEntropy functions
  - [types] [\#3245](https://github.com/tendermint/tendermint/issues/3245) Commit uses `type CommitSig Vote` instead of `Vote` directly.
    In preparation for removing redundant fields from the commit [\#1648](https://github.com/tendermint/tendermint/issues/1648)

### IMPROVEMENTS:
- [consensus] [\#3246](https://github.com/tendermint/tendermint/issues/3246) Better logging and notes on recovery for corrupted WAL file
- [crypto] [\#3163](https://github.com/tendermint/tendermint/issues/3163) Use ethereum's libsecp256k1 go-wrapper for signatures when cgo is available
- [crypto] [\#3162](https://github.com/tendermint/tendermint/issues/3162) Wrap btcd instead of forking it to keep up with fixes (used if cgo is not available)
- [makefile] [\#3233](https://github.com/tendermint/tendermint/issues/3233) Use golangci-lint instead of go-metalinter
- [tools] [\#3218](https://github.com/tendermint/tendermint/issues/3218) Add go-deadlock tool to help detect deadlocks
- [tools] [\#3106](https://github.com/tendermint/tendermint/issues/3106) Add tm-signer-harness test harness for remote signers
- [tests] [\#3258](https://github.com/tendermint/tendermint/issues/3258) Fixed a bunch of non-deterministic test failures

### BUG FIXES:
- [node] [\#3186](https://github.com/tendermint/tendermint/issues/3186) EventBus and indexerService should be started before first block (for replay last block on handshake) execution (@ackratos)
- [p2p] [\#3232](https://github.com/tendermint/tendermint/issues/3232) Fix infinite loop leading to addrbook deadlock for seed nodes
- [p2p] [\#3247](https://github.com/tendermint/tendermint/issues/3247) Fix panic in SeedMode when calling FlushStop and OnStop
  concurrently
- [p2p] [\#3040](https://github.com/tendermint/tendermint/issues/3040) Fix MITM on secret connection by checking low-order points
- [privval] [\#3258](https://github.com/tendermint/tendermint/issues/3258) Fix race between sign requests and ping requests in socket that was causing messages to be corrupted

## v0.29.1

*January 24, 2019*

Special thanks to external contributors on this release:
@infinytum, @gauthamzz

This release contains two important fixes: one for p2p layer where we sometimes
were not closing connections and one for consensus layer where consensus with
no empty blocks (`create_empty_blocks = false`) could halt.

Friendly reminder, we have a [bug bounty
program](https://hackerone.com/tendermint).

### IMPROVEMENTS:
- [pex] [\#3037](https://github.com/tendermint/tendermint/issues/3037) Only log "Reached max attempts to dial" once
- [rpc] [\#3159](https://github.com/tendermint/tendermint/issues/3159) Expose
  `triggered_timeout_commit` in the `/dump_consensus_state`

### BUG FIXES:
- [consensus] [\#3199](https://github.com/tendermint/tendermint/issues/3199) Fix consensus halt with no empty blocks from not resetting triggeredTimeoutCommit
- [p2p] [\#2967](https://github.com/tendermint/tendermint/issues/2967) Fix file descriptor leak

## v0.29.0

*January 21, 2019*

Special thanks to external contributors on this release:
@bradyjoestar, @kunaldhariwal, @gauthamzz, @hrharder

This release is primarily about making some breaking changes to
the Block protocol version before Cosmos launch, and to fixing more issues
in the proposer selection algorithm discovered on Cosmos testnets.

The Block protocol changes include using a standard Merkle tree format (RFC 6962),
fixing some inconsistencies between field orders in Vote and Proposal structs,
and constraining the hash of the ConsensusParams to include only a few fields.

The proposer selection algorithm saw significant progress,
including a [formal proof by @cwgoes for the base-case in Idris](https://github.com/cwgoes/tm-proposer-idris)
and a [much more detailed specification (still in progress) by
@ancazamfir](https://github.com/tendermint/tendermint/pull/3140).

Fixes to the proposer selection algorithm include normalizing the proposer
priorities to mitigate the effects of large changes to the validator set.
That said, we just discovered [another bug](https://github.com/tendermint/tendermint/issues/3181),
which will be fixed in the next breaking release.

While we are trying to stabilize the Block protocol to preserve compatibility
with old chains, there may be some final changes yet to come before Cosmos
launch as we continue to audit and test the software.

Friendly reminder, we have a [bug bounty
program](https://hackerone.com/tendermint).

### BREAKING CHANGES:

* CLI/RPC/Config

* Apps
  - [state] [\#3049](https://github.com/tendermint/tendermint/issues/3049) Total voting power of the validator set is upper bounded by
    `MaxInt64 / 8`. Apps must ensure they do not return changes to the validator
    set that cause this maximum to be exceeded.

* Go API
  - [node] [\#3082](https://github.com/tendermint/tendermint/issues/3082) MetricsProvider now requires you to pass a chain ID
  - [types] [\#2713](https://github.com/tendermint/tendermint/issues/2713) Rename `TxProof.LeafHash` to `TxProof.Leaf`
  - [crypto/merkle] [\#2713](https://github.com/tendermint/tendermint/issues/2713) `SimpleProof.Verify` takes a `leaf` instead of a
    `leafHash` and performs the hashing itself

* Blockchain Protocol
  * [crypto/merkle] [\#2713](https://github.com/tendermint/tendermint/issues/2713) Merkle trees now match the RFC 6962 specification
  * [types] [\#3078](https://github.com/tendermint/tendermint/issues/3078) Re-order Timestamp and BlockID in CanonicalVote so it's
    consistent with CanonicalProposal (BlockID comes
    first)
  * [types] [\#3165](https://github.com/tendermint/tendermint/issues/3165) Hash of ConsensusParams only includes BlockSize.MaxBytes and
    BlockSize.MaxGas

* P2P Protocol
  - [consensus] [\#3049](https://github.com/tendermint/tendermint/issues/3049) Normalize priorities to not exceed `2*TotalVotingPower` to mitigate unfair proposer selection
    heavily preferring earlier joined validators in the case of an early bonded large validator unbonding

### FEATURES:

### IMPROVEMENTS:
- [rpc] [\#3065](https://github.com/tendermint/tendermint/issues/3065) Return maxPerPage (100), not defaultPerPage (30) if `per_page` is greater than the max 100.
- [instrumentation] [\#3082](https://github.com/tendermint/tendermint/issues/3082) Add `chain_id` label for all metrics

### BUG FIXES:
- [crypto] [\#3164](https://github.com/tendermint/tendermint/issues/3164) Update `btcd` fork for rare signRFC6979 bug
- [lite] [\#3171](https://github.com/tendermint/tendermint/issues/3171) Fix verifying large validator set changes
- [log] [\#3125](https://github.com/tendermint/tendermint/issues/3125) Fix year format
- [mempool] [\#3168](https://github.com/tendermint/tendermint/issues/3168) Limit tx size to fit in the max reactor msg size
- [scripts] [\#3147](https://github.com/tendermint/tendermint/issues/3147) Fix json2wal for large block parts (@bradyjoestar)

## v0.28.1

*January 18th, 2019*

Special thanks to external contributors on this release:
@HaoyangLiu

Friendly reminder, we have a [bug bounty
program](https://hackerone.com/tendermint).

### BUG FIXES:
- [consensus] Fix consensus halt from proposing blocks with too much evidence

## v0.28.0

*January 16th, 2019*

Special thanks to external contributors on this release:
@fmauricios, @gianfelipe93, @husio, @needkane, @srmo, @yutianwu

This release is primarily about upgrades to the `privval` system -
separating the `priv_validator.json` into distinct config and data files, and
refactoring the socket validator to support reconnections.

**Note:** Please backup your existing `priv_validator.json` before using this
version.

See [UPGRADING.md](UPGRADING.md) for more details.

### BREAKING CHANGES:

* CLI/RPC/Config
  - [cli] Removed `--proxy_app=dummy` option. Use `kvstore` (`persistent_kvstore`) instead.
  - [cli] Renamed `--proxy_app=nilapp` to `--proxy_app=noop`.
  - [config] [\#2992](https://github.com/tendermint/tendermint/issues/2992) `allow_duplicate_ip` is now set to false
  - [privval] [\#1181](https://github.com/tendermint/tendermint/issues/1181) Split `priv_validator.json` into immutable (`config/priv_validator_key.json`) and mutable (`data/priv_validator_state.json`) parts (@yutianwu)
  - [privval] [\#2926](https://github.com/tendermint/tendermint/issues/2926) Split up `PubKeyMsg` into `PubKeyRequest` and `PubKeyResponse` to be consistent with other message types
  - [privval] [\#2923](https://github.com/tendermint/tendermint/issues/2923) Listen for unix socket connections instead of dialing them

* Apps

* Go API
  - [types] [\#2981](https://github.com/tendermint/tendermint/issues/2981) Remove `PrivValidator.GetAddress()`

* Blockchain Protocol

* P2P Protocol

### FEATURES:
- [rpc] [\#3052](https://github.com/tendermint/tendermint/issues/3052) Include peer's remote IP in `/net_info`

### IMPROVEMENTS:
- [consensus] [\#3086](https://github.com/tendermint/tendermint/issues/3086) Log peerID on ignored votes (@srmo)
- [docs] [\#3061](https://github.com/tendermint/tendermint/issues/3061) Added specification for signing consensus msgs at
  ./docs/spec/consensus/signing.md
- [privval] [\#2948](https://github.com/tendermint/tendermint/issues/2948) Memoize pubkey so it's only requested once on startup
- [privval] [\#2923](https://github.com/tendermint/tendermint/issues/2923) Retry RemoteSigner connections on error

### BUG FIXES:

- [build] [\#3085](https://github.com/tendermint/tendermint/issues/3085) Fix `Version` field in build scripts (@husio)
- [crypto/multisig] [\#3102](https://github.com/tendermint/tendermint/issues/3102) Fix multisig keys address length
- [crypto/encoding] [\#3101](https://github.com/tendermint/tendermint/issues/3101) Fix `PubKeyMultisigThreshold` unmarshalling into `crypto.PubKey` interface
- [p2p/conn] [\#3111](https://github.com/tendermint/tendermint/issues/3111) Make SecretConnection thread safe
- [rpc] [\#3053](https://github.com/tendermint/tendermint/issues/3053) Fix internal error in `/tx_search` when results are empty
  (@gianfelipe93)
- [types] [\#2926](https://github.com/tendermint/tendermint/issues/2926) Do not panic if retrieving the privval's public key fails

## v0.27.4

*December 21st, 2018*

### BUG FIXES:

- [mempool] [\#3036](https://github.com/tendermint/tendermint/issues/3036) Fix
  LRU cache by popping the least recently used item when the cache is full,
  not the most recently used one!

## v0.27.3

*December 16th, 2018*

### BREAKING CHANGES:

* Go API
  - [dep] [\#3027](https://github.com/tendermint/tendermint/issues/3027) Revert to mainline Go crypto library, eliminating the modified
    `bcrypt.GenerateFromPassword`

## v0.27.2

*December 16th, 2018*

### IMPROVEMENTS:

- [node] [\#3025](https://github.com/tendermint/tendermint/issues/3025) Validate NodeInfo addresses on startup.

### BUG FIXES:

- [p2p] [\#3025](https://github.com/tendermint/tendermint/pull/3025) Revert to using defers in addrbook.  Fixes deadlocks in pex and consensus upon invalid ExternalAddr/ListenAddr configuration.

## v0.27.1

*December 15th, 2018*

Special thanks to external contributors on this release:
@danil-lashin, @hleb-albau, @james-ray, @leo-xinwang

### FEATURES:
- [rpc] [\#2964](https://github.com/tendermint/tendermint/issues/2964) Add `UnconfirmedTxs(limit)` and `NumUnconfirmedTxs()` methods to HTTP/Local clients (@danil-lashin)
- [docs] [\#3004](https://github.com/tendermint/tendermint/issues/3004) Enable full-text search on docs pages

### IMPROVEMENTS:
- [consensus] [\#2971](https://github.com/tendermint/tendermint/issues/2971) Return error if ValidatorSet is empty after InitChain
  (@leo-xinwang)
- [ci/cd] [\#3005](https://github.com/tendermint/tendermint/issues/3005) Updated CircleCI job to trigger website build when docs are updated
- [docs] Various updates

### BUG FIXES:
- [cmd] [\#2983](https://github.com/tendermint/tendermint/issues/2983) `testnet` command always sets `addr_book_strict = false`
- [config] [\#2980](https://github.com/tendermint/tendermint/issues/2980) Fix CORS options formatting
- [kv indexer] [\#2912](https://github.com/tendermint/tendermint/issues/2912) Don't ignore key when executing CONTAINS
- [mempool] [\#2961](https://github.com/tendermint/tendermint/issues/2961) Call `notifyTxsAvailable` if there're txs left after committing a block, but recheck=false
- [mempool] [\#2994](https://github.com/tendermint/tendermint/issues/2994) Reject txs with negative GasWanted
- [p2p] [\#2990](https://github.com/tendermint/tendermint/issues/2990) Fix a bug where seeds don't disconnect from a peer after 3h
- [consensus] [\#3006](https://github.com/tendermint/tendermint/issues/3006) Save state after InitChain only when stateHeight is also 0 (@james-ray)

## v0.27.0

*December 5th, 2018*

Special thanks to external contributors on this release:
@danil-lashin, @srmo

Special thanks to @dlguddus for discovering a [major
issue](https://github.com/tendermint/tendermint/issues/2718#issuecomment-440888677)
in the proposer selection algorithm.

Friendly reminder, we have a [bug bounty
program](https://hackerone.com/tendermint).

This release is primarily about fixes to the proposer selection algorithm
in preparation for the [Cosmos Game of
Stakes](https://blog.cosmos.network/the-game-of-stakes-is-open-for-registration-83a404746ee6).
It also makes use of the `ConsensusParams.Validator.PubKeyTypes` to restrict the
key types that can be used by validators, and removes the `Heartbeat` consensus
message.

### BREAKING CHANGES:

* CLI/RPC/Config
  - [rpc] [\#2932](https://github.com/tendermint/tendermint/issues/2932) Rename `accum` to `proposer_priority`

* Go API
  - [db] [\#2913](https://github.com/tendermint/tendermint/pull/2913)
    ReverseIterator API change: start < end, and end is exclusive.
  - [types] [\#2932](https://github.com/tendermint/tendermint/issues/2932) Rename `Validator.Accum` to `Validator.ProposerPriority`

* Blockchain Protocol
  - [state] [\#2714](https://github.com/tendermint/tendermint/issues/2714) Validators can now only use pubkeys allowed within
    ConsensusParams.Validator.PubKeyTypes

* P2P Protocol
  - [consensus] [\#2871](https://github.com/tendermint/tendermint/issues/2871)
    Remove *ProposalHeartbeat* message as it serves no real purpose (@srmo)
  - [state] Fixes for proposer selection:
    - [\#2785](https://github.com/tendermint/tendermint/issues/2785) Accum for new validators is `-1.125*totalVotingPower` instead of 0
    - [\#2941](https://github.com/tendermint/tendermint/issues/2941) val.Accum is preserved during ValidatorSet.Update to avoid being
      reset to 0

### IMPROVEMENTS:

- [state] [\#2929](https://github.com/tendermint/tendermint/issues/2929) Minor refactor of updateState logic (@danil-lashin)
- [node] [\#2959](https://github.com/tendermint/tendermint/issues/2959) Allow node to start even if software's BlockProtocol is
  different from state's BlockProtocol
- [pex] [\#2959](https://github.com/tendermint/tendermint/issues/2959) Pex reactor logger uses `module=pex`

### BUG FIXES:

- [p2p] [\#2968](https://github.com/tendermint/tendermint/issues/2968) Panic on transport error rather than continuing to run but not
  accept new connections
- [p2p] [\#2969](https://github.com/tendermint/tendermint/issues/2969) Fix mismatch in peer count between `/net_info` and the prometheus
  metrics
- [rpc] [\#2408](https://github.com/tendermint/tendermint/issues/2408) `/broadcast_tx_commit`: Fix "interface conversion: interface {} in nil, not EventDataTx" panic (could happen if somebody sent a tx using `/broadcast_tx_commit` while Tendermint was being stopped)
- [state] [\#2785](https://github.com/tendermint/tendermint/issues/2785) Fix accum for new validators to be `-1.125*totalVotingPower`
  instead of 0, forcing them to wait before becoming the proposer. Also:
    - do not batch clip
    - keep accums averaged near 0
- [txindex/kv] [\#2925](https://github.com/tendermint/tendermint/issues/2925) Don't return false positives when range searching for a prefix of a tag value
- [types] [\#2938](https://github.com/tendermint/tendermint/issues/2938) Fix regression in v0.26.4 where we panic on empty
  genDoc.Validators
- [types] [\#2941](https://github.com/tendermint/tendermint/issues/2941) Preserve val.Accum during ValidatorSet.Update to avoid it being
  reset to 0 every time a validator is updated

## v0.26.4

*November 27th, 2018*

Special thanks to external contributors on this release:
@ackratos, @goolAdapter, @james-ray, @joe-bowman, @kostko,
@nagarajmanjunath, @tomtau

Friendly reminder, we have a [bug bounty
program](https://hackerone.com/tendermint).

### FEATURES:

- [rpc] [\#2747](https://github.com/tendermint/tendermint/issues/2747) Enable subscription to tags emitted from `BeginBlock`/`EndBlock` (@kostko)
- [types] [\#2747](https://github.com/tendermint/tendermint/issues/2747) Add `ResultBeginBlock` and `ResultEndBlock` fields to `EventDataNewBlock`
    and `EventDataNewBlockHeader` to support subscriptions (@kostko)
- [types] [\#2918](https://github.com/tendermint/tendermint/issues/2918) Add Marshal, MarshalTo, Unmarshal methods to various structs
  to support Protobuf compatibility (@nagarajmanjunath)

### IMPROVEMENTS:

- [config] [\#2877](https://github.com/tendermint/tendermint/issues/2877) Add `blocktime_iota` to the config.toml (@ackratos)
    - NOTE: this should be a ConsensusParam, not part of the config, and will be
      removed from the config at a later date
      ([\#2920](https://github.com/tendermint/tendermint/issues/2920).
- [mempool] [\#2882](https://github.com/tendermint/tendermint/issues/2882) Add txs from Update to cache
- [mempool] [\#2891](https://github.com/tendermint/tendermint/issues/2891) Remove local int64 counter from being stored in every tx
- [node] [\#2866](https://github.com/tendermint/tendermint/issues/2866) Add ability to instantiate IPCVal (@joe-bowman)

### BUG FIXES:

- [blockchain] [\#2731](https://github.com/tendermint/tendermint/issues/2731) Retry both blocks if either is bad to avoid getting stuck during fast sync (@goolAdapter)
- [consensus] [\#2893](https://github.com/tendermint/tendermint/issues/2893) Use genDoc.Validators instead of state.NextValidators on replay when appHeight==0 (@james-ray)
- [log] [\#2868](https://github.com/tendermint/tendermint/issues/2868) Fix `module=main` setting overriding all others
    - NOTE: this changes the default logging behaviour to be much less verbose.
      Set `log_level="info"` to restore the previous behaviour.
- [rpc] [\#2808](https://github.com/tendermint/tendermint/issues/2808) Fix `accum` field in `/validators` by calling `IncrementAccum` if necessary
- [rpc] [\#2811](https://github.com/tendermint/tendermint/issues/2811) Allow integer IDs in JSON-RPC requests (@tomtau)
- [txindex/kv] [\#2759](https://github.com/tendermint/tendermint/issues/2759) Fix tx.height range queries
- [txindex/kv] [\#2775](https://github.com/tendermint/tendermint/issues/2775) Order tx results by index if height is the same
- [txindex/kv] [\#2908](https://github.com/tendermint/tendermint/issues/2908) Don't return false positives when searching for a prefix of a tag value

## v0.26.3

*November 17th, 2018*

Special thanks to external contributors on this release:
@danil-lashin, @kevlubkcm, @krhubert, @srmo

Friendly reminder, we have a [bug bounty
program](https://hackerone.com/tendermint).

### BREAKING CHANGES:

* Go API
  - [rpc] [\#2791](https://github.com/tendermint/tendermint/issues/2791) Functions that start HTTP servers are now blocking:
    - Impacts `StartHTTPServer`, `StartHTTPAndTLSServer`, and `StartGRPCServer`
    - These functions now take a `net.Listener` instead of an address
  - [rpc] [\#2767](https://github.com/tendermint/tendermint/issues/2767) Subscribing to events
  `NewRound` and `CompleteProposal` return new types `EventDataNewRound` and
  `EventDataCompleteProposal`, respectively, instead of the generic `EventDataRoundState`. (@kevlubkcm)

### FEATURES:

- [log] [\#2843](https://github.com/tendermint/tendermint/issues/2843) New `log_format` config option, which can be set to 'plain' for colored
  text or 'json' for JSON output
- [types] [\#2767](https://github.com/tendermint/tendermint/issues/2767) New event types EventDataNewRound (with ProposerInfo) and EventDataCompleteProposal (with BlockID). (@kevlubkcm)

### IMPROVEMENTS:

- [dep] [\#2844](https://github.com/tendermint/tendermint/issues/2844) Dependencies are no longer pinned to an exact version in the
  Gopkg.toml:
  - Serialization libs are allowed to vary by patch release
  - Other libs are allowed to vary by minor release
- [p2p] [\#2857](https://github.com/tendermint/tendermint/issues/2857) "Send failed" is logged at debug level instead of error.
- [rpc] [\#2780](https://github.com/tendermint/tendermint/issues/2780) Add read and write timeouts to HTTP servers
- [state] [\#2848](https://github.com/tendermint/tendermint/issues/2848) Make "Update to validators" msg value pretty (@danil-lashin)

### BUG FIXES:
- [consensus] [\#2819](https://github.com/tendermint/tendermint/issues/2819) Don't send proposalHearbeat if not a validator
- [docs] [\#2859](https://github.com/tendermint/tendermint/issues/2859) Fix ConsensusParams details in spec
- [libs/autofile] [\#2760](https://github.com/tendermint/tendermint/issues/2760) Comment out autofile permissions check - should fix
  running Tendermint on Windows
- [p2p] [\#2869](https://github.com/tendermint/tendermint/issues/2869) Set connection config properly instead of always using default
- [p2p/pex] [\#2802](https://github.com/tendermint/tendermint/issues/2802) Seed mode fixes:
  - Only disconnect from inbound peers
  - Use FlushStop instead of Sleep to ensure all messages are sent before
    disconnecting

## v0.26.2

*November 15th, 2018*

Special thanks to external contributors on this release: @hleb-albau, @zhuzeyu

Friendly reminder, we have a [bug bounty program](https://hackerone.com/tendermint).

### FEATURES:

- [rpc] [\#2582](https://github.com/tendermint/tendermint/issues/2582) Enable CORS on RPC API (@hleb-albau)

### BUG FIXES:

- [abci] [\#2748](https://github.com/tendermint/tendermint/issues/2748) Unlock mutex in localClient so even when app panics (e.g. during CheckTx), consensus continue working
- [abci] [\#2748](https://github.com/tendermint/tendermint/issues/2748) Fix DATA RACE in localClient
- [amino] [\#2822](https://github.com/tendermint/tendermint/issues/2822) Update to v0.14.1 to support compiling on 32-bit platforms
- [rpc] [\#2748](https://github.com/tendermint/tendermint/issues/2748) Drain channel before calling Unsubscribe(All) in `/broadcast_tx_commit`

## v0.26.1

*November 11, 2018*

Special thanks to external contributors on this release: @katakonst

Friendly reminder, we have a [bug bounty program](https://hackerone.com/tendermint).

### IMPROVEMENTS:

- [consensus] [\#2704](https://github.com/tendermint/tendermint/issues/2704) Simplify valid POL round logic
- [docs] [\#2749](https://github.com/tendermint/tendermint/issues/2749) Deduplicate some ABCI docs
- [mempool] More detailed log messages
    - [\#2724](https://github.com/tendermint/tendermint/issues/2724)
    - [\#2762](https://github.com/tendermint/tendermint/issues/2762)

### BUG FIXES:

- [autofile] [\#2703](https://github.com/tendermint/tendermint/issues/2703) Do not panic when checking Head size
- [crypto/merkle] [\#2756](https://github.com/tendermint/tendermint/issues/2756) Fix crypto/merkle ProofOperators.Verify to check bounds on keypath parts.
- [mempool] fix a bug where we create a WAL despite `wal_dir` being empty
- [p2p] [\#2771](https://github.com/tendermint/tendermint/issues/2771) Fix `peer-id` label name to `peer_id` in prometheus metrics
- [p2p] [\#2797](https://github.com/tendermint/tendermint/pull/2797) Fix IDs in peer NodeInfo and require them for addresses
  in AddressBook
- [p2p] [\#2797](https://github.com/tendermint/tendermint/pull/2797) Do not close conn immediately after sending pex addrs in seed mode. Partial fix for [\#2092](https://github.com/tendermint/tendermint/issues/2092).

## v0.26.0

*November 2, 2018*

Special thanks to external contributors on this release:
@bradyjoestar, @connorwstein, @goolAdapter, @HaoyangLiu,
@james-ray, @overbool, @phymbert, @Slamper, @Uzair1995, @yutianwu.

Special thanks to @Slamper for a series of bug reports in our [bug bounty
program](https://hackerone.com/tendermint) which are fixed in this release.

This release is primarily about adding Version fields to various data structures,
optimizing consensus messages for signing and verification in
restricted environments (like HSMs and the Ethereum Virtual Machine), and
aligning the consensus code with the [specification](https://arxiv.org/abs/1807.04938).
It also includes our first take at a generalized merkle proof system, and
changes the length of hashes used for hashing data structures from 20 to 32
bytes.

See the [UPGRADING.md](UPGRADING.md#v0.26.0) for details on upgrading to the new
version.

Please note that we are still making breaking changes to the protocols.
While the new Version fields should help us to keep the software backwards compatible
even while upgrading the protocols, we cannot guarantee that new releases will
be compatible with old chains just yet. We expect there will be another breaking
release or two before the Cosmos Hub launch, but we will otherwise be paying
increasing attention to backwards compatibility. Thanks for bearing with us!

### BREAKING CHANGES:

* CLI/RPC/Config
  * [config] [\#2232](https://github.com/tendermint/tendermint/issues/2232) Timeouts are now strings like "3s" and "100ms", not ints
  * [config] [\#2505](https://github.com/tendermint/tendermint/issues/2505) Remove Mempool.RecheckEmpty (it was effectively useless anyways)
  * [config] [\#2490](https://github.com/tendermint/tendermint/issues/2490) `mempool.wal` is disabled by default
  * [privval] [\#2459](https://github.com/tendermint/tendermint/issues/2459) Split `SocketPVMsg`s implementations into Request and Response, where the Response may contain a error message (returned by the remote signer)
  * [state] [\#2644](https://github.com/tendermint/tendermint/issues/2644) Add Version field to State, breaking the format of State as
    encoded on disk.
  * [rpc] [\#2298](https://github.com/tendermint/tendermint/issues/2298) `/abci_query` takes `prove` argument instead of `trusted` and switches the default
    behaviour to `prove=false`
  * [rpc] [\#2654](https://github.com/tendermint/tendermint/issues/2654) Remove all `node_info.other.*_version` fields in `/status` and
    `/net_info`
  * [rpc] [\#2636](https://github.com/tendermint/tendermint/issues/2636) Remove
    `_params` suffix from fields in `consensus_params`.

* Apps
  * [abci] [\#2298](https://github.com/tendermint/tendermint/issues/2298) ResponseQuery.Proof is now a structured merkle.Proof, not just
    arbitrary bytes
  * [abci] [\#2644](https://github.com/tendermint/tendermint/issues/2644) Add Version to Header and shift all fields by one
  * [abci] [\#2662](https://github.com/tendermint/tendermint/issues/2662) Bump the field numbers for some `ResponseInfo` fields to make room for
      `AppVersion`
  * [abci] [\#2636](https://github.com/tendermint/tendermint/issues/2636) Updates to ConsensusParams
    * Remove `Params` suffix from field names
    * Add `Params` suffix to message types
    * Add new field and type, `Validator ValidatorParams`, to control what types of validator keys are allowed.

* Go API
  * [config] [\#2232](https://github.com/tendermint/tendermint/issues/2232) Timeouts are time.Duration, not ints
  * [crypto/merkle & lite] [\#2298](https://github.com/tendermint/tendermint/issues/2298) Various changes to accomodate General Merkle trees
  * [crypto/merkle] [\#2595](https://github.com/tendermint/tendermint/issues/2595) Remove all Hasher objects in favor of byte slices
  * [crypto/merkle] [\#2635](https://github.com/tendermint/tendermint/issues/2635) merkle.SimpleHashFromTwoHashes is no longer exported
  * [node] [\#2479](https://github.com/tendermint/tendermint/issues/2479) Remove node.RunForever
  * [rpc/client] [\#2298](https://github.com/tendermint/tendermint/issues/2298) `ABCIQueryOptions.Trusted` -> `ABCIQueryOptions.Prove`
  * [types] [\#2298](https://github.com/tendermint/tendermint/issues/2298) Remove `Index` and `Total` fields from `TxProof`.
  * [types] [\#2598](https://github.com/tendermint/tendermint/issues/2598)
    `VoteTypeXxx` are now of type `SignedMsgType byte` and named `XxxType`, eg.
    `PrevoteType`, `PrecommitType`.
  * [types] [\#2636](https://github.com/tendermint/tendermint/issues/2636) Rename fields in ConsensusParams to remove `Params` suffixes
  * [types] [\#2735](https://github.com/tendermint/tendermint/issues/2735) Simplify Proposal message to align with spec

* Blockchain Protocol
  * [crypto/tmhash] [\#2732](https://github.com/tendermint/tendermint/issues/2732) TMHASH is now full 32-byte SHA256
    * All hashes in the block header and Merkle trees are now 32-bytes
    * PubKey Addresses are still only 20-bytes
  * [state] [\#2587](https://github.com/tendermint/tendermint/issues/2587) Require block.Time of the fist block to be genesis time
  * [state] [\#2644](https://github.com/tendermint/tendermint/issues/2644) Require block.Version to match state.Version
  * [types] Update SignBytes for `Vote`/`Proposal`/`Heartbeat`:
    * [\#2459](https://github.com/tendermint/tendermint/issues/2459) Use amino encoding instead of JSON in `SignBytes`.
    * [\#2598](https://github.com/tendermint/tendermint/issues/2598) Reorder fields and use fixed sized encoding.
    * [\#2598](https://github.com/tendermint/tendermint/issues/2598) Change `Type` field from `string` to `byte` and use new
      `SignedMsgType` to enumerate.
  * [types] [\#2730](https://github.com/tendermint/tendermint/issues/2730) Use
    same order for fields in `Vote` as in the SignBytes
  * [types] [\#2732](https://github.com/tendermint/tendermint/issues/2732) Remove the address field from the validator hash
  * [types] [\#2644](https://github.com/tendermint/tendermint/issues/2644) Add Version struct to Header
  * [types] [\#2609](https://github.com/tendermint/tendermint/issues/2609) ConsensusParams.Hash() is the hash of the amino encoded
    struct instead of the Merkle tree of the fields
  * [types] [\#2670](https://github.com/tendermint/tendermint/issues/2670) Header.Hash() builds Merkle tree out of fields in the same
    order they appear in the header, instead of sorting by field name
  * [types] [\#2682](https://github.com/tendermint/tendermint/issues/2682) Use proto3 `varint` encoding for ints that are usually unsigned (instead of zigzag encoding).
  * [types] [\#2636](https://github.com/tendermint/tendermint/issues/2636) Add Validator field to ConsensusParams
      (Used to control which pubkey types validators can use, by abci type).

* P2P Protocol
  * [consensus] [\#2652](https://github.com/tendermint/tendermint/issues/2652)
    Replace `CommitStepMessage` with `NewValidBlockMessage`
  * [consensus] [\#2735](https://github.com/tendermint/tendermint/issues/2735) Simplify `Proposal` message to align with spec
  * [consensus] [\#2730](https://github.com/tendermint/tendermint/issues/2730)
    Add `Type` field to `Proposal` and use same order of fields as in the
    SignBytes for both `Proposal` and `Vote`
  * [p2p] [\#2654](https://github.com/tendermint/tendermint/issues/2654) Add `ProtocolVersion` struct with protocol versions to top of
    DefaultNodeInfo and require `ProtocolVersion.Block` to match during peer handshake


### FEATURES:
- [abci] [\#2557](https://github.com/tendermint/tendermint/issues/2557) Add `Codespace` field to `Response{CheckTx, DeliverTx, Query}`
- [abci] [\#2662](https://github.com/tendermint/tendermint/issues/2662) Add `BlockVersion` and `P2PVersion` to `RequestInfo`
- [crypto/merkle] [\#2298](https://github.com/tendermint/tendermint/issues/2298) General Merkle Proof scheme for chaining various types of Merkle trees together
- [docs/architecture] [\#1181](https://github.com/tendermint/tendermint/issues/1181) S
plit immutable and mutable parts of priv_validator.json

### IMPROVEMENTS:
- Additional Metrics
    - [consensus] [\#2169](https://github.com/cosmos/cosmos-sdk/issues/2169)
    - [p2p] [\#2169](https://github.com/cosmos/cosmos-sdk/issues/2169)
- [config] [\#2232](https://github.com/tendermint/tendermint/issues/2232) Added ValidateBasic method, which performs basic checks
- [crypto/ed25519] [\#2558](https://github.com/tendermint/tendermint/issues/2558) Switch to use latest `golang.org/x/crypto` through our fork at
  github.com/tendermint/crypto
- [libs/log] [\#2707](https://github.com/tendermint/tendermint/issues/2707) Add year to log format (@yutianwu)
- [tools] [\#2238](https://github.com/tendermint/tendermint/issues/2238) Binary dependencies are now locked to a specific git commit

### BUG FIXES:
- [\#2711](https://github.com/tendermint/tendermint/issues/2711) Validate all incoming reactor messages. Fixes various bugs due to negative ints.
- [autofile] [\#2428](https://github.com/tendermint/tendermint/issues/2428) Group.RotateFile need call Flush() before rename (@goolAdapter)
- [common] [\#2533](https://github.com/tendermint/tendermint/issues/2533) Fixed a bug in the `BitArray.Or` method
- [common] [\#2506](https://github.com/tendermint/tendermint/issues/2506) Fixed a bug in the `BitArray.Sub` method (@james-ray)
- [common] [\#2534](https://github.com/tendermint/tendermint/issues/2534) Fix `BitArray.PickRandom` to choose uniformly from true bits
- [consensus] [\#1690](https://github.com/tendermint/tendermint/issues/1690) Wait for
  timeoutPrecommit before starting next round
- [consensus] [\#1745](https://github.com/tendermint/tendermint/issues/1745) Wait for
  Proposal or timeoutProposal before entering prevote
- [consensus] [\#2642](https://github.com/tendermint/tendermint/issues/2642) Only propose ValidBlock, not LockedBlock
- [consensus] [\#2642](https://github.com/tendermint/tendermint/issues/2642) Initialized ValidRound and LockedRound to -1
- [consensus] [\#1637](https://github.com/tendermint/tendermint/issues/1637) Limit the amount of evidence that can be included in a
  block
- [consensus] [\#2652](https://github.com/tendermint/tendermint/issues/2652) Ensure valid block property with faulty proposer
- [evidence] [\#2515](https://github.com/tendermint/tendermint/issues/2515) Fix db iter leak (@goolAdapter)
- [libs/event] [\#2518](https://github.com/tendermint/tendermint/issues/2518) Fix event concurrency flaw (@goolAdapter)
- [node] [\#2434](https://github.com/tendermint/tendermint/issues/2434) Make node respond to signal interrupts while sleeping for genesis time
- [state] [\#2616](https://github.com/tendermint/tendermint/issues/2616) Pass nil to NewValidatorSet() when genesis file's Validators field is nil
- [p2p] [\#2555](https://github.com/tendermint/tendermint/issues/2555) Fix p2p switch FlushThrottle value (@goolAdapter)
- [p2p] [\#2668](https://github.com/tendermint/tendermint/issues/2668) Reconnect to originally dialed address (not self-reported address) for persistent peers

## v0.25.0

*September 22, 2018*

Special thanks to external contributors on this release:
@scriptionist, @bradyjoestar, @WALL-E

This release is mostly about the ConsensusParams - removing fields and enforcing MaxGas.
It also addresses some issues found via security audit, removes various unused
functions from `libs/common`, and implements
[ADR-012](https://github.com/tendermint/tendermint/blob/develop/docs/architecture/adr-012-peer-transport.md).

Friendly reminder, we have a [bug bounty program](https://hackerone.com/tendermint).

BREAKING CHANGES:

* CLI/RPC/Config
  * [rpc] [\#2391](https://github.com/tendermint/tendermint/issues/2391) /status `result.node_info.other` became a map
  * [types] [\#2364](https://github.com/tendermint/tendermint/issues/2364) Remove `TxSize` and `BlockGossip` from `ConsensusParams`
    * Maximum tx size is now set implicitly via the `BlockSize.MaxBytes`
    * The size of block parts in the consensus is now fixed to 64kB

* Apps
  * [mempool] [\#2360](https://github.com/tendermint/tendermint/issues/2360) Mempool tracks the `ResponseCheckTx.GasWanted` and
    `ConsensusParams.BlockSize.MaxGas` and enforces:
    - `GasWanted <= MaxGas` for every tx
    - `(sum of GasWanted in block) <= MaxGas` for block proposal

* Go API
  * [libs/common] [\#2431](https://github.com/tendermint/tendermint/issues/2431) Remove Word256 due to lack of use
  * [libs/common] [\#2452](https://github.com/tendermint/tendermint/issues/2452) Remove the following functions due to lack of use:
    * byteslice.go: cmn.IsZeros, cmn.RightPadBytes, cmn.LeftPadBytes, cmn.PrefixEndBytes
    * strings.go: cmn.IsHex, cmn.StripHex
    * int.go: Uint64Slice, all put/get int64 methods

FEATURES:
- [rpc] [\#2415](https://github.com/tendermint/tendermint/issues/2415) New `/consensus_params?height=X` endpoint to query the consensus
  params at any height (@scriptonist)
- [types] [\#1714](https://github.com/tendermint/tendermint/issues/1714) Add Address to GenesisValidator
- [metrics] [\#2337](https://github.com/tendermint/tendermint/issues/2337) `consensus.block_interval_metrics` is now gauge, not histogram (you will be able to see spikes, if any)
- [libs] [\#2286](https://github.com/tendermint/tendermint/issues/2286) Panic if `autofile` or `db/fsdb` permissions change from 0600.

IMPROVEMENTS:
- [libs/db] [\#2371](https://github.com/tendermint/tendermint/issues/2371) Output error instead of panic when the given `db_backend` is not initialised (@bradyjoestar)
- [mempool] [\#2399](https://github.com/tendermint/tendermint/issues/2399) Make mempool cache a proper LRU (@bradyjoestar)
- [p2p] [\#2126](https://github.com/tendermint/tendermint/issues/2126) Introduce PeerTransport interface to improve isolation of concerns
- [libs/common] [\#2326](https://github.com/tendermint/tendermint/issues/2326) Service returns ErrNotStarted

BUG FIXES:
- [node] [\#2294](https://github.com/tendermint/tendermint/issues/2294) Delay starting node until Genesis time
- [consensus] [\#2048](https://github.com/tendermint/tendermint/issues/2048) Correct peer statistics for marking peer as good
- [rpc] [\#2460](https://github.com/tendermint/tendermint/issues/2460) StartHTTPAndTLSServer() now passes StartTLS() errors back to the caller rather than hanging forever.
- [p2p] [\#2047](https://github.com/tendermint/tendermint/issues/2047) Accept new connections asynchronously
- [tm-bench] [\#2410](https://github.com/tendermint/tendermint/issues/2410) Enforce minimum transaction size (@WALL-E)

## 0.24.0

*September 6th, 2018*

Special thanks to external contributors with PRs included in this release: ackratos, james-ray, bradyjoestar,
peerlink, Ahmah2009, bluele, b00f.

This release includes breaking upgrades in the block header,
including the long awaited changes for delaying validator set updates by one
block to better support light clients.
It also fixes enforcement on the maximum size of blocks, and includes a BFT
timestamp in each block that can be safely used by applications.
There are also some minor breaking changes to the rpc, config, and ABCI.

See the [UPGRADING.md](UPGRADING.md#v0.24.0) for details on upgrading to the new
version.

From here on, breaking changes will be broken down to better reflect how users
are affected by a change.

A few more breaking changes are in the works - each will come with a clear
Architecture Decision Record (ADR) explaining the change. You can review ADRs
[here](https://github.com/tendermint/tendermint/tree/develop/docs/architecture)
or in the [open Pull Requests](https://github.com/tendermint/tendermint/pulls).
You can also check in on the [issues marked as
breaking](https://github.com/tendermint/tendermint/issues?q=is%3Aopen+is%3Aissue+label%3Abreaking).

BREAKING CHANGES:

* CLI/RPC/Config
  - [config] [\#2169](https://github.com/tendermint/tendermint/issues/2169) Replace MaxNumPeers with MaxNumInboundPeers and MaxNumOutboundPeers
  - [config] [\#2300](https://github.com/tendermint/tendermint/issues/2300) Reduce default mempool size from 100k to 5k, until ABCI rechecking is implemented.
  - [rpc] [\#1815](https://github.com/tendermint/tendermint/issues/1815) `/commit` returns a `signed_header` field instead of everything being top-level

* Apps
  - [abci] Added address of the original proposer of the block to Header
  - [abci] Change ABCI Header to match Tendermint exactly
  - [abci] [\#2159](https://github.com/tendermint/tendermint/issues/2159) Update use of `Validator` (see
    [ADR-018](https://github.com/tendermint/tendermint/blob/develop/docs/architecture/adr-018-ABCI-Validators.md)):
    - Remove PubKey from `Validator` (so it's just Address and Power)
    - Introduce `ValidatorUpdate` (with just PubKey and Power)
    - InitChain and EndBlock use ValidatorUpdate
    - Update field names and types in BeginBlock
  - [state] [\#1815](https://github.com/tendermint/tendermint/issues/1815) Validator set changes are now delayed by one block
    - updates returned in ResponseEndBlock for block H will be included in RequestBeginBlock for block H+2

* Go API
  - [lite] [\#1815](https://github.com/tendermint/tendermint/issues/1815) Complete refactor of the package
  - [node] [\#2212](https://github.com/tendermint/tendermint/issues/2212) NewNode now accepts a `*p2p.NodeKey` (@bradyjoestar)
  - [libs/common] [\#2199](https://github.com/tendermint/tendermint/issues/2199) Remove Fmt, in favor of fmt.Sprintf
  - [libs/common] SplitAndTrim was deleted
  - [libs/common] [\#2274](https://github.com/tendermint/tendermint/issues/2274) Remove unused Math functions like MaxInt, MaxInt64,
    MinInt, MinInt64 (@Ahmah2009)
  - [libs/clist] Panics if list extends beyond MaxLength
  - [crypto] [\#2205](https://github.com/tendermint/tendermint/issues/2205) Rename AminoRoute variables to no longer be prefixed by signature type.

* Blockchain Protocol
  - [state] [\#1815](https://github.com/tendermint/tendermint/issues/1815) Validator set changes are now delayed by one block (!)
    - Add NextValidatorSet to State, changes on-disk representation of state
  - [state] [\#2184](https://github.com/tendermint/tendermint/issues/2184) Enforce ConsensusParams.BlockSize.MaxBytes (See
    [ADR-020](https://github.com/tendermint/tendermint/blob/develop/docs/architecture/adr-020-block-size.md)).
    - Remove ConsensusParams.BlockSize.MaxTxs
    - Introduce maximum sizes for all components of a block, including ChainID
  - [types] Updates to the block Header:
    - [\#1815](https://github.com/tendermint/tendermint/issues/1815) NextValidatorsHash - hash of the validator set for the next block,
      so the current validators actually sign over the hash for the new
      validators
    - [\#2106](https://github.com/tendermint/tendermint/issues/2106) ProposerAddress - address of the block's original proposer
  - [consensus] [\#2203](https://github.com/tendermint/tendermint/issues/2203) Implement BFT time
    - Timestamp in block must be monotonic and equal the median of timestamps in block's LastCommit
  - [crypto] [\#2239](https://github.com/tendermint/tendermint/issues/2239) Secp256k1 signature changes (See
    [ADR-014](https://github.com/tendermint/tendermint/blob/develop/docs/architecture/adr-014-secp-malleability.md)):
    - format changed from DER to `r || s`, both little endian encoded as 32 bytes.
    - malleability removed by requiring `s` to be in canonical form.

* P2P Protocol
  - [p2p] [\#2263](https://github.com/tendermint/tendermint/issues/2263) Update secret connection to use a little endian encoded nonce
  - [blockchain] [\#2213](https://github.com/tendermint/tendermint/issues/2213) Fix Amino routes for blockchain reactor messages
    (@peerlink)


FEATURES:
- [types] [\#2015](https://github.com/tendermint/tendermint/issues/2015) Allow genesis file to have 0 validators (@b00f)
  - Initial validator set can be determined by the app in ResponseInitChain
- [rpc] [\#2161](https://github.com/tendermint/tendermint/issues/2161) New event `ValidatorSetUpdates` for when the validator set changes
- [crypto/multisig] [\#2164](https://github.com/tendermint/tendermint/issues/2164) Introduce multisig pubkey and signature format
- [libs/db] [\#2293](https://github.com/tendermint/tendermint/issues/2293) Allow passing options through when creating instances of leveldb dbs

IMPROVEMENTS:
- [docs] Lint documentation with `write-good` and `stop-words`.
- [docs] [\#2249](https://github.com/tendermint/tendermint/issues/2249) Refactor, deduplicate, and improve the ABCI docs and spec (with thanks to @ttmc).
- [scripts] [\#2196](https://github.com/tendermint/tendermint/issues/2196) Added json2wal tool, which is supposed to help our users restore (@bradyjoestar)
  corrupted WAL files and compose test WAL files (@bradyjoestar)
- [mempool] [\#2234](https://github.com/tendermint/tendermint/issues/2234) Now stores txs by hash inside of the cache, to mitigate memory leakage
- [mempool] [\#2166](https://github.com/tendermint/tendermint/issues/2166) Set explicit capacity for map when updating txs (@bluele)

BUG FIXES:
- [config] [\#2284](https://github.com/tendermint/tendermint/issues/2284) Replace `db_path` with `db_dir` from automatically generated configuration files.
- [mempool] [\#2188](https://github.com/tendermint/tendermint/issues/2188) Fix OOM issue from cache map and list getting out of sync
- [state] [\#2051](https://github.com/tendermint/tendermint/issues/2051) KV store index supports searching by `tx.height` (@ackratos)
- [rpc] [\#2327](https://github.com/tendermint/tendermint/issues/2327) `/dial_peers` does not try to dial existing peers
- [node] [\#2323](https://github.com/tendermint/tendermint/issues/2323) Filter empty strings from config lists (@james-ray)
- [abci/client] [\#2236](https://github.com/tendermint/tendermint/issues/2236) Fix closing GRPC connection (@bradyjoestar)

## 0.23.1

*August 22nd, 2018*

BUG FIXES:
- [libs/autofile] [\#2261](https://github.com/tendermint/tendermint/issues/2261) Fix log rotation so it actually happens.
    - Fixes issues with consensus WAL growing unbounded ala [\#2259](https://github.com/tendermint/tendermint/issues/2259)

## 0.23.0

*August 5th, 2018*

This release includes breaking upgrades in our P2P encryption,
some ABCI messages, and how we encode time and signatures.

A few more changes are still coming to the Header, ABCI,
and validator set handling to better support light clients, BFT time, and
upgrades. Most notably, validator set changes will be delayed by one block (see
[#1815][i1815]).

We also removed `make ensure_deps` in favour of `make get_vendor_deps`.

BREAKING CHANGES:
- [abci] Changed time format from int64 to google.protobuf.Timestamp
- [abci] Changed Validators to LastCommitInfo in RequestBeginBlock
- [abci] Removed Fee from ResponseDeliverTx and ResponseCheckTx
- [crypto] Switch crypto.Signature from interface to []byte for space efficiency
  [#2128](https://github.com/tendermint/tendermint/pull/2128)
    - NOTE: this means signatures no longer have the prefix bytes in Amino
      binary nor the `type` field in Amino JSON. They're just bytes.
- [p2p] Remove salsa and ripemd primitives, in favor of using chacha as a stream cipher, and hkdf [#2054](https://github.com/tendermint/tendermint/pull/2054)
- [tools] Removed `make ensure_deps` in favor of `make get_vendor_deps`
- [types] CanonicalTime uses nanoseconds instead of clipping to ms
    - breaks serialization/signing of all messages with a timestamp

FEATURES:
- [tools] Added `make check_dep`
    - ensures gopkg.lock is synced with gopkg.toml
    - ensures no branches are used in the gopkg.toml

IMPROVEMENTS:
- [blockchain] Improve fast-sync logic
  [#1805](https://github.com/tendermint/tendermint/pull/1805)
    - tweak params
    - only process one block at a time to avoid starving
- [common] bit array functions which take in another parameter are now thread safe
- [crypto] Switch hkdfchachapoly1305 to xchachapoly1305
- [p2p] begin connecting to peers as soon a seed node provides them to you ([#2093](https://github.com/tendermint/tendermint/issues/2093))

BUG FIXES:
- [common] Safely handle cases where atomic write files already exist [#2109](https://github.com/tendermint/tendermint/issues/2109)
- [privval] fix a deadline for accepting new connections in socket private
  validator.
- [p2p] Allow startup if a configured seed node's IP can't be resolved ([#1716](https://github.com/tendermint/tendermint/issues/1716))
- [node] Fully exit when CTRL-C is pressed even if consensus state panics [#2072](https://github.com/tendermint/tendermint/issues/2072)

[i1815]: https://github.com/tendermint/tendermint/pull/1815

## 0.22.8

*July 26th, 2018*

BUG FIXES

- [consensus, blockchain] Fix 0.22.7 below.

## 0.22.7

*July 26th, 2018*

BUG FIXES

- [consensus, blockchain] Register the Evidence interface so it can be
  marshalled/unmarshalled by the blockchain and consensus reactors

## 0.22.6

*July 24th, 2018*

BUG FIXES

- [rpc] Fix `/blockchain` endpoint
    - (#2049) Fix OOM attack by returning error on negative input
    - Fix result length to have max 20 (instead of 21) block metas
- [rpc] Validate height is non-negative in `/abci_query`
- [consensus] (#2050) Include evidence in proposal block parts (previously evidence was
  not being included in blocks!)
- [p2p] (#2046) Close rejected inbound connections so file descriptor doesn't
  leak
- [Gopkg] (#2053) Fix versions in the toml

## 0.22.5

*July 23th, 2018*

BREAKING CHANGES:
- [crypto] Refactor `tendermint/crypto` into many subpackages
- [libs/common] remove exponentially distributed random numbers

IMPROVEMENTS:
- [abci, libs/common] Generated gogoproto static marshaller methods
- [config] Increase default send/recv rates to 5 mB/s
- [p2p] reject addresses coming from private peers
- [p2p] allow persistent peers to be private

BUG FIXES:
- [mempool] fixed a race condition when `create_empty_blocks=false` where a
  transaction is published at an old height.
- [p2p] dial external IP setup by `persistent_peers`, not internal NAT IP
- [rpc] make `/status` RPC endpoint resistant to consensus halt

## 0.22.4

*July 14th, 2018*

BREAKING CHANGES:
- [genesis] removed deprecated `app_options` field.
- [types] Genesis.AppStateJSON -> Genesis.AppState

FEATURES:
- [tools] Merged in from github.com/tendermint/tools

BUG FIXES:
- [tools/tm-bench] Various fixes
- [consensus] Wait for WAL to stop on shutdown
- [abci] Fix #1891, pending requests cannot hang when abci server dies.
  Previously a crash in BeginBlock could leave tendermint in broken state.

## 0.22.3

*July 10th, 2018*

IMPROVEMENTS
- Update dependencies
    * pin all values in Gopkg.toml to version or commit
    * update golang/protobuf to v1.1.0

## 0.22.2

*July 10th, 2018*

IMPROVEMENTS
- More cleanup post repo merge!
- [docs] Include `ecosystem.json` and `tendermint-bft.md` from deprecated `aib-data` repository.
- [config] Add `instrumentation.max_open_connections`, which limits the number
  of requests in flight to Prometheus server (if enabled). Default: 3.


BUG FIXES
- [rpc] Allow unquoted integers in requests
    - NOTE: this is only for URI requests. JSONRPC requests and all responses
      will use quoted integers (the proto3 JSON standard).
- [consensus] Fix halt on shutdown

## 0.22.1

*July 5th, 2018*

IMPROVEMENTS

* Cleanup post repo-merge.
* [docs] Various improvements.

BUG FIXES

* [state] Return error when EndBlock returns a 0-power validator that isn't
  already in the validator set.
* [consensus] Shut down WAL properly.


## 0.22.0

*July 2nd, 2018*

BREAKING CHANGES:
- [config]
    * Remove `max_block_size_txs` and `max_block_size_bytes` in favor of
        consensus params from the genesis file.
    * Rename `skip_upnp` to `upnp`, and turn it off by default.
    * Change `max_packet_msg_size` back to `max_packet_msg_payload_size`
- [rpc]
    * All integers are encoded as strings (part of the update for Amino v0.10.1)
    * `syncing` is now called `catching_up`
- [types] Update Amino to v0.10.1
    * Amino is now fully proto3 compatible for the basic types
    * JSON-encoded types now use the type name instead of the prefix bytes
    * Integers are encoded as strings
- [crypto] Update go-crypto to v0.10.0 and merge into `crypto`
    * privKey.Sign returns error.
    * ed25519 address changed to the first 20-bytes of the SHA256 of the raw pubkey bytes
    * `tmlibs/merkle` -> `crypto/merkle`. Uses SHA256 instead of RIPEMD160
- [tmlibs] Update to v0.9.0 and merge into `libs`
    * remove `merkle` package (moved to `crypto/merkle`)

FEATURES
- [cmd] Added metrics (served under `/metrics` using a Prometheus client;
  disabled by default). See the new `instrumentation` section in the config and
  [metrics](https://tendermint.readthedocs.io/projects/tools/en/develop/metrics.html)
  guide.
- [p2p] Add IPv6 support to peering.
- [p2p] Add `external_address` to config to allow specifying the address for
  peers to dial

IMPROVEMENT
- [rpc/client] Supports https and wss now.
- [crypto] Make public key size into public constants
- [mempool] Log tx hash, not entire tx
- [abci] Merged in github.com/tendermint/abci
- [crypto] Merged in github.com/tendermint/go-crypto
- [libs] Merged in github.com/tendermint/tmlibs
- [docs] Move from .rst to .md

BUG FIXES:
- [rpc] Limit maximum number of HTTP/WebSocket connections
  (`rpc.max_open_connections`) and gRPC connections
  (`rpc.grpc_max_open_connections`). Check out "Running In Production" guide if
  you want to increase them.
- [rpc] Limit maximum request body size to 1MB (header is limited to 1MB).
- [consensus] Fix a halting bug where `create_empty_blocks=false`
- [p2p] Fix panic in seed mode

## 0.21.0

*June 21th, 2018*

BREAKING CHANGES

- [config] Change default ports from 4665X to 2665X. Ports over 32768 are
  ephemeral and reserved for use by the kernel.
- [cmd] `unsafe_reset_all` removes the addrbook.json

IMPROVEMENT

- [pubsub] Set default capacity to 0
- [docs] Various improvements

BUG FIXES

- [consensus] Fix an issue where we don't make blocks after `fast_sync` when `create_empty_blocks=false`
- [mempool] Fix #1761 where we don't process txs if `cache_size=0`
- [rpc] Fix memory leak in Websocket (when using `/subscribe` method)
- [config] Escape paths in config - fixes config paths on Windows

## 0.20.0

*June 6th, 2018*

This is the first in a series of breaking releases coming to Tendermint after
soliciting developer feedback and conducting security audits.

This release does not break any blockchain data structures or
protocols other than the ABCI messages between Tendermint and the application.

Applications that upgrade for ABCI v0.11.0 should be able to continue running Tendermint
v0.20.0 on blockchains created with v0.19.X

BREAKING CHANGES

- [abci] Upgrade to
  [v0.11.0](https://github.com/tendermint/abci/blob/master/CHANGELOG.md#0110)
- [abci] Change Query path for filtering peers by node ID from
  `p2p/filter/pubkey/<id>` to `p2p/filter/id/<id>`

## 0.19.9

*June 5th, 2018*

BREAKING CHANGES

- [types/priv_validator] Moved to top level `privval` package

FEATURES

- [config] Collapse PeerConfig into P2PConfig
- [docs] Add quick-install script
- [docs/spec] Add table of Amino prefixes

BUG FIXES

- [rpc] Return 404 for unknown endpoints
- [consensus] Flush WAL on stop
- [evidence] Don't send evidence to peers that are behind
- [p2p] Fix memory leak on peer disconnects
- [rpc] Fix panic when `per_page=0`

## 0.19.8

*June 4th, 2018*

BREAKING:

- [p2p] Remove `auth_enc` config option, peer connections are always auth
  encrypted. Technically a breaking change but seems no one was using it and
  arguably a bug fix :)

BUG FIXES

- [mempool] Fix deadlock under high load when `skip_timeout_commit=true` and
  `create_empty_blocks=false`

## 0.19.7

*May 31st, 2018*

BREAKING:

- [libs/pubsub] TagMap#Get returns a string value
- [libs/pubsub] NewTagMap accepts a map of strings

FEATURES

- [rpc] the RPC documentation is now published to https://tendermint.github.io/slate
- [p2p] AllowDuplicateIP config option to refuse connections from same IP.
    - true by default for now, false by default in next breaking release
- [docs] Add docs for query, tx indexing, events, pubsub
- [docs] Add some notes about running Tendermint in production

IMPROVEMENTS:

- [consensus] Consensus reactor now receives events from a separate synchronous event bus,
  which is not dependant on external RPC load
- [consensus/wal] do not look for height in older files if we've seen height - 1
- [docs] Various cleanup and link fixes

## 0.19.6

*May 29th, 2018*

BUG FIXES

- [blockchain] Fix fast-sync deadlock during high peer turnover

BUG FIX:

- [evidence] Dont send peers evidence from heights they haven't synced to yet
- [p2p] Refuse connections to more than one peer with the same IP
- [docs] Various fixes

## 0.19.5

*May 20th, 2018*

BREAKING CHANGES

- [rpc/client] TxSearch and UnconfirmedTxs have new arguments (see below)
- [rpc/client] TxSearch returns ResultTxSearch
- [version] Breaking changes to Go APIs will not be reflected in breaking
  version change, but will be included in changelog.

FEATURES

- [rpc] `/tx_search` takes `page` (starts at 1) and `per_page` (max 100, default 30) args to paginate results
- [rpc] `/unconfirmed_txs` takes `limit` (max 100, default 30) arg to limit the output
- [config] `mempool.size` and `mempool.cache_size` options

IMPROVEMENTS

- [docs] Lots of updates
- [consensus] Only Fsync() the WAL before executing msgs from ourselves

BUG FIXES

- [mempool] Enforce upper bound on number of transactions

## 0.19.4 (May 17th, 2018)

IMPROVEMENTS

- [state] Improve tx indexing by using batches
- [consensus, state] Improve logging (more consensus logs, fewer tx logs)
- [spec] Moved to `docs/spec` (TODO cleanup the rest of the docs ...)

BUG FIXES

- [consensus] Fix issue #1575 where a late proposer can get stuck

## 0.19.3 (May 14th, 2018)

FEATURES

- [rpc] New `/consensus_state` returns just the votes seen at the current height

IMPROVEMENTS

- [rpc] Add stringified votes and fraction of power voted to `/dump_consensus_state`
- [rpc] Add PeerStateStats to `/dump_consensus_state`

BUG FIXES

- [cmd] Set GenesisTime during `tendermint init`
- [consensus] fix ValidBlock rules

## 0.19.2 (April 30th, 2018)

FEATURES:

- [p2p] Allow peers with different Minor versions to connect
- [rpc] `/net_info` includes `n_peers`

IMPROVEMENTS:

- [p2p] Various code comments, cleanup, error types
- [p2p] Change some Error logs to Debug

BUG FIXES:

- [p2p] Fix reconnect to persistent peer when first dial fails
- [p2p] Validate NodeInfo.ListenAddr
- [p2p] Only allow (MaxNumPeers - MaxNumOutboundPeers) inbound peers
- [p2p/pex] Limit max msg size to 64kB
- [p2p] Fix panic when pex=false
- [p2p] Allow multiple IPs per ID in AddrBook
- [p2p] Fix before/after bugs in addrbook isBad()

## 0.19.1 (April 27th, 2018)

Note this release includes some small breaking changes in the RPC and one in the
config that are really bug fixes. v0.19.1 will work with existing chains, and make Tendermint
easier to use and debug. With <3

BREAKING (MINOR)

- [config] Removed `wal_light` setting. If you really needed this, let us know

FEATURES:

- [networks] moved in tooling from devops repo: terraform and ansible scripts for deploying testnets !
- [cmd] Added `gen_node_key` command

BUG FIXES

Some of these are breaking in the RPC response, but they're really bugs!

- [spec] Document address format and pubkey encoding pre and post Amino
- [rpc] Lower case JSON field names
- [rpc] Fix missing entries, improve, and lower case the fields in `/dump_consensus_state`
- [rpc] Fix NodeInfo.Channels format to hex
- [rpc] Add Validator address to `/status`
- [rpc] Fix `prove` in ABCIQuery
- [cmd] MarshalJSONIndent on init

## 0.19.0 (April 13th, 2018)

BREAKING:
- [cmd] improved `testnet` command; now it can fill in `persistent_peers` for you in the config file and much more (see `tendermint testnet --help` for details)
- [cmd] `show_node_id` now returns an error if there is no node key
- [rpc]: changed the output format for the `/status` endpoint (see https://godoc.org/github.com/tendermint/tendermint/rpc/core#Status)

Upgrade from go-wire to go-amino. This is a sweeping change that breaks everything that is
serialized to disk or over the network.

See github.com/tendermint/go-amino for details on the new format.

See `scripts/wire2amino.go` for a tool to upgrade
genesis/priv_validator/node_key JSON files.

FEATURES

- [test] docker-compose for local testnet setup (thanks Greg!)

## 0.18.0 (April 6th, 2018)

BREAKING:

- [types] Merkle tree uses different encoding for varints (see tmlibs v0.8.0)
- [types] ValidtorSet.GetByAddress returns -1 if no validator found
- [p2p] require all addresses come with an ID no matter what
- [rpc] Listening address must contain tcp:// or unix:// prefix

FEATURES:

- [rpc] StartHTTPAndTLSServer (not used yet)
- [rpc] Include validator's voting power in `/status`
- [rpc] `/tx` and `/tx_search` responses now include the transaction hash
- [rpc] Include peer NodeIDs in `/net_info`

IMPROVEMENTS:
- [config] trim whitespace from elements of lists (like `persistent_peers`)
- [rpc] `/tx_search` results are sorted by height
- [p2p] do not try to connect to ourselves (ok, maybe only once)
- [p2p] seeds respond with a bias towards good peers

BUG FIXES:
- [rpc] fix subscribing using an abci.ResponseDeliverTx tag
- [rpc] fix tx_indexers matchRange
- [rpc] fix unsubscribing (see tmlibs v0.8.0)

## 0.17.1 (March 27th, 2018)

BUG FIXES:
- [types] Actually support `app_state` in genesis as `AppStateJSON`

## 0.17.0 (March 27th, 2018)

BREAKING:
- [types] WriteSignBytes -> SignBytes

IMPROVEMENTS:
- [all] renamed `dummy` (`persistent_dummy`) to `kvstore` (`persistent_kvstore`) (name "dummy" is deprecated and will not work in the next breaking release)
- [docs] note on determinism (docs/determinism.rst)
- [genesis] `app_options` field is deprecated. please rename it to `app_state` in your genesis file(s). `app_options` will not work in the next breaking release
- [p2p] dial seeds directly without potential peers
- [p2p] exponential backoff for addrs in the address book
- [p2p] mark peer as good if it contributed enough votes or block parts
- [p2p] stop peer if it sends incorrect data, msg to unknown channel, msg we did not expect
- [p2p] when `auth_enc` is true, all dialed peers must have a node ID in their address
- [spec] various improvements
- switched from glide to dep internally for package management
- [wire] prep work for upgrading to new go-wire (which is now called go-amino)

FEATURES:
- [config] exposed `auth_enc` flag to enable/disable encryption
- [config] added the `--p2p.private_peer_ids` flag and `PrivatePeerIDs` config variable (see config for description)
- [rpc] added `/health` endpoint, which returns empty result for now
- [types/priv_validator] new format and socket client, allowing for remote signing

BUG FIXES:
- [consensus] fix liveness bug by introducing ValidBlock mechanism

## 0.16.0 (February 20th, 2018)

BREAKING CHANGES:
- [config] use $TMHOME/config for all config and json files
- [p2p] old `--p2p.seeds` is now `--p2p.persistent_peers` (persistent peers to which TM will always connect to)
- [p2p] now `--p2p.seeds` only used for getting addresses (if addrbook is empty; not persistent)
- [p2p] NodeInfo: remove RemoteAddr and add Channels
    - we must have at least one overlapping channel with peer
    - we only send msgs for channels the peer advertised
- [p2p/conn] pong timeout
- [lite] comment out IAVL related code

FEATURES:
- [p2p] added new `/dial_peers&persistent=_` **unsafe** endpoint
- [p2p] persistent node key in `$THMHOME/config/node_key.json`
- [p2p] introduce peer ID and authenticate peers by ID using addresses like `ID@IP:PORT`
- [p2p/pex] new seed mode crawls the network and serves as a seed.
- [config] MempoolConfig.CacheSize
- [config] P2P.SeedMode (`--p2p.seed_mode`)

IMPROVEMENT:
- [p2p/pex] stricter rules in the PEX reactor for better handling of abuse
- [p2p] various improvements to code structure including subpackages for `pex` and `conn`
- [docs] new spec!
- [all] speed up the tests!

BUG FIX:
- [blockchain] StopPeerForError on timeout
- [consensus] StopPeerForError on a bad Maj23 message
- [state] flush mempool conn before calling commit
- [types] fix priv val signing things that only differ by timestamp
- [mempool] fix memory leak causing zombie peers
- [p2p/conn] fix potential deadlock

## 0.15.0 (December 29, 2017)

BREAKING CHANGES:
- [p2p] enable the Peer Exchange reactor by default
- [types] add Timestamp field to Proposal/Vote
- [types] add new fields to Header: TotalTxs, ConsensusParamsHash, LastResultsHash, EvidenceHash
- [types] add Evidence to Block
- [types] simplify ValidateBasic
- [state] updates to support changes to the header
- [state] Enforce <1/3 of validator set can change at a time

FEATURES:
- [state] Send indices of absent validators and addresses of byzantine validators in BeginBlock
- [state] Historical ConsensusParams and ABCIResponses
- [docs] Specification for the base Tendermint data structures.
- [evidence] New evidence reactor for gossiping and managing evidence
- [rpc] `/block_results?height=X` returns the DeliverTx results for a given height.

IMPROVEMENTS:
- [consensus] Better handling of corrupt WAL file

BUG FIXES:
- [lite] fix race
- [state] validate block.Header.ValidatorsHash
- [p2p] allow seed addresses to be prefixed with eg. `tcp://`
- [p2p] use consistent key to refer to peers so we dont try to connect to existing peers
- [cmd] fix `tendermint init` to ignore files that are there and generate files that aren't.

## 0.14.0 (December 11, 2017)

BREAKING CHANGES:
- consensus/wal: removed separator
- rpc/client: changed Subscribe/Unsubscribe/UnsubscribeAll funcs signatures to be identical to event bus.

FEATURES:
- new `tendermint lite` command (and `lite/proxy` pkg) for running a light-client RPC proxy.
    NOTE it is currently insecure and its APIs are not yet covered by semver

IMPROVEMENTS:
- rpc/client: can act as event bus subscriber (See https://github.com/tendermint/tendermint/issues/945).
- p2p: use exponential backoff from seconds to hours when attempting to reconnect to persistent peer
- config: moniker defaults to the machine's hostname instead of "anonymous"

BUG FIXES:
- p2p: no longer exit if one of the seed addresses is incorrect

## 0.13.0 (December 6, 2017)

BREAKING CHANGES:
- abci: update to v0.8 using gogo/protobuf; includes tx tags, vote info in RequestBeginBlock, data.Bytes everywhere, use int64, etc.
- types: block heights are now `int64` everywhere
- types & node: EventSwitch and EventCache have been replaced by EventBus and EventBuffer; event types have been overhauled
- node: EventSwitch methods now refer to EventBus
- rpc/lib/types: RPCResponse is no longer a pointer; WSRPCConnection interface has been modified
- rpc/client: WaitForOneEvent takes an EventsClient instead of types.EventSwitch
- rpc/client: Add/RemoveListenerForEvent are now Subscribe/Unsubscribe
- rpc/core/types: ResultABCIQuery wraps an abci.ResponseQuery
- rpc: `/subscribe` and `/unsubscribe` take `query` arg instead of `event`
- rpc: `/status` returns the LatestBlockTime in human readable form instead of in nanoseconds
- mempool: cached transactions return an error instead of an ABCI response with BadNonce

FEATURES:
- rpc: new `/unsubscribe_all` WebSocket RPC endpoint
- rpc: new `/tx_search` endpoint for filtering transactions by more complex queries
- p2p/trust: new trust metric for tracking peers. See ADR-006
- config: TxIndexConfig allows to set what DeliverTx tags to index

IMPROVEMENTS:
- New asynchronous events system using `tmlibs/pubsub`
- logging: Various small improvements
- consensus: Graceful shutdown when app crashes
- tests: Fix various non-deterministic errors
- p2p: more defensive programming

BUG FIXES:
- consensus: fix panic where prs.ProposalBlockParts is not initialized
- p2p: fix panic on bad channel

## 0.12.1 (November 27, 2017)

BUG FIXES:
- upgrade tmlibs dependency to enable Windows builds for Tendermint

## 0.12.0 (October 27, 2017)

BREAKING CHANGES:
 - rpc/client: websocket ResultsCh and ErrorsCh unified in ResponsesCh.
 - rpc/client: ABCIQuery no longer takes `prove`
 - state: remove GenesisDoc from state.
 - consensus: new binary WAL format provides efficiency and uses checksums to detect corruption
    - use scripts/wal2json to convert to json for debugging

FEATURES:
 - new `Verifiers` pkg contains the tendermint light-client library (name subject to change)!
 - rpc: `/genesis` includes the `app_options` .
 - rpc: `/abci_query` takes an additional `height` parameter to support historical queries.
 - rpc/client: new ABCIQueryWithOptions supports options like `trusted` (set false to get a proof) and `height` to query a historical height.

IMPROVEMENTS:
 - rpc: `/genesis` result includes `app_options`
 - rpc/lib/client: add jitter to reconnects.
 - rpc/lib/types: `RPCError` satisfies the `error` interface.

BUG FIXES:
 - rpc/client: fix ws deadlock after stopping
 - blockchain: fix panic on AddBlock when peer is nil
 - mempool: fix sending on TxsAvailable when a tx has been invalidated
 - consensus: dont run WAL catchup if we fast synced

## 0.11.1 (October 10, 2017)

IMPROVEMENTS:
 - blockchain/reactor: respondWithNoResponseMessage for missing height

BUG FIXES:
 - rpc: fixed client WebSocket timeout
 - rpc: client now resubscribes on reconnection
 - rpc: fix panics on missing params
 - rpc: fix `/dump_consensus_state` to have normal json output (NOTE: technically breaking, but worth a bug fix label)
 - types: fixed out of range error in VoteSet.addVote
 - consensus: fix wal autofile via https://github.com/tendermint/tmlibs/blob/master/CHANGELOG.md#032-october-2-2017

## 0.11.0 (September 22, 2017)

BREAKING:
 - genesis file: validator `amount` is now `power`
 - abci: Info, BeginBlock, InitChain all take structs
 - rpc: various changes to match JSONRPC spec (http://www.jsonrpc.org/specification), including breaking ones:
    - requests that previously returned HTTP code 4XX now return 200 with an error code in the JSONRPC.
    - `rpctypes.RPCResponse` uses new `RPCError` type instead of `string`.

 - cmd: if there is no genesis, exit immediately instead of waiting around for one to show.
 - types: `Signer.Sign` returns an error.
 - state: every validator set change is persisted to disk, which required some changes to the `State` structure.
 - p2p: new `p2p.Peer` interface used for all reactor methods (instead of `*p2p.Peer` struct).

FEATURES:
 - rpc: `/validators?height=X` allows querying of validators at previous heights.
 - rpc: Leaving the `height` param empty for `/block`, `/validators`, and `/commit` will return the value for the latest height.

IMPROVEMENTS:
 - docs: Moved all docs from the website and tools repo in, converted to `.rst`, and cleaned up for presentation on `tendermint.readthedocs.io`

BUG FIXES:
 - fix WAL openning issue on Windows

## 0.10.4 (September 5, 2017)

IMPROVEMENTS:
- docs: Added Slate docs to each rpc function (see rpc/core)
- docs: Ported all website docs to Read The Docs
- config: expose some p2p params to tweak performance: RecvRate, SendRate, and MaxMsgPacketPayloadSize
- rpc: Upgrade the websocket client and server, including improved auto reconnect, and proper ping/pong

BUG FIXES:
- consensus: fix panic on getVoteBitArray
- consensus: hang instead of panicking on byzantine consensus failures
- cmd: dont load config for version command

## 0.10.3 (August 10, 2017)

FEATURES:
- control over empty block production:
  - new flag, `--consensus.create_empty_blocks`; when set to false, blocks are only created when there are txs or when the AppHash changes.
  - new config option, `consensus.create_empty_blocks_interval`; an empty block is created after this many seconds.
  - in normal operation, `create_empty_blocks = true` and `create_empty_blocks_interval = 0`, so blocks are being created all the time (as in all previous versions of tendermint). The number of empty blocks can be reduced by increasing `create_empty_blocks_interval` or by setting `create_empty_blocks = false`.
  - new `TxsAvailable()` method added to Mempool that returns a channel which fires when txs are available.
  - new heartbeat message added to consensus reactor to notify peers that a node is waiting for txs before entering propose step.
- rpc: Add `syncing` field to response returned by `/status`. Is `true` while in fast-sync mode.

IMPROVEMENTS:
- various improvements to documentation and code comments

BUG FIXES:
- mempool: pass height into constructor so it doesn't always start at 0

## 0.10.2 (July 10, 2017)

FEATURES:
- Enable lower latency block commits by adding consensus reactor sleep durations and p2p flush throttle timeout to the config

IMPROVEMENTS:
- More detailed logging in the consensus reactor and state machine
- More in-code documentation for many exposed functions, especially in consensus/reactor.go and p2p/switch.go
- Improved readability for some function definitions and code blocks with long lines

## 0.10.1 (June 28, 2017)

FEATURES:
- Use `--trace` to get stack traces for logged errors
- types: GenesisDoc.ValidatorHash returns the hash of the genesis validator set
- types: GenesisDocFromFile parses a GenesiDoc from a JSON file

IMPROVEMENTS:
- Add a Code of Conduct
- Variety of improvements as suggested by `megacheck` tool
- rpc: deduplicate tests between rpc/client and rpc/tests
- rpc: addresses without a protocol prefix default to `tcp://`. `http://` is also accepted as an alias for `tcp://`
- cmd: commands are more easily reuseable from other tools
- DOCKER: automate build/push

BUG FIXES:
- Fix log statements using keys with spaces (logger does not currently support spaces)
- rpc: set logger on websocket connection
- rpc: fix ws connection stability by setting write deadline on pings

## 0.10.0 (June 2, 2017)

Includes major updates to configuration, logging, and json serialization.
Also includes the Grand Repo-Merge of 2017.

BREAKING CHANGES:

- Config and Flags:
  - The `config` map is replaced with a [`Config` struct](https://github.com/tendermint/tendermint/blob/master/config/config.go#L11),
containing substructs: `BaseConfig`, `P2PConfig`, `MempoolConfig`, `ConsensusConfig`, `RPCConfig`
  - This affects the following flags:
    - `--seeds` is now `--p2p.seeds`
    - `--node_laddr` is now `--p2p.laddr`
    - `--pex` is now `--p2p.pex`
    - `--skip_upnp` is now `--p2p.skip_upnp`
    - `--rpc_laddr` is now `--rpc.laddr`
    - `--grpc_laddr` is now `--rpc.grpc_laddr`
  - Any configuration option now within a substract must come under that heading in the `config.toml`, for instance:
    ```
    [p2p]
    laddr="tcp://1.2.3.4:46656"

    [consensus]
    timeout_propose=1000
    ```
  - Use viper and `DefaultConfig() / TestConfig()` functions to handle defaults, and remove `config/tendermint` and `config/tendermint_test`
  - Change some function and method signatures to
  - Change some [function and method signatures](https://gist.github.com/ebuchman/640d5fc6c2605f73497992fe107ebe0b) accomodate new config

- Logger
  - Replace static `log15` logger with a simple interface, and provide a new implementation using `go-kit`.
See our new [logging library](https://github.com/tendermint/tmlibs/log) and [blog post](https://tendermint.com/blog/abstracting-the-logger-interface-in-go) for more details
  - Levels `warn` and `notice` are removed (you may need to change them in your `config.toml`!)
  - Change some [function and method signatures](https://gist.github.com/ebuchman/640d5fc6c2605f73497992fe107ebe0b) to accept a logger

- JSON serialization:
  - Replace `[TypeByte, Xxx]` with `{"type": "some-type", "data": Xxx}` in RPC and all `.json` files by using `go-wire/data`. For instance, a public key is now:
    ```
    "pub_key": {
      "type": "ed25519",
      "data": "83DDF8775937A4A12A2704269E2729FCFCD491B933C4B0A7FFE37FE41D7760D0"
    }
    ```
  - Remove type information about RPC responses, so `[TypeByte, {"jsonrpc": "2.0", ... }]` is now just `{"jsonrpc": "2.0", ... }`
  - Change `[]byte` to `data.Bytes` in all serialized types (for hex encoding)
  - Lowercase the JSON tags in `ValidatorSet` fields
  - Introduce `EventDataInner` for serializing events

- Other:
  - Send InitChain message in handshake if `appBlockHeight == 0`
  - Do not include the `Accum` field when computing the validator hash. This makes the ValidatorSetHash unique for a given validator set, rather than changing with every block (as the Accum changes)
  - Unsafe RPC calls are not enabled by default. This includes `/dial_seeds`, and all calls prefixed with `unsafe`. Use the `--rpc.unsafe` flag to enable.


FEATURES:

- Per-module log levels. For instance, the new default is `state:info,*:error`, which means the `state` package logs at `info` level, and everything else logs at `error` level
- Log if a node is validator or not in every consensus round
- Use ldflags to set git hash as part of the version
- Ignore `address` and `pub_key` fields in `priv_validator.json` and overwrite them with the values derrived from the `priv_key`

IMPROVEMENTS:

- Merge `tendermint/go-p2p -> tendermint/tendermint/p2p` and `tendermint/go-rpc -> tendermint/tendermint/rpc/lib`
- Update paths for grand repo merge:
  - `go-common -> tmlibs/common`
  - `go-data -> go-wire/data`
  - All other `go-` libs, except `go-crypto` and `go-wire`, are merged under `tmlibs`
- No global loggers (loggers are passed into constructors, or preferably set with a SetLogger method)
- Return HTTP status codes with errors for RPC responses
- Limit `/blockchain_info` call to return a maximum of 20 blocks
- Use `.Wrap()` and `.Unwrap()` instead of eg. `PubKeyS` for `go-crypto` types
- RPC JSON responses use pretty printing (via `json.MarshalIndent`)
- Color code different instances of the consensus for tests
- Isolate viper to `cmd/tendermint/commands` and do not read config from file for tests


## 0.9.2 (April 26, 2017)

BUG FIXES:

- Fix bug in `ResetPrivValidator` where we were using the global config and log (causing external consumers, eg. basecoin, to fail).

## 0.9.1 (April 21, 2017)

FEATURES:

- Transaction indexing - txs are indexed by their hash using a simple key-value store; easily extended to more advanced indexers
- New `/tx?hash=X` endpoint to query for transactions and their DeliverTx result by hash. Optionally returns a proof of the tx's inclusion in the block
- `tendermint testnet` command initializes files for a testnet

IMPROVEMENTS:

- CLI now uses Cobra framework
- TMROOT is now TMHOME (TMROOT will stop working in 0.10.0)
- `/broadcast_tx_XXX` also returns the Hash (can be used to query for the tx)
- `/broadcast_tx_commit` also returns the height the block was committed in
- ABCIResponses struct persisted to disk before calling Commit; makes handshake replay much cleaner
- WAL uses #ENDHEIGHT instead of #HEIGHT (#HEIGHT will stop working in 0.10.0)
- Peers included via `--seeds`, under `seeds` in the config, or in `/dial_seeds` are now persistent, and will be reconnected to if the connection breaks

BUG FIXES:

- Fix bug in fast-sync where we stop syncing after a peer is removed, even if they're re-added later
- Fix handshake replay to handle validator set changes and results of DeliverTx when we crash after app.Commit but before state.Save()

## 0.9.0 (March 6, 2017)

BREAKING CHANGES:

- Update ABCI to v0.4.0, where Query is now `Query(RequestQuery) ResponseQuery`, enabling precise proofs at particular heights:

```
message RequestQuery{
	bytes data = 1;
	string path = 2;
	uint64 height = 3;
	bool prove = 4;
}

message ResponseQuery{
	CodeType          code        = 1;
	int64             index       = 2;
	bytes             key         = 3;
	bytes             value       = 4;
	bytes             proof       = 5;
	uint64            height      = 6;
	string            log         = 7;
}
```


- `BlockMeta` data type unifies its Hash and PartSetHash under a `BlockID`:

```
type BlockMeta struct {
	BlockID BlockID `json:"block_id"` // the block hash and partsethash
	Header  *Header `json:"header"`   // The block's Header
}
```

- `ValidatorSet.Proposer` is exposed as a field and persisted with the `State`. Use `GetProposer()` to initialize or update after validator-set changes.

- `tendermint gen_validator` command output is now pure JSON

FEATURES:

- New RPC endpoint `/commit?height=X` returns header and commit for block at height `X`
- Client API for each endpoint, including mocks for testing

IMPROVEMENTS:

- `Node` is now a `BaseService`
- Simplified starting Tendermint in-process from another application
- Better organized Makefile
- Scripts for auto-building binaries across platforms
- Docker image improved, slimmed down (using Alpine), and changed from tendermint/tmbase to tendermint/tendermint
- New repo files: `CONTRIBUTING.md`, Github `ISSUE_TEMPLATE`, `CHANGELOG.md`
- Improvements on CircleCI for managing build/test artifacts
- Handshake replay is doen through the consensus package, possibly using a mockApp
- Graceful shutdown of RPC listeners
- Tests for the PEX reactor and DialSeeds

BUG FIXES:

- Check peer.Send for failure before updating PeerState in consensus
- Fix panic in `/dial_seeds` with invalid addresses
- Fix proposer selection logic in ValidatorSet by taking the address into account in the `accumComparable`
- Fix inconcistencies with `ValidatorSet.Proposer` across restarts by persisting it in the `State`


## 0.8.0 (January 13, 2017)

BREAKING CHANGES:

- New data type `BlockID` to represent blocks:

```
type BlockID struct {
	Hash        []byte        `json:"hash"`
	PartsHeader PartSetHeader `json:"parts"`
}
```

- `Vote` data type now includes validator address and index:

```
type Vote struct {
	ValidatorAddress []byte           `json:"validator_address"`
	ValidatorIndex   int              `json:"validator_index"`
	Height           int              `json:"height"`
	Round            int              `json:"round"`
	Type             byte             `json:"type"`
	BlockID          BlockID          `json:"block_id"` // zero if vote is nil.
	Signature        crypto.Signature `json:"signature"`
}
```

- Update TMSP to v0.3.0, where it is now called ABCI and AppendTx is DeliverTx
- Hex strings in the RPC are now "0x" prefixed


FEATURES:

- New message type on the ConsensusReactor, `Maj23Msg`, for peers to alert others they've seen a Maj23,
in order to track and handle conflicting votes intelligently to prevent Byzantine faults from causing halts:

```
type VoteSetMaj23Message struct {
	Height  int
	Round   int
	Type    byte
	BlockID types.BlockID
}
```

- Configurable block part set size
- Validator set changes
- Optionally skip TimeoutCommit if we have all the votes
- Handshake between Tendermint and App on startup to sync latest state and ensure consistent recovery from crashes
- GRPC server for BroadcastTx endpoint

IMPROVEMENTS:

- Less verbose logging
- Better test coverage (37% -> 49%)
- Canonical SignBytes for signable types
- Write-Ahead Log for Mempool and Consensus via tmlibs/autofile
- Better in-process testing for the consensus reactor and byzantine faults
- Better crash/restart testing for individual nodes at preset failure points, and of networks at arbitrary points
- Better abstraction over timeout mechanics

BUG FIXES:

- Fix memory leak in mempool peer
- Fix panic on POLRound=-1
- Actually set the CommitTime
- Actually send BeginBlock message
- Fix a liveness issues caused by Byzantine proposals/votes. Uses the new `Maj23Msg`.


## 0.7.4 (December 14, 2016)

FEATURES:

- Enable the Peer Exchange reactor with the `--pex` flag for more resilient gossip network (feature still in development, beware dragons)

IMPROVEMENTS:

- Remove restrictions on RPC endpoint `/dial_seeds` to enable manual network configuration

## 0.7.3 (October 20, 2016)

IMPROVEMENTS:

- Type safe FireEvent
- More WAL/replay tests
- Cleanup some docs

BUG FIXES:

- Fix deadlock in mempool for synchronous apps
- Replay handles non-empty blocks
- Fix race condition in HeightVoteSet

## 0.7.2 (September 11, 2016)

BUG FIXES:

- Set mustConnect=false so tendermint will retry connecting to the app

## 0.7.1 (September 10, 2016)

FEATURES:

- New TMSP connection for Query/Info
- New RPC endpoints:
	- `tmsp_query`
	- `tmsp_info`
- Allow application to filter peers through Query (off by default)

IMPROVEMENTS:

- TMSP connection type enforced at compile time
- All listen/client urls use a "tcp://" or "unix://" prefix

BUG FIXES:

- Save LastSignature/LastSignBytes to `priv_validator.json` for recovery
- Fix event unsubscribe
- Fix fastsync/blockchain reactor

## 0.7.0 (August 7, 2016)

BREAKING CHANGES:

- Strict SemVer starting now!
- Update to ABCI v0.2.0
- Validation types now called Commit
- NewBlock event only returns the block header


FEATURES:

- TMSP and RPC support TCP and UNIX sockets
- Addition config options including block size and consensus parameters
- New WAL mode `cswal_light`; logs only the validator's own votes
- New RPC endpoints:
	- for starting/stopping profilers, and for updating config
	- `/broadcast_tx_commit`, returns when tx is included in a block, else an error
	- `/unsafe_flush_mempool`, empties the mempool


IMPROVEMENTS:

- Various optimizations
- Remove bad or invalidated transactions from the mempool cache (allows later duplicates)
- More elaborate testing using CircleCI including benchmarking throughput on 4 digitalocean droplets

BUG FIXES:

- Various fixes to WAL and replay logic
- Various race conditions

## PreHistory

Strict versioning only began with the release of v0.7.0, in late summer 2016.
The project itself began in early summer 2014 and was workable decentralized cryptocurrency software by the end of that year.
Through the course of 2015, in collaboration with Eris Industries (now Monax Indsutries),
many additional features were integrated, including an implementation from scratch of the Ethereum Virtual Machine.
That implementation now forms the heart of [Burrow](https://github.com/hyperledger/burrow).
In the later half of 2015, the consensus algorithm was upgraded with a more asynchronous design and a more deterministic and robust implementation.

By late 2015, frustration with the difficulty of forking a large monolithic stack to create alternative cryptocurrency designs led to the
invention of the Application Blockchain Interface (ABCI), then called the Tendermint Socket Protocol (TMSP).
The Ethereum Virtual Machine and various other transaction features were removed, and Tendermint was whittled down to a core consensus engine
driving an application running in another process.
The ABCI interface and implementation were iterated on and improved over the course of 2016,
until versioned history kicked in with v0.7.0.
