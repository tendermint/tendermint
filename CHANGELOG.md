# Changelog

## 0.9.0 (December 28, 2017)

BREAKING CHANGES:
 - [types] Id -> ID
 - [types] ResponseEndBlock: renamed Diffs field to ValidatorUpdates
 - [types] changed protobuf field indices for Request and Response oneof types

FEATURES:
 - [types] ResponseEndBlock: added ConsensusParamUpdates

BUG FIXES:
 - [cmd] fix console and batch commands to use a single persistent connection

## 0.8.0 (December 6, 2017)

BREAKING CHANGES:
 - [client] all XxxSync methods now return (ResponseXxx, error)
 - [types] all methods on Application interface now take RequestXxx and return (ResponseXxx, error).
    - Except `CheckTx`/`DeliverTx`, which takes a `tx []byte` argument.
    - Except `Commit`, which takes no arguments.
 - [types] removed Result and ResultQuery
 - [types] removed CodeType - only `0 == OK` is defined here, everything else is left to convention at the application level
 - [types] switched to using `gogo/protobuf` for code generation
 - [types] use `customtype` feature of `gogo/protobuf` to replace `[]byte` with `data.Bytes` in all generated types :)
    - this eliminates the need for additional types like ResultQuery
 - [types] `pubKey` -> `pub_key`
 - [types] `uint64` -> `int32` for `Header.num_txs` and `PartSetHeader.total`
 - [types] `uint64` -> `int64` for everything else
 - [types] ResponseSetOption includes error code
 - [abci-cli] codes are printed as their number instead of a message, except for `code == 0`, which is still printed as `OK`

FEATURES:
 - [types] ResponseDeliverTx: added `tags` field
 - [types] ResponseCheckTx: added `gas` and `fee` fields
 - [types] RequestBeginBlock: added `absent_validators` and `byzantine_validators` fields
 - [dummy] DeliverTx returns an owner tag and a key tag
 - [abci-cli] added `log_level` flag to control the logger
 - [abci-cli] introduce `abci-cli test` command for simple testing of ABCI server implementations via Counter application

## 0.7.1 (November 14, 2017)

IMPROVEMENTS:
 - [cli] added version command

BUG FIXES:
 - [server] fix "Connection error module=abci-server error=EOF"

## 0.7.0 (October 27, 2017)

BREAKING CHANGES:
 - [cli] consolidate example apps under a single `abci-cli` binary

IMPROVEMENTS:
 - [cli] use spf13/cobra instead of urfave/cli
 - [dummy] use iavl instead of merkleeyes, and add support for historical queries

BUG FIXES:
 - [client] fix deadlock on StopForError

## 0.6.0 (September 22, 2017)

BREAKING CHANGES:

- [types/client] app.BeginBlock takes RequestBeginBlock
- [types/client] app.InitChain takes RequestInitChain
- [types/client] app.Info takes RequestInfo

IMPROVEMENTS:

- various linting

## 0.5.0 (May 18, 2017)

BREAKING CHANGES:

- `NewSocketClient` and `NewGRPCClient` no longer start the client automatically, and don't return errors. The caller is responsible for running `client.Start()` and checking the error.
- `NewSocketServer` and `NewGRPCServer` no longer start the server automatically, and don't return errors. The caller is responsible for running `server.Start()` and checking the error.


FEATURES:

- [types] new method `func (res Result) IsSameCode(compare Result) bool` checks whether two results have the same code
- [types] new methods `func (r *ResponseCheckTx) Result() Result` and `func (r *ResponseDeliverTx) Result() Result` to convert from protobuf types (for control over json serialization)
- [types] new method `func (r *ResponseQuery) Result() *ResultQuery` and struct `ResultQuery` to convert from protobuf types (for control over json serializtion)

IMPROVEMENTS:

- Update imports for new `tmlibs` repository
- Use the new logger
- [abci-cli] Add flags to the query command for `path`, `height`, and `prove`
- [types] use `data.Bytes` and `json` tags in the `Result` struct

BUG FIXES:

## 0.4.1 (April 18, 2017)

IMPROVEMENTS:

- Update dependencies

## 0.4.0 (March 6, 2017)

BREAKING CHANGES:

- Query takes RequestQuery and returns ResponseQuery. The request is split into `data` and `path`,
can specify a height to query the state from, and whether or not the response should come with a proof.
The response returns the corresponding key-value pair, with proof if requested.

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

IMPROVEMENTS:

- Updates to Makefile
- Various cleanup
- BaseApplication can be embedded by new apps to avoid implementing empty methods
- Drop BlockchainAware and make BeginBlock/EndBlock part of the `type Application interface`

## 0.3.0 (January 12, 2017)

BREAKING CHANGES:

- TMSP is now ABCI (Application/Asynchronous/A BlockChain Interface or Atomic BroadCast Interface)
- AppendTx is now DeliverTx (conforms to the literature)
- BeginBlock takes a Header:

```
message RequestBeginBlock{
	bytes hash = 1;
	Header header = 2;
}
```

- Info returns a ResponseInfo, containing last block height and app hash:

```
message ResponseInfo {
	string data = 1;
	string version = 2;
	uint64 last_block_height = 3;
	bytes last_block_app_hash = 4;
}
```

- EndBlock returns a ResponseEndBlock, containing the changed validators:

```
message ResponseEndBlock{
	repeated Validator diffs = 4;
}
```

- Hex strings are 0x-prefixed in the CLI
- Query on the Dummy app now uses hex-strings

FEATURES:

- New app, PersistentDummy, uses Info/BeginBlock to recover from failures and supports validator set changes
- New message types for blockchain data:

```
//----------------------------------------
// Blockchain Types

message Header {
	string chain_id = 1;
	uint64 height = 2;
	uint64 time = 3;
	uint64 num_txs = 4;
	BlockID last_block_id = 5;
	bytes last_commit_hash = 6;
	bytes data_hash = 7;
	bytes validators_hash = 8;
	bytes app_hash = 9;
}

message BlockID {
	bytes hash = 1;
	PartSetHeader parts = 2;
}

message PartSetHeader {
	uint64 total = 1;
	bytes hash = 2;
}

message Validator {
            bytes             pubKey      = 1;
            uint64            power       = 2;
}
```

- Add support for Query to Counter app

IMPROVEMENT:

- Don't exit the tmsp-cli console on bad args

BUG FIXES:

- Fix parsing in the Counter app to handle invalid transactions


## 0.2.1 (September 12, 2016)

IMPROVEMENTS

- Better error handling in console


## 0.2.0 (July 23, 2016)

BREAKING CHANGES:

- Use `oneof` types in protobuf

FEATURES:

- GRPC support


## PreHistory

##### Mar 26h, 2016
* Introduce BeginBlock

##### Mar 6th, 2016

* Added InitChain, EndBlock

##### Feb 14th, 2016

* s/GetHash/Commit/g
* Document Protobuf request/response fields

##### Jan 23th, 2016

* Added CheckTx/Query ABCI message types
* Added Result/Log fields to DeliverTx/CheckTx/SetOption
* Removed Listener messages
* Removed Code from ResponseSetOption and ResponseGetHash
* Made examples BigEndian

##### Jan 12th, 2016

* Added "RetCodeBadNonce = 0x06" return code

##### Jan 8th, 2016

* Tendermint/ABCI now comes to consensus on the order first before DeliverTx.



