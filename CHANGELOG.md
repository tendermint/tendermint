# Changelog

## 0.10.0 (May 18, 2017)

BREAKING CHANGES:

- New JSON encoding for `go-crypto` types (using `go-wire/data`):

```
"pub_key": {
  "type": "ed25519",
  "data": "83DDF8775937A4A12A2704269E2729FCFCD491B933C4B0A7FFE37FE41D7760D0"
}
```

- New config
  - Isolate viper to `cmd/tendermint/commands`
  - New Config structs in `config/`: `BaseConfig`, `P2PConfig`, `MempoolConfig`, `ConsensusConfig`
  - Remove config/tendermint and config/tendermint_test. Defaults are handled by viper and `DefaultConfig() / `TestConfig()` functions
  - Tests do not read config from file
- New logger (`github.com/tendermint/tmlibs/log`)
  - Reduced to three levels: `error`, `info`, and `debug`
  - NOTE: The default in previous versions was `notice`, which is no longer valid. Please change it in the `config.toml`
  - Per-module log levels
  - The new default is `state:info,*:error`, which means the `state` package logs at `info` level, and everything else logs at `error` level
  - No global loggers (loggers are passed into constructors, or preferably set with a `SetLogger` method)
- RPC serialization cleanup:
  - Lowercase json names for ValidatorSet fields
  - No longer uses go-wire, so no more `[TypeByte, XXX]` madness
  - Responses have no type information
  - Introduce EventDataInner for serializing events
- Remove all use of go-wire (and `[TypeByte, XXX]`) in the `genesis.json` and `priv_validator.json`
- [consensus/abci] Send InitChain message in handshake if `appBlockHeight == 0`
- [types] `[]byte -> data.Bytes`
- [types] Do not include the `Accum` field when computing the hash of a validator. This makes the ValidatorSetHash unique for a given validator set, rather than changing with every block (as the Accum changes)
- A number of functions and methods ahd their signatures modified to accomodate new config and logger. See https://gist.github.com/ebuchman/640d5fc6c2605f73497992fe107ebe0b for comprehensive list. Note many also had `[]byte` arguments changed to `data.Bytes`, but this is not actually breaking.

FEATURES:

- Log if a node is validator or not in every consensus round
- Use ldflags to set git hash as part of the version

IMPROVEMENTS:

- Merge `tendermint/go-p2p -> tendermint/tendermint/p2p` and `tendermint/go-rpc -> tendermint/tendermint/rpc/lib`
- Update paths for grand repo merge:
  - `go-common -> tmlibs/common`
  - `go-data -> go-wire/data`
  - All other `go-*` libs, except `go-crypto` and `go-wire`, merged under `tmlibs`
- Return HTTP status codes with errors for RPC responses
- Use `.Wrap()` and `.Unwrap()` instead of eg. `PubKeyS` for `go-crypto` types
- Color code different instances of the consensus for tests 
- RPC JSON responses use pretty printing (via `json.MarshalIndent`)


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
That implementation now forms the heart of [ErisDB](https://github.com/eris-ltd/eris-db).
In the later half of 2015, the consensus algorithm was upgraded with a more asynchronous design and a more deterministic and robust implementation.

By late 2015, frustration with the difficulty of forking a large monolithic stack to create alternative cryptocurrency designs led to the 
invention of the Application Blockchain Interface (ABCI), then called the Tendermint Socket Protocol (TMSP).
The Ethereum Virtual Machine and various other transaction features were removed, and Tendermint was whittled down to a core consensus engine
driving an application running in another process. 
The ABCI interface and implementation were iterated on and improved over the course of 2016,
until versioned history kicked in with v0.7.0.
