# Application BlockChain Interface eXtension (ABCIx)

ABCIx is an extension of [ABCI](https://github.com/tendermint/tendermint/tree/master/abci) that offers:
- greater flexibility in block production: when creating a new block, instead of including the transactions from the mempool in an FIFO manner, ABCIx allows the application to decide which transactions from the mempool will be included and their orders; and
- greater security: when creating a block, ABCIx allows the application to detect conflicting transactions and remove them from the mempool.  This ensures that all transactions in a block are valid, while ABCI allows invalid transactions in a block and defers to the application to handle such transactions; and
- backward compatible with ABCI: ABCIx can easily support existing ABCI applications by using ABCI adaptor, which is an ABCIx application and provides ABCI to the underly ABCI applications.

ABCIx provides the following methods:
- CheckTx
Input:

- DeliverTx
