ABCI Specification
==================

NOTE: this file has moved to `specification.md <./specification.md>`__. It is left to prevent link breakages for the forseable future. It can safely be deleted in a few months.

Message Types
~~~~~~~~~~~~~

ABCI requests/responses are defined as simple Protobuf messages in `this
schema
file <https://github.com/tendermint/abci/blob/master/types/types.proto>`__.
TendermintCore sends the requests, and the ABCI application sends the
responses. Here, we provide an overview of the messages types and how they
are used by Tendermint. Then we describe each request-response pair as a
function with arguments and return values, and add some notes on usage.

Some messages (``Echo, Info, InitChain, BeginBlock, EndBlock, Commit``), don't
return errors because an error would indicate a critical failure in the
application and there's nothing Tendermint can do.  The problem should be
addressed and both Tendermint and the application restarted.  All other
messages (``SetOption, Query, CheckTx, DeliverTx``) return an
application-specific response ``Code uint32``, where only ``0`` is reserved for
``OK``.

Some messages (``SetOption, Query, CheckTx, DeliverTx``) return
non-deterministic data in the form of ``Info`` and ``Log``. The ``Log`` is
intended for the literal output from the application's logger, while the
``Info`` is any additional info that should be returned.

The first time a new blockchain is started, Tendermint calls ``InitChain``.
From then on, the Block Execution Sequence that causes the committed state to
be updated is as follows:

``BeginBlock, [DeliverTx], EndBlock, Commit``

where one ``DeliverTx`` is called for each transaction in the block.
Cryptographic commitments to the results of DeliverTx, EndBlock, and
Commit are included in the header of the next block.

Tendermint opens three connections to the application to handle the different message
types:

- ``Consensus Connection - InitChain, BeginBlock, DeliverTx, EndBlock, Commit``

- ``Mempool Connection - CheckTx``

- ``Info Connection - Info, SetOption, Query``

The ``Flush`` message is used on every connection, and the ``Echo`` message
is only used for debugging.

Note that messages may be sent concurrently across all connections -
a typical application will thus maintain a distinct state for each
connection. They may be referred to as the ``DeliverTx state``, the
``CheckTx state``, and the ``Commit state`` respectively.

See below for more details on the message types and how they are used.

Echo
^^^^

-  **Arguments**:

   -  ``Message (string)``: A string to echo back

-  **Returns**:

   -  ``Message (string)``: The input string

-  **Usage**:

   -  Echo a string to test an abci client/server implementation

Flush
^^^^^

-  **Usage**:

   -  Signals that messages queued on the client should be flushed to
      the server. It is called periodically by the client implementation
      to ensure asynchronous requests are actually sent, and is called
      immediately to make a synchronous request, which returns when the
      Flush response comes back.

Info
^^^^

-  **Arguments**:

   -  ``Version (string)``: The Tendermint version

-  **Returns**:

   -  ``Data (string)``: Some arbitrary information
   -  ``Version (Version)``: Version information
   -  ``LastBlockHeight (int64)``: Latest block for which the app has
      called Commit
   -  ``LastBlockAppHash ([]byte)``: Latest result of Commit

-  **Usage**:

   - Return information about the application state.
   - Used to sync Tendermint with the application during a handshake that
     happens on startup.
   - Tendermint expects ``LastBlockAppHash`` and ``LastBlockHeight`` to be
     updated during ``Commit``, ensuring that ``Commit`` is never called twice
     for the same block height.

SetOption
^^^^^^^^^

-  **Arguments**:

   -  ``Key (string)``: Key to set
   -  ``Value (string)``: Value to set for key

-  **Returns**:

   -  ``Code (uint32)``: Response code
   -  ``Log (string)``: The output of the application's logger. May be non-deterministic.
   -  ``Info (string)``: Additional information. May be non-deterministic.

-  **Usage**:

   - Set non-consensus critical application specific options.
   - e.g. Key="min-fee", Value="100fermion" could set the minimum fee required for CheckTx
     (but not DeliverTx - that would be consensus critical).

InitChain
^^^^^^^^^

-  **Arguments**:

   -  ``Validators ([]Validator)``: Initial genesis validators
   -  ``AppStateBytes ([]byte)``: Serialized initial application state

-  **Usage**:

   - Called once upon genesis.

Query
^^^^^

-  **Arguments**:

   -  ``Data ([]byte)``: Raw query bytes. Can be used with or in lieu of
      Path.
   -  ``Path (string)``: Path of request, like an HTTP GET path. Can be
      used with or in liue of Data.
   -  Apps MUST interpret '/store' as a query by key on the underlying
      store. The key SHOULD be specified in the Data field.
   -  Apps SHOULD allow queries over specific types like '/accounts/...'
      or '/votes/...'
   -  ``Height (int64)``: The block height for which you want the query
      (default=0 returns data for the latest committed block). Note that
      this is the height of the block containing the application's
      Merkle root hash, which represents the state as it was after
      committing the block at Height-1
   -  ``Prove (bool)``: Return Merkle proof with response if possible

-  **Returns**:

   -  ``Code (uint32)``: Response code.
   -  ``Log (string)``: The output of the application's logger. May be non-deterministic.
   -  ``Info (string)``: Additional information. May be non-deterministic.
   -  ``Index (int64)``: The index of the key in the tree.
   -  ``Key ([]byte)``: The key of the matching data.
   -  ``Value ([]byte)``: The value of the matching data.
   -  ``Proof ([]byte)``: Proof for the data, if requested.
   -  ``Height (int64)``: The block height from which data was derived.
      Note that this is the height of the block containing the
      application's Merkle root hash, which represents the state as it
      was after committing the block at Height-1

-  **Usage**:

   - Query for data from the application at current or past height.
   - Optionally return Merkle proof.

BeginBlock
^^^^^^^^^^

-  **Arguments**:

   -  ``Hash ([]byte)``: The block's hash. This can be derived from the
      block header.
   -  ``Header (struct{})``: The block header
   -  ``AbsentValidators ([]int32)``: List of indices of validators not
      included in the LastCommit
   -  ``ByzantineValidators ([]Evidence)``: List of evidence of
      validators that acted maliciously

-  **Usage**:

   - Signals the beginning of a new block. Called prior to any DeliverTxs.
   - The header is expected to at least contain the Height.
   - The ``AbsentValidators`` and ``ByzantineValidators`` can be used to
     determine rewards and punishments for the validators.

CheckTx
^^^^^^^

-  **Arguments**:

   -  ``Tx ([]byte)``: The request transaction bytes

-  **Returns**:

   -  ``Code (uint32)``: Response code
   -  ``Data ([]byte)``: Result bytes, if any.
   -  ``Log (string)``: The output of the application's logger. May be non-deterministic.
   -  ``Info (string)``: Additional information. May be non-deterministic.
   -  ``GasWanted (int64)``: Amount of gas request for transaction.
   -  ``GasUsed (int64)``: Amount of gas consumed by transaction.
   -  ``Tags ([]cmn.KVPair)``: Key-Value tags for filtering and indexing transactions (eg. by account).
   -  ``Fee (cmn.KI64Pair)``: Fee paid for the transaction.

-  **Usage**: Validate a mempool transaction, prior to broadcasting or
   proposing. CheckTx should perform stateful but light-weight checks
   of the validity of the transaction (like checking signatures and account balances),
   but need not execute in full (like running a smart contract).

   Tendermint runs CheckTx and DeliverTx concurrently with eachother,
   though on distinct ABCI connections - the mempool connection and the consensus
   connection, respectively.

   The application should maintain a separate state to support CheckTx.
   This state can be reset to the latest committed state during ``Commit``,
   where Tendermint ensures the mempool is locked and not sending new ``CheckTx``.
   After ``Commit``, the mempool will rerun CheckTx on all remaining
   transactions, throwing out any that are no longer valid.

   Keys and values in Tags must be UTF-8 encoded strings (e.g. "account.owner": "Bob", "balance": "100.0", "date": "2018-01-02")


DeliverTx
^^^^^^^^^

-  **Arguments**:

   -  ``Tx ([]byte)``: The request transaction bytes.

-  **Returns**:

   -  ``Code (uint32)``: Response code.
   -  ``Data ([]byte)``: Result bytes, if any.
   -  ``Log (string)``: The output of the application's logger. May be non-deterministic.
   -  ``Info (string)``: Additional information. May be non-deterministic.
   -  ``GasWanted (int64)``: Amount of gas requested for transaction.
   -  ``GasUsed (int64)``: Amount of gas consumed by transaction.
   -  ``Tags ([]cmn.KVPair)``: Key-Value tags for filtering and indexing transactions (eg. by account).
   -  ``Fee (cmn.KI64Pair)``: Fee paid for the transaction.

-  **Usage**:

   - Deliver a transaction to be executed in full by the application. If the transaction is valid,
     returns CodeType.OK.
   - Keys and values in Tags must be UTF-8 encoded strings (e.g. "account.owner": "Bob", "balance": "100.0", "time": "2018-01-02T12:30:00Z")

EndBlock
^^^^^^^^

-  **Arguments**:

   -  ``Height (int64)``: Height of the block just executed.

-  **Returns**:

   -  ``ValidatorUpdates ([]Validator)``: Changes to validator set (set
      voting power to 0 to remove).
   -  ``ConsensusParamUpdates (ConsensusParams)``: Changes to
      consensus-critical time, size, and other parameters.

-  **Usage**:

   - Signals the end of a block.
   - Called prior to each Commit, after all transactions.
   - Validator set and consensus params are updated with the result.
   - Validator pubkeys are expected to be go-wire encoded.

Commit
^^^^^^

-  **Returns**:

   -  ``Data ([]byte)``: The Merkle root hash

-  **Usage**:

   - Persist the application state.
   - Return a Merkle root hash of the application state.
   - It's critical that all application instances return the same hash. If not,
     they will not be able to agree on the next block, because the hash is
     included in the next block!
