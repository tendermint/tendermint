Application Development Guide
=============================

ABCI Design
-----------

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

-  message protocol

   -  pairs of request and response messages
   -  consensus makes requests, application responds
   -  defined using protobuf

-  server/client

   -  consensus engine runs the client
   -  application runs the server
   -  two implementations:

      -  async raw bytes
      -  grpc

-  blockchain protocol

   -  abci is connection oriented
   -  Tendermint Core maintains three connections:

      -  `mempool connection <#mempool-connection>`__: for checking if
         transactions should be relayed before they are committed; only
         uses ``CheckTx``
      -  `consensus connection <#consensus-connection>`__: for executing
         transactions that have been committed. Message sequence is -
         for every block -
         ``BeginBlock, [DeliverTx, ...], EndBlock, Commit``
      -  `query connection <#query-connection>`__: for querying the
         application state; only uses Query and Info

The mempool and consensus logic act as clients, and each maintains an
open ABCI connection with the application, which hosts an ABCI server.
Shown are the request and response types sent on each connection.

Message Protocol
----------------

The message protocol consists of pairs of requests and responses. Some
messages have no fields, while others may include byte-arrays, strings,
or integers. See the ``message Request`` and ``message Response``
definitions in `the protobuf definition
file <https://github.com/tendermint/abci/blob/master/types/types.proto>`__,
and the `protobuf
documentation <https://developers.google.com/protocol-buffers/docs/overview>`__
for more details.

For each request, a server should respond with the corresponding
response, where order of requests is preserved in the order of
responses.

Server
------

To use ABCI in your programming language of choice, there must be a ABCI
server in that language. Tendermint supports two kinds of implementation
of the server:

-  Asynchronous, raw socket server (Tendermint Socket Protocol, also
   known as TSP or Teaspoon)
-  GRPC

Both can be tested using the ``abci-cli`` by setting the ``--abci`` flag
appropriately (ie. to ``socket`` or ``grpc``).

See examples, in various stages of maintenance, in
`Go <https://github.com/tendermint/abci/tree/master/server>`__,
`JavaScript <https://github.com/tendermint/js-abci>`__,
`Python <https://github.com/tendermint/abci/tree/master/example/python3/abci>`__,
`C++ <https://github.com/mdyring/cpp-tmsp>`__, and
`Java <https://github.com/jTendermint/jabci>`__.

GRPC
~~~~

If GRPC is available in your language, this is the easiest approach,
though it will have significant performance overhead.

To get started with GRPC, copy in the `protobuf
file <https://github.com/tendermint/abci/blob/master/types/types.proto>`__
and compile it using the GRPC plugin for your language. For instance,
for golang, the command is
``protoc --go_out=plugins=grpc:. types.proto``. See the `grpc
documentation for more details <http://www.grpc.io/docs/>`__. ``protoc``
will autogenerate all the necessary code for ABCI client and server in
your language, including whatever interface your application must
satisfy to be used by the ABCI server for handling requests.

TSP
~~~

If GRPC is not available in your language, or you require higher
performance, or otherwise enjoy programming, you may implement your own
ABCI server using the Tendermint Socket Protocol, known affectionately
as Teaspoon. The first step is still to auto-generate the relevant data
types and codec in your language using ``protoc``. Messages coming over
the socket are Protobuf3 encoded, but additionally length-prefixed to
facilitate use as a streaming protocol. Protobuf3 doesn't have an
official length-prefix standard, so we use our own. The first byte in
the prefix represents the length of the Big Endian encoded length. The
remaining bytes in the prefix are the Big Endian encoded length.

For example, if the Protobuf3 encoded ABCI message is 0xDEADBEEF (4
bytes), the length-prefixed message is 0x0104DEADBEEF. If the Protobuf3
encoded ABCI message is 65535 bytes long, the length-prefixed message
would be like 0x02FFFF....

Note this prefixing does not apply for grpc.

An ABCI server must also be able to support multiple connections, as
Tendermint uses three connections.

Client
------

There are currently two use-cases for an ABCI client. One is a testing
tool, as in the ``abci-cli``, which allows ABCI requests to be sent via
command line. The other is a consensus engine, such as Tendermint Core,
which makes requests to the application every time a new transaction is
received or a block is committed.

It is unlikely that you will need to implement a client. For details of
our client, see
`here <https://github.com/tendermint/abci/tree/master/client>`__.

Most of the examples below are from `dummy application
<https://github.com/tendermint/abci/blob/master/example/dummy/dummy.go>`__,
which is a part of the abci repo. `persistent_dummy application
<https://github.com/tendermint/abci/blob/master/example/dummy/persistent_dummy.go>`__
is used to show ``BeginBlock``, ``EndBlock`` and ``InitChain``
example implementations.

Blockchain Protocol
-------------------

In ABCI, a transaction is simply an arbitrary length byte-array. It is
the application's responsibility to define the transaction codec as they
please, and to use it for both CheckTx and DeliverTx.

Note that there are two distinct means for running transactions,
corresponding to stages of 'awareness' of the transaction in the
network. The first stage is when a transaction is received by a
validator from a client into the so-called mempool or transaction pool -
this is where we use CheckTx. The second is when the transaction is
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

Mempool Connection
~~~~~~~~~~~~~~~~~~

The mempool connection is used *only* for CheckTx requests. Transactions
are run using CheckTx in the same order they were received by the
validator. If the CheckTx returns ``OK``, the transaction is kept in
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
mempool state.

::

    func (app *DummyApplication) CheckTx(tx []byte) types.Result {
      return types.OK
    }

Consensus Connection
~~~~~~~~~~~~~~~~~~~~

The consensus connection is used only when a new block is committed, and
communicates all information from the block in a series of requests:
``BeginBlock, [DeliverTx, ...], EndBlock, Commit``. That is, when a
block is committed in the consensus, we send a list of DeliverTx
requests (one for each transaction) sandwiched by BeginBlock and
EndBlock requests, and followed by a Commit.

DeliverTx
^^^^^^^^^

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

::

    // tx is either "key=value" or just arbitrary bytes
    func (app *DummyApplication) DeliverTx(tx []byte) types.Result {
      parts := strings.Split(string(tx), "=")
      if len(parts) == 2 {
        app.state.Set([]byte(parts[0]), []byte(parts[1]))
      } else {
        app.state.Set(tx, tx)
      }
      return types.OK
    }

Commit
^^^^^^

Once all processing of the block is complete, Tendermint sends the
Commit request and blocks waiting for a response. While the mempool may
run concurrently with block processing (the BeginBlock, DeliverTxs, and
EndBlock), it is locked for the Commit request so that its state can be
safely reset during Commit. This means the app *MUST NOT* do any
blocking communication with the mempool (ie. broadcast\_tx) during
Commit, or there will be deadlock. Note also that all remaining
transactions in the mempool are replayed on the mempool connection
(CheckTx) following a commit.

The app should respond to the Commit request with a byte array, which is the deterministic
state root of the application. It is included in the header of the next
block. It can be used to provide easily verified Merkle-proofs of the
state of the application.

It is expected that the app will persist state to disk on Commit. The
option to have all transactions replayed from some previous block is the
job of the `Handshake <#handshake>`__.

::

    func (app *DummyApplication) Commit() types.Result {
      hash := app.state.Hash()
      return types.NewResultOK(hash, "")
    }

BeginBlock
^^^^^^^^^^

The BeginBlock request can be used to run some code at the beginning of
every block. It also allows Tendermint to send the current block hash
and header to the application, before it sends any of the transactions.

The app should remember the latest height and header (ie. from which it
has run a successful Commit) so that it can tell Tendermint where to
pick up from when it restarts. See information on the Handshake, below.

::

    // Track the block hash and header information
    func (app *PersistentDummyApplication) BeginBlock(params types.RequestBeginBlock) {
      // update latest block info
      app.blockHeader = params.Header

      // reset valset changes
      app.changes = make([]*types.Validator, 0)
    }

EndBlock
^^^^^^^^

The EndBlock request can be used to run some code at the end of every
block. Additionally, the response may contain a list of validators,
which can be used to update the validator set. To add a new validator or
update an existing one, simply include them in the list returned in the
EndBlock response. To remove one, include it in the list with a
``power`` equal to ``0``. Tendermint core will take care of updating the
validator set. Note validator set changes are only available in v0.8.0
and up.

::

    // Update the validator set
    func (app *PersistentDummyApplication) EndBlock(height uint64) (resEndBlock types.ResponseEndBlock) {
      return types.ResponseEndBlock{Diffs: app.changes}
    }

Query Connection
~~~~~~~~~~~~~~~~

This connection is used to query the application without engaging
consensus. It's exposed over the tendermint core rpc, so clients can
query the app without exposing a server on the app itself, but they must
serialize each query as a single byte array. Additionally, certain
"standardized" queries may be used to inform local decisions, for
instance about which peers to connect to.

Tendermint Core currently uses the Query connection to filter peers upon
connecting, according to IP address or public key. For instance,
returning non-OK ABCI response to either of the following queries will
cause Tendermint to not connect to the corresponding peer:

-  ``p2p/filter/addr/<addr>``, where ``<addr>`` is an IP address.
-  ``p2p/filter/pubkey/<pubkey>``, where ``<pubkey>`` is the hex-encoded
   ED25519 key of the node (not it's validator key)

Note: these query formats are subject to change!

::

    func (app *DummyApplication) Query(reqQuery types.RequestQuery) (resQuery types.ResponseQuery) {
      if reqQuery.Prove {
        value, proof, exists := app.state.Proof(reqQuery.Data)
        resQuery.Index = -1 // TODO make Proof return index
        resQuery.Key = reqQuery.Data
        resQuery.Value = value
        resQuery.Proof = proof
        if exists {
          resQuery.Log = "exists"
        } else {
          resQuery.Log = "does not exist"
        }
        return
      } else {
        index, value, exists := app.state.Get(reqQuery.Data)
        resQuery.Index = int64(index)
        resQuery.Value = value
        if exists {
          resQuery.Log = "exists"
        } else {
          resQuery.Log = "does not exist"
        }
        return
      }
    }

Handshake
~~~~~~~~~

When the app or tendermint restarts, they need to sync to a common
height. When an ABCI connection is first established, Tendermint will
call ``Info`` on the Query connection. The response should contain the
LastBlockHeight and LastBlockAppHash - the former is the last block for
which the app ran Commit successfully, the latter is the response
from that Commit.

Using this information, Tendermint will determine what needs to be
replayed, if anything, against the app, to ensure both Tendermint and
the app are synced to the latest block height.

If the app returns a LastBlockHeight of 0, Tendermint will just replay
all blocks.

::

    func (app *DummyApplication) Info(req types.RequestInfo) (resInfo types.ResponseInfo) {
      return types.ResponseInfo{Data: cmn.Fmt("{\"size\":%v}", app.state.Size())}
    }

Genesis
~~~~~~~

``InitChain`` will be called once upon the genesis. ``params`` includes the
initial validator set. Later on, it may be extended to take parts of the
consensus params.

::

    // Save the validators in the merkle tree
    func (app *PersistentDummyApplication) InitChain(params types.RequestInitChain) {
      for _, v := range params.Validators {
        r := app.updateValidator(v)
        if r.IsErr() {
          app.logger.Error("Error updating validators", "r", r)
        }
      }
    }
