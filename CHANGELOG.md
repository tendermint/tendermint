# Changelog

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
 - new `certifiers` pkg contains the tendermint light-client library (name subject to change)!
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
