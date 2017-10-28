# Changelog

## Roadmap

BREAKING CHANGES:
- Upgrade the header to support better proofs on validtors, results, evidence, and possibly more
- Better support for injecting randomness
- Pass evidence/voteInfo through ABCI
- Upgrade consensus for more real-time use of evidence

FEATURES:
- Peer reputation management
- Use the chain as its own CA for nodes and validators
- Tooling to run multiple blockchains/apps, possibly in a single process
- State syncing (without transaction replay)
- Add authentication and rate-limitting to the RPC

IMPROVEMENTS:
- Improve subtleties around mempool caching and logic
- Consensus optimizations:
	- cache block parts for faster agreement after round changes
	- propagate block parts rarest first
- Better testing of the consensus state machine (ie. use a DSL)
- Auto compiled serialization/deserialization code instead of go-wire reflection

BUG FIXES:
- Graceful handling/recovery for apps that have non-determinism or fail to halt
- Graceful handling/recovery for violations of safety, or liveness

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
