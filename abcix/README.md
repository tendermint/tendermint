# Application BlockChain Interface eXtension (ABCIx)

## Overview
ABCIx is an extension of [ABCI](https://github.com/tendermint/tendermint/tree/master/abci) that offers:
- **greater flexibility in block production**: when creating a new block, instead of including the transactions from the mempool in an FIFO manner, ABCIx allows the application to decide which transactions from the mempool will be included and the order; and
- **greater security**: when creating a block, ABCIx allows the application to detect conflicting transactions and remove them from the mempool.  This ensures that all transactions in a block are valid, while ABCI allows invalid transactions in a block and defers to the application to handle such transactions; and
- **backward compatible with ABCI**: ABCIx can easily support existing ABCI applications by using ABCI adaptor, which is an ABCIx application and provides ABCI to the underly ABCI applications.

Similar to ABCI, ABCIx provides the methods in the following ABCIx [_connections_](https://github.com/tendermint/spec/edit/master/spec/abci/abci.md):

- Consensus connection: `InitChain`, `DeliverBlock`, `CreateBlock`
- Validation connection: `CheckTx`, `CheckBlock`
- Info connection: `Info`, `SetOption`, `Query`
- Snapshot connection: `ListSnapshots`, `LoadSnapshotChunk`, `OfferSnapshot`, `ApplySnapshotChunk`

where
- `CheckTx` will return an extra field `Priority (int64)` to indicate how to order the tx in mempool.
- `CreateBlock` allows the application to iterate the transactions from mempool by the order and select the ones to be included in the block.
- `DeliverBlock` behaves similar to the combination of `ABCI.BeginBlock`, a list of `ABCI.DeliverBlock`, `ABCI.EndBlock`, and `ABCI.Commit` methods.
- `CheckBlock` will be called after a proposed block is received and before the node votes for the block.  `CheckBlock` will return `Code` to determine whether the block is valid or not.  For example, 
  - The block contains invalid transactions; or
  - The block's app hash does not match the app hash determined by application;
  - The block's result hash does not match the results emitted by application;
  - The block's gas usage exceeds the gas limit.
  If the block is invalid, TendermintX will not vote for the block in the following `pre-vote` phase.
- The rest of ABCIx methods should behave the same as those of ABCI.


## Methods
### `CheckTx`
- **Input**:
  - Input of [ABCI.CheckTx](https://github.com/tendermint/spec/blob/master/spec/abci/abci.md#checktx).
- **Output**: 
  - Output of [ABCI.CheckTx](https://github.com/tendermint/spec/blob/master/spec/abci/abci.md#checktx).
  - `Priority (int64)`:  The priority of the tx in mempool.
- **Note**
  - The priority determines the order the tx that will be fed to CreateBlock.
  - If some txs have the same priority in the mempool, they will be ordered in an FIFO manner.
  
### `CreateBlock`
- **Input**
  - `Height (int64)`: Height of the block to be created.
  - `LastCommitInfo (LastCommitInfo)`: Info about the last commit, including the
    round, and the list of validators and which ones signed the last block.
  - `ByzantineValidators ([]Evidence)`: List of evidence of
    validators that acted maliciously.
  - `MempoolIterator (interface{})`: Iterator to iterate all txs in the mempool.  It provides a function `Next(maxBytes int64, maxGas int64) []byte`, which returns a tx ordered by priority with size <= maxBytes and GasWanted <= maxGas or nil if no such tx is found.
 - **Output**
  - `TxList ([][]byte)`: A list of txs will be included in the block.
  - `InvalidTxList ([][]byte)`: A list of txs should be removed from mempool.
  - `AppHash ([]byte)`: App hash after executing the transactions.
  - `Tags ([]kv.Pair)`: A list of tags (events) emitted by the block.
  
  
### `DeliverBlock`
- **Input**
  - Input of ABCI.EndBlock.
  - A list of the inputs of ABCI.DeliverTx.
  - Input of ABCI.BeginBlock.
  - Input of ABCI.Commit.
- **Output**
  - Output of ABCI.BeginBlock.
  - A list of the outputs of ABCI.DeliverTx
  - Output of ABCI.EndBlock.
  - Output of ABCI.Commit.
- **Note**
  - ABCIx will compare the hash of the returned `Tags` with `LastResultHash` in header (`LastResultHash` will be renamed to `ResultHash` in TendermintX).  If the comparison fails, TendermintX will stop working.
  
### `CheckBlock`
- **Input**
  - Input of ABCI.EndBlock.
  - A list of the input of ABCI.DeliverTx.
  - Input of ABCI.BeginBlock.
  - Input of ABCI.Commit.
- **Output**
  - Code (uint32): Response code.
  - Output of ABCI.BeginBlock.
  - A list of the output of ABCI.DeliverTx
  - Output of ABCI.EndBlock.
  - Output of ABCI.Commit.
- **Note**
  - The method will be called after a proposed block is received and before a vote is sent for the block.
  - If `Code` is non-zero, the block is treated as invalid.
  - ABCIx will compare the hash of the returned `Tags` with `LastResultHash` in header (`LastResultHash` will be renamed to `ResultHash` in TendermintX).  If the comparison fails, TendermintX will treat the block as invalid.
  
