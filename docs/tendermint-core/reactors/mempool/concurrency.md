# Mempool Concurrency

Look at the concurrency model this uses...

- Receiving CheckTx
- Broadcasting new tx
- Interfaces with consensus engine, reap/update while checking
- Calling the ABCI app (ordering. callbacks. how proxy works alongside the blockchain proxy which actually writes blocks)
