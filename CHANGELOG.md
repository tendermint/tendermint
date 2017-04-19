# Changelog

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



