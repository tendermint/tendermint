NOTE: this wiki is mostly deprecated and left for archival purposes. Please see the [documentation website](http://tendermint.readthedocs.io/en/master/) which is built from the [docs directory](https://github.com/tendermint/tendermint/tree/master/docs). Additional information about the specification can also be found in that directory.

# Background

For a lighter introduction, see [the ABCI blog post](https://tendermint.com/blog/abci-the-application-blockchain-interface) and the [guide to run your first ABCI application](https://tendermint.com/intro/getting-started/first-abci). Here we'll provide more details about writing ABCI applications and implementing ABCI servers to support new programming languages.

# ABCI Design

The purpose of ABCI is to provide a clean interface between state transition machines on one computer and the mechanics of their replication across multiple computers. The former we call 'application logic' and the latter the 'consensus engine'. Application logic validates transactions and optionally executes transactions against some persistent state. A consensus engine ensures all transactions are replicated in the same order on every machine. We call each machine in a consensus engine a 'validator', and each validator runs the same transactions through the same application logic. In particular, we are interested in blockchain-style consensus engines, where transactions are committed in hash-linked blocks.

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
- ABCI is connection oriented
- Tendermint Core maintains three connections:
- [mempool connection](#mempool-connection): for checking if transactions should be relayed before they are committed. only uses `CheckTx`
- [consensus connection](#consensus-connection): for executing transactions that have been committed. Message sequence is, for every block, `BeginBlock, [DeliverTx, ...], EndBlock, Commit`
- [query connection](#query-connection): for querying the application state.  only uses Query and Info

<img src="http://tendermint.readthedocs.io/en/master/_images/abci.png" width="600">

The mempool and consensus logic act as clients, and each maintains an open TMSP connection with the application, which hosts a TMSP server. Shown are the request and response types sent on each connection.

# Message Protocol

The message protocol consists of pairs of requests and responses. Some messages have no fields, while others may include byte-arrays, strings, or integers. See the `message Request` and `message Response` definitions in [the protobuf definition file](https://github.com/tendermint/tmsp/blob/master/types/types.proto), and the [protobuf documentation](https://developers.google.com/protocol-buffers/docs/overview) for more details.

For each request, a server should respond with the corresponding response, where order of requests is preserved in the order of responses.

# Server

To use ABCI in your programming language of choice, there must be a ABCI server in that language.
Tendermint supports two kinds of implementation of the server:

- Asynchronous, raw socket server
- GRPC

Both can be tested using the `abci-cli` by setting the `--abci` flag appropriately (ie. to `socket` or `grpc`).

See examples, in various stages of maintenance, in [go](https://github.com/tendermint/tmsp/tree/master/server), [javascript](https://github.com/tendermint/js-tmsp), [python](https://github.com/tendermint/tmsp/tree/master/example/python3/tmsp), [c++](https://github.com/mdyring/cpp-tmsp), and [java](https://github.com/jTMSP/jTMSP).

## GRPC

If GRPC is available in your language, this is the easiest approach,
though it will have significant performance overhead.

To get started with GRPC, copy in the [protobuf file](https://github.com/tendermint/tmsp/blob/master/types/types.proto) and compile it using the GRPC plugin for your language.
For instance, for golang, the command is `protoc --go_out=plugins=grpc:. types.proto`. See the [grpc documentation for more details](http://www.grpc.io/docs/). `protoc` will autogenerate all the necessary code for TMSP client and server in your language, including whatever interface your application must satisfy to be used by the TMSP server for handling requests.

## Async Raw

If GRPC is not available in your language, or you require higher performance, or otherwise enjoy programming, you may implement your own ABCI server.
The first step is still to auto-generate the relevant data types and codec in your language using `protoc`.
Messages coming over the socket are Protobuf3 encoded, but additionally length-prefixed to facilitate use as a streaming protocol. Protobuf3 doesn't have an official length-prefix standard, so we use our own. The first byte in the prefix represents the length of the Big Endian encoded length. The remaining bytes in the prefix are the Big Endian encoded length.

For example, if the Protobuf3 encoded TMSP message is 0xDEADBEEF (4 bytes), the length-prefixed message is 0x0104DEADBEEF. If the Protobuf3 encoded TMSP message is 65535 bytes long, the length-prefixed message would be like 0x02FFFF....

Note this prefixing does not apply for grpc.

An ABCI server must also be able to support multiple connections, as Tendermint uses three connections.

# Client

There are currently two use-cases for a ABCI client.
One is a testing tool, as in the `abci-cli`, which allows ABCI requests to be sent via command line.
The other is a consensus engine, such as Tendermint Core, which makes requests to the application every time a new transaction is received or a block is committed.

It is unlikely that you will need to implement a client. For details of our client, see [here](https://github.com/tendermint/abci/tree/master/client).

# Blockchain Protocol

In ABCI, a transaction is simply an arbitrary length byte-array.
It is the application's responsibility to define the transaction codec as they please,
and to use it for both CheckTx and DeliverTx.

Note that there are two distinct means for running transactions, corresponding to stages of 'awareness'
of the transaction in the network. The first stage is when a transaction is received by a validator from a client into the so-called mempool or transaction pool - this is where we use CheckTx. The second is when the transaction is successfully committed on more than 2/3 of validators - where we use DeliverTx. In the former case, it may not be necessary to run all the state transitions associated with the transaction, as the transaction may not ultimately be committed until some much later time, when the result of its execution will be different.
For instance, an Ethereum TMSP app would check signatures and amounts in CheckTx, but would not actually execute any contract code until the DeliverTx, so as to avoid executing state transitions that have not been finalized.

To formalize the distinction further, two explicit ABCI connections are made between Tendermint Core and the application: the mempool connection and the consensus connection. We also make a third connection, the query connection, to query the local state of the app.

## Mempool Connection

The mempool connection is used *only* for CheckTx requests.
Transactions are run using CheckTx in the same order they were received by the validator.
If the CheckTx returns `OK`, the transaction is kept in memory and relayed to other peers in the same order it was received. Otherwise, it is discarded.

CheckTx requests run concurrently with block processing;
so they should run against a copy of the main application state which is reset after every block.
This copy is necessary to track transitions made by a sequence of CheckTx requests before they are included in a block. When a block is committed, the application must ensure to reset the mempool state to the latest committed state. Tendermint Core will then filter through all transactions in the mempool, removing any that were included in the block, and re-run the rest using CheckTx against the post-Commit mempool state.

## Consensus Connection

The consensus connection is used only when a new block is committed, and communicates all information from the block in a series of requests:  `BeginBlock, [DeliverTx, ...], EndBlock, Commit`.
That is, when a block is committed in the consensus, we send a list of DeliverTx requests (one for each transaction) sandwiched by BeginBlock and EndBlock requests, and followed by a Commit.

### DeliverTx

DeliverTx is the workhorse of the blockchain. Tendermint sends the DeliverTx requests asynchronously but in order,
and relies on the underlying socket protocol (ie. TCP) to ensure they are received by the app in order. They have already been ordered in the global consensus by the Tendermint protocol.

DeliverTx returns a abci.Result, which includes a Code, Data, and Log. The code may be non-zero (non-OK), meaning the corresponding transaction should have been rejected by the mempool,
but may have been included in a block by a Byzantine proposer.

The block header will be updated (TODO) to include some commitment to the results of DeliverTx, be it a bitarray of non-OK transactions, or a merkle root of the data returned by the DeliverTx requests, or both.

### Commit

Once all processing of the block is complete, Tendermint sends the Commit request and blocks waiting
for a response. While the mempool may run concurrently with block processing (the BeginBlock, DeliverTxs, and EndBlock), it is locked for the Commit request so that its state can be safely reset during Commit. This means the app *MUST NOT* do any blocking communication with the mempool (ie. broadcast_tx) during Commit, or there will be deadlock. Note also that all remaining transactions in the mempool are replayed on the mempool connection (CheckTx) following a commit.

The Commit response includes a byte array, which is the deterministic state root of the application. It is included in the header of the next block. It can be used to provide easily verified Merkle-proofs of the state of the application.

It is expected that the app will persist state to disk on Commit. The option to have all transactions replayed from some previous block is the job of the [Handshake](#handshake).

### BeginBlock

The BeginBlock request can be used to run some code at the beginning of every block. It also allows Tendermint to send the current block hash and header to the application, before it sends any of the transactions.

The app should remember the latest height and header (ie. from which it has run a successful Commit) so that it can tell Tendermint where to pick up from when it restarts. See [Handshake](#handshake).

### EndBlock

The EndBlock request can be used to run some code at the end of every block. Additionally, the response may contain a list of validators, which can be used to update the validator set. To add a new validator or update an existing one, simply include them in the list returned in the EndBlock response. To remove one, include it in the list with a `power` equal to `0`. Tendermint core will take care of updating the validator set (TODO).

## Query Connection

This connection is used to query the application without engaging consensus. It's exposed over the tendermint core rpc, so clients can query the app without exposing a server on the app itself, but they must serialize each query as a single byte array. Additionally, certain "standardized" queries may be used to inform local decisions, for instance about which peers to connect to.

Tendermint Core currently uses the Query connection to filter peers upon connecting, according to IP address or public key. For instance, returning non-OK TMSP response to either of the following queries will cause Tendermint to not connect to the corresponding peer:

- `p2p/filter/addr/<addr>`, where `<addr>` is an IP address.
- `p2p/filter/pubkey/<pubkey>`, where `<pubkey>` is the hex-encoded ED25519 key of the node (not it's validator key)

Note: these query formats are subject to change!

## Handshake

The ABCI handshake and related improvements are [upcoming in v0.8.0](https://github.com/tendermint/tendermint/issues/300) 
