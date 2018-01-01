Specification
=============

The `primary
specification <https://github.com/tendermint/abci/blob/master/types/types.proto>`__
is made using Protocol Buffers. To build it, run

::

    make protoc

See ``protoc --help`` and `the Protocol Buffers
site <https://developers.google.com/protocol-buffers/>`__ for details on
compiling for other languages. Note we also include a
`GRPC <http://www.grpc.io/docs>`__ service definition.

For the specification as an interface in Go, see the
`types/application.go
file <https://github.com/tendermint/abci/blob/master/types/application.go>`__.

Message Types
~~~~~~~~~~~~~

ABCI requests/responses are defined as simple Protobuf messages in `this
schema
file <https://github.com/tendermint/abci/blob/master/types/types.proto>`__.
TendermintCore sends the requests, and the ABCI application sends the
responses. Here, we describe the requests and responses as function
arguments and return values, and make some notes about usage:

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

-  **Returns**:

   -  ``Data (string)``: Some arbitrary information
   -  ``Version (Version)``: Version information
   -  ``LastBlockHeight (int64)``: Latest block for which the app has
      called Commit
   -  ``LastBlockAppHash ([]byte)``: Latest result of Commit

-  **Usage**: Return information about the application state. Used to
   sync the app with Tendermint on crash/restart.

SetOption
^^^^^^^^^

-  **Arguments**:

   -  ``Key (string)``: Key to set
   -  ``Value (string)``: Value to set for key

-  **Returns**:

   -  ``Code (uint32)``: Response code
   -  ``Log (string)``: Debug or error message

-  **Usage**: Set application options. E.g. Key="mode", Value="mempool"
   for a mempool connection, or Key="mode", Value="consensus" for a
   consensus connection. Other options are application specific.

InitChain
^^^^^^^^^

-  **Arguments**:

   -  ``Validators ([]Validator)``: Initial genesis validators

-  **Usage**: Called once upon genesis

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

   -  ``Code (uint32)``: Response code
   -  ``Key ([]byte)``: The key of the matching data
   -  ``Value ([]byte)``: The value of the matching data
   -  ``Proof ([]byte)``: Proof for the data, if requested
   -  ``Height (int64)``: The block height from which data was derived.
      Note that this is the height of the block containing the
      application's Merkle root hash, which represents the state as it
      was after committing the block at Height-1
   -  ``Log (string)``: Debug or error message

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

-  **Usage**: Signals the beginning of a new block. Called prior to any
   DeliverTxs. The header is expected to at least contain the Height.
   The ``AbsentValidators`` and ``ByzantineValidators`` can be used to
   determine rewards and punishments for the validators.

CheckTx
^^^^^^^

-  **Arguments**:

   -  ``Data ([]byte)``: The request transaction bytes

-  **Returns**:

   -  ``Code (uint32)``: Response code
   -  ``Data ([]byte)``: Result bytes, if any
   -  ``Log (string)``: Debug or error message
   -  ``Gas (int64)``: Amount of gas consumed by transaction
   -  ``Fee (int64)``: Fee paid by transaction

-  **Usage**: Validate a mempool transaction, prior to broadcasting or
   proposing. This message should not mutate the main state, but
   application developers may want to keep a separate CheckTx state that
   gets reset upon Commit.

   CheckTx can happen interspersed with DeliverTx, but they happen on
   different ABCI connections - CheckTx from the mempool connection, and
   DeliverTx from the consensus connection. During Commit, the mempool
   is locked, so you can reset the mempool state to the latest state
   after running all those DeliverTxs, and then the mempool will re-run
   whatever txs it has against that latest mempool state.

   Transactions are first run through CheckTx before broadcast to peers
   in the mempool layer. You can make CheckTx semi-stateful and clear
   the state upon ``Commit`` or ``BeginBlock``, to allow for dependent
   sequences of transactions in the same block.

DeliverTx
^^^^^^^^^

-  **Arguments**:

   -  ``Data ([]byte)``: The request transaction bytes

-  **Returns**:

   -  ``Code (uint32)``: Response code
   -  ``Data ([]byte)``: Result bytes, if any
   -  ``Log (string)``: Debug or error message
   -  ``Tags ([]*KVPair)``: Optional tags for indexing

-  **Usage**: Append and run a transaction. If the transaction is valid,
   returns CodeType.OK

EndBlock
^^^^^^^^

-  **Arguments**:

   -  ``Height (int64)``: The block height that ended

-  **Returns**:

   -  ``ValidatorUpdates ([]Validator)``: Changes to validator set (set
      voting power to 0 to remove)
   -  ``ConsensusParamUpdates (ConsensusParams)``: Changes to
      consensus-critical time/size parameters

-  **Usage**: Signals the end of a block. Called prior to each Commit
   after all transactions. Validator set is updated with the result.

Commit
^^^^^^

-  **Returns**:

   -  ``Data ([]byte)``: The Merkle root hash
   -  ``Log (string)``: Debug or error message

-  **Usage**: Return a Merkle root hash of the application state.
