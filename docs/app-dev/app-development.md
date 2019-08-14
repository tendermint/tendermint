# Application Development Guide

## XXX

This page is undergoing deprecation. All content is being moved to the new [home
of the ABCI specification](../spec/abci/README.md).

## ABCI Design

The purpose of ABCI is to provide a clean interface between state
transition machines on one computer and the mechanics of their
replication across multiple computers. The former we call 'application
logic' and the latter the 'consensus engine'. Application logic
validates transactions and optionally executes transactions against some
persistent state. A consensus engine ensures all transactions are
replicated in the same order on every machine. We call each machine in a
consensus engine a 'validator', and each validator runs the same
transactions through the same application logic. In particular, we are
interested in blockchain-style consensus engines, where transactions are
committed in hash-linked blocks.

The ABCI design has a few distinct components:

- message protocol
  - pairs of request and response messages
  - consensus makes requests, application responds
  - defined using protobuf
- server/client
  - consensus engine runs the client
  - application runs the server
  - two implementations:
    - async raw bytes
    - grpc
- blockchain protocol
  - abci is connection oriented
  - Tendermint Core maintains three connections:
    - [mempool connection](#mempool-connection): for checking if
      transactions should be relayed before they are committed;
      only uses `CheckTx`
    - [consensus connection](#consensus-connection): for executing
      transactions that have been committed. Message sequence is
      -for every block -`BeginBlock, [DeliverTx, ...], EndBlock, Commit`
    - [query connection](#query-connection): for querying the
      application state; only uses Query and Info

The mempool and consensus logic act as clients, and each maintains an
open ABCI connection with the application, which hosts an ABCI server.
Shown are the request and response types sent on each connection.

Most of the examples below are from [kvstore
application](https://github.com/tendermint/tendermint/blob/master/abci/example/kvstore/kvstore.go),
which is a part of the abci repo. [persistent_kvstore
application](https://github.com/tendermint/tendermint/blob/master/abci/example/kvstore/persistent_kvstore.go)
is used to show `BeginBlock`, `EndBlock` and `InitChain` example
implementations.

## Blockchain Protocol

In ABCI, a transaction is simply an arbitrary length byte-array. It is
the application's responsibility to define the transaction codec as they
please, and to use it for both CheckTx and DeliverTx.

Note that there are two distinct means for running transactions,
corresponding to stages of 'awareness' of the transaction in the
network. The first stage is when a transaction is received by a
validator from a client into the so-called mempool or transaction pool
-this is where we use CheckTx. The second is when the transaction is
successfully committed on more than 2/3 of validators - where we use
DeliverTx. In the former case, it may not be necessary to run all the
state transitions associated with the transaction, as the transaction
may not ultimately be committed until some much later time, when the
result of its execution will be different. For instance, an Ethereum
ABCI app would check signatures and amounts in CheckTx, but would not
actually execute any contract code until the DeliverTx, so as to avoid
executing state transitions that have not been finalized.

To formalize the distinction further, two explicit ABCI connections are
made between Tendermint Core and the application: the mempool connection
and the consensus connection. We also make a third connection, the query
connection, to query the local state of the app.

### Mempool Connection

The mempool connection is used _only_ for CheckTx requests. Transactions
are run using CheckTx in the same order they were received by the
validator. If the CheckTx returns `OK`, the transaction is kept in
memory and relayed to other peers in the same order it was received.
Otherwise, it is discarded.

CheckTx requests run concurrently with block processing; so they should
run against a copy of the main application state which is reset after
every block. This copy is necessary to track transitions made by a
sequence of CheckTx requests before they are included in a block. When a
block is committed, the application must ensure to reset the mempool
state to the latest committed state. Tendermint Core will then filter
through all transactions in the mempool, removing any that were included
in the block, and re-run the rest using CheckTx against the post-Commit
mempool state (this behaviour can be turned off with
`[mempool] recheck = false`).

In go:

```
func (app *KVStoreApplication) CheckTx(req types.RequestCheckTx) types.ResponseCheckTx {
	return types.ResponseCheckTx{Code: code.CodeTypeOK, GasWanted: 1}
}
```

In Java:

```
ResponseCheckTx requestCheckTx(RequestCheckTx req) {
    byte[] transaction = req.getTx().toByteArray();

    // validate transaction

    if (notValid) {
        return ResponseCheckTx.newBuilder().setCode(CodeType.BadNonce).setLog("invalid tx").build();
    } else {
        return ResponseCheckTx.newBuilder().setCode(CodeType.OK).build();
    }
}
```

### Replay Protection

To prevent old transactions from being replayed, CheckTx must implement
replay protection.

Tendermint provides the first defence layer by keeping a lightweight
in-memory cache of 100k (`[mempool] cache_size`) last transactions in
the mempool. If Tendermint is just started or the clients sent more than
100k transactions, old transactions may be sent to the application. So
it is important CheckTx implements some logic to handle them.

If there are cases in your application where a transaction may become invalid in some
future state, you probably want to disable Tendermint's
cache. You can do that by setting `[mempool] cache_size = 0` in the
config.

### Consensus Connection

The consensus connection is used only when a new block is committed, and
communicates all information from the block in a series of requests:
`BeginBlock, [DeliverTx, ...], EndBlock, Commit`. That is, when a block
is committed in the consensus, we send a list of DeliverTx requests (one
for each transaction) sandwiched by BeginBlock and EndBlock requests,
and followed by a Commit.

### DeliverTx

DeliverTx is the workhorse of the blockchain. Tendermint sends the
DeliverTx requests asynchronously but in order, and relies on the
underlying socket protocol (ie. TCP) to ensure they are received by the
app in order. They have already been ordered in the global consensus by
the Tendermint protocol.

DeliverTx returns a abci.Result, which includes a Code, Data, and Log.
The code may be non-zero (non-OK), meaning the corresponding transaction
should have been rejected by the mempool, but may have been included in
a block by a Byzantine proposer.

The block header will be updated (TODO) to include some commitment to
the results of DeliverTx, be it a bitarray of non-OK transactions, or a
merkle root of the data returned by the DeliverTx requests, or both.

In go:

```
// tx is either "key=value" or just arbitrary bytes
func (app *KVStoreApplication) DeliverTx(req types.RequestDeliverTx) types.ResponseDeliverTx {
	var key, value []byte
	parts := bytes.Split(req.Tx, []byte("="))
	if len(parts) == 2 {
		key, value = parts[0], parts[1]
	} else {
		key, value = req.Tx, req.Tx
	}

	app.state.db.Set(prefixKey(key), value)
	app.state.Size += 1

	events := []types.Event{
		{
			Type: "app",
			Attributes: []cmn.KVPair{
				{Key: []byte("creator"), Value: []byte("Cosmoshi Netowoko")},
				{Key: []byte("key"), Value: key},
			},
		},
	}

	return types.ResponseDeliverTx{Code: code.CodeTypeOK, Events: events}
}
```

In Java:

```
/**
 * Using Protobuf types from the protoc compiler, we always start with a byte[]
 */
ResponseDeliverTx deliverTx(RequestDeliverTx request) {
    byte[] transaction  = request.getTx().toByteArray();

    // validate your transaction

    if (notValid) {
        return ResponseDeliverTx.newBuilder().setCode(CodeType.BadNonce).setLog("transaction was invalid").build();
    } else {
        ResponseDeliverTx.newBuilder().setCode(CodeType.OK).build();
    }

}
```

### Commit

Once all processing of the block is complete, Tendermint sends the
Commit request and blocks waiting for a response. While the mempool may
run concurrently with block processing (the BeginBlock, DeliverTxs, and
EndBlock), it is locked for the Commit request so that its state can be
safely updated during Commit. This means the app _MUST NOT_ do any
blocking communication with the mempool (ie. broadcast_tx) during
Commit, or there will be deadlock. Note also that all remaining
transactions in the mempool are replayed on the mempool connection
(CheckTx) following a commit.

The app should respond to the Commit request with a byte array, which is
the deterministic state root of the application. It is included in the
header of the next block. It can be used to provide easily verified
Merkle-proofs of the state of the application.

It is expected that the app will persist state to disk on Commit. The
option to have all transactions replayed from some previous block is the
job of the [Handshake](#handshake).

In go:

```
func (app *KVStoreApplication) Commit() types.ResponseCommit {
	// Using a memdb - just return the big endian size of the db
	appHash := make([]byte, 8)
	binary.PutVarint(appHash, app.state.Size)
	app.state.AppHash = appHash
	app.state.Height += 1
	saveState(app.state)
	return types.ResponseCommit{Data: appHash}
}
```

In Java:

```
ResponseCommit requestCommit(RequestCommit requestCommit) {

    // update the internal app-state
    byte[] newAppState = calculateAppState();

    // and return it to the node
    return ResponseCommit.newBuilder().setCode(CodeType.OK).setData(ByteString.copyFrom(newAppState)).build();
}
```

### BeginBlock

The BeginBlock request can be used to run some code at the beginning of
every block. It also allows Tendermint to send the current block hash
and header to the application, before it sends any of the transactions.

The app should remember the latest height and header (ie. from which it
has run a successful Commit) so that it can tell Tendermint where to
pick up from when it restarts. See information on the Handshake, below.

In go:

```
// Track the block hash and header information
func (app *PersistentKVStoreApplication) BeginBlock(req types.RequestBeginBlock) types.ResponseBeginBlock {
	// reset valset changes
	app.ValUpdates = make([]types.ValidatorUpdate, 0)
	return types.ResponseBeginBlock{}
}
```

In Java:

```
/*
 * all types come from protobuf definition
 */
ResponseBeginBlock requestBeginBlock(RequestBeginBlock req) {

    Header header = req.getHeader();
    byte[] prevAppHash = header.getAppHash().toByteArray();
    long prevHeight = header.getHeight();
    long numTxs = header.getNumTxs();

    // run your pre-block logic. Maybe prepare a state snapshot, message components, etc

    return ResponseBeginBlock.newBuilder().build();
}
```

### EndBlock

The EndBlock request can be used to run some code at the end of every block.
Additionally, the response may contain a list of validators, which can be used
to update the validator set. To add a new validator or update an existing one,
simply include them in the list returned in the EndBlock response. To remove
one, include it in the list with a `power` equal to `0`. Validator's `address`
field can be left empty. Tendermint core will take care of updating the
validator set. Note the change in voting power must be strictly less than 1/3
per block if you want a light client to be able to prove the transition
externally. See the [light client
docs](https://godoc.org/github.com/tendermint/tendermint/lite#hdr-How_We_Track_Validators)
for details on how it tracks validators.

In go:

```
// Update the validator set
func (app *PersistentKVStoreApplication) EndBlock(req types.RequestEndBlock) types.ResponseEndBlock {
	return types.ResponseEndBlock{ValidatorUpdates: app.ValUpdates}
}
```

In Java:

```
/*
 * Assume that one validator changes. The new validator has a power of 10
 */
ResponseEndBlock requestEndBlock(RequestEndBlock req) {
    final long currentHeight = req.getHeight();
    final byte[] validatorPubKey = getValPubKey();

    ResponseEndBlock.Builder builder = ResponseEndBlock.newBuilder();
    builder.addDiffs(1, Types.Validator.newBuilder().setPower(10L).setPubKey(ByteString.copyFrom(validatorPubKey)).build());

    return builder.build();
}
```

### Query Connection

This connection is used to query the application without engaging
consensus. It's exposed over the tendermint core rpc, so clients can
query the app without exposing a server on the app itself, but they must
serialize each query as a single byte array. Additionally, certain
"standardized" queries may be used to inform local decisions, for
instance about which peers to connect to.

Tendermint Core currently uses the Query connection to filter peers upon
connecting, according to IP address or node ID. For instance,
returning non-OK ABCI response to either of the following queries will
cause Tendermint to not connect to the corresponding peer:

- `p2p/filter/addr/<ip addr>`, where `<ip addr>` is an IP address.
- `p2p/filter/id/<id>`, where `<is>` is the hex-encoded node ID (the hash of
  the node's p2p pubkey).

Note: these query formats are subject to change!

In go:

```
func (app *KVStoreApplication) Query(reqQuery types.RequestQuery) (resQuery types.ResponseQuery) {
	if reqQuery.Prove {
		value := app.state.db.Get(prefixKey(reqQuery.Data))
		resQuery.Index = -1 // TODO make Proof return index
		resQuery.Key = reqQuery.Data
		resQuery.Value = value
		if value != nil {
			resQuery.Log = "exists"
		} else {
			resQuery.Log = "does not exist"
		}
		return
	} else {
		resQuery.Key = reqQuery.Data
		value := app.state.db.Get(prefixKey(reqQuery.Data))
		resQuery.Value = value
		if value != nil {
			resQuery.Log = "exists"
		} else {
			resQuery.Log = "does not exist"
		}
		return
	}
}
```

In Java:

```
    ResponseQuery requestQuery(RequestQuery req) {
        final boolean isProveQuery = req.getProve();
        final ResponseQuery.Builder responseBuilder = ResponseQuery.newBuilder();
		byte[] queryData = req.getData().toByteArray();

        if (isProveQuery) {
            com.app.example.QueryResultWithProof result = generateQueryResultWithProof(queryData);
            responseBuilder.setIndex(result.getLeftIndex());
            responseBuilder.setKey(req.getData());
            responseBuilder.setValue(result.getValueOrNull(0));
            responseBuilder.setHeight(result.getHeight());
            responseBuilder.setProof(result.getProof());
            responseBuilder.setLog(result.getLogValue());
        } else {
            com.app.example.QueryResult result = generateQueryResult(queryData);
            responseBuilder.setIndex(result.getIndex());
            responseBuilder.setValue(result.getValue());
            responseBuilder.setLog(result.getLogValue());
        }

        responseBuilder.setIndex(result.getIndex());
        responseBuilder.setValue(ByteString.copyFrom(result.getValue()));
        responseBuilder.setLog(result.getLogValue());
    }

    return responseBuilder.build();
}
```

### Handshake

When the app or tendermint restarts, they need to sync to a common
height. When an ABCI connection is first established, Tendermint will
call `Info` on the Query connection. The response should contain the
LastBlockHeight and LastBlockAppHash - the former is the last block for
which the app ran Commit successfully, the latter is the response from
that Commit.

Using this information, Tendermint will determine what needs to be
replayed, if anything, against the app, to ensure both Tendermint and
the app are synced to the latest block height.

If the app returns a LastBlockHeight of 0, Tendermint will just replay
all blocks.

In go:

```
func (app *KVStoreApplication) Info(req types.RequestInfo) (resInfo types.ResponseInfo) {
	return types.ResponseInfo{
		Data:       fmt.Sprintf("{\"size\":%v}", app.state.Size),
		Version:    version.ABCIVersion,
		AppVersion: ProtocolVersion.Uint64(),
	}
}
```

In Java:

```
ResponseInfo requestInfo(RequestInfo req) {
    final byte[] lastAppHash = getLastAppHash();
    final long lastHeight = getLastHeight();
    return ResponseInfo.newBuilder().setLastBlockAppHash(ByteString.copyFrom(lastAppHash)).setLastBlockHeight(lastHeight).build();
}
```

### Genesis

`InitChain` will be called once upon the genesis. `params` includes the
initial validator set. Later on, it may be extended to take parts of the
consensus params.

In go:

```
// Save the validators in the merkle tree
func (app *PersistentKVStoreApplication) InitChain(req types.RequestInitChain) types.ResponseInitChain {
	for _, v := range req.Validators {
		r := app.updateValidator(v)
		if r.IsErr() {
			app.logger.Error("Error updating validators", "r", r)
		}
	}
	return types.ResponseInitChain{}
}
```

In Java:

```
/*
 * all types come from protobuf definition
 */
ResponseInitChain requestInitChain(RequestInitChain req) {
    final int validatorsCount = req.getValidatorsCount();
    final List<Types.Validator> validatorsList = req.getValidatorsList();

    validatorsList.forEach((validator) -> {
        long power = validator.getPower();
        byte[] validatorPubKey = validator.getPubKey().toByteArray();

        // do somehing for validator setup in app
    });

    return ResponseInitChain.newBuilder().build();
}
```
