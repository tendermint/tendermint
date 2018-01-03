# Blockchain Reactor

The Blockchain Reactor's high level responsibility is to maintain connection to a reasonable number
of peers in the network, request blocks from them or provide them with blocks, validate and persist
the blocks to disk and play blocks to the ABCI app.

## Block Reactor

* coordinates the pool for synching
* coordinates the store for persistence
* coordinates the playing of blocks towards the app using a sm.BlockExecutor
* handles switching between fastsync and consensus
* it is a p2p.BaseReactor
* starts the pool.Start() and its poolRoutine()
* registers all the concrete types and interfaces for serialisation

### poolRoutine

* requests blocks from a specific peer based on the pool
* periodically asks for status updates
* tries to switch to consensus
* tries to sync the app by taking downloaded blocks from the pool, gives them to the app and stores
  them on disk

## Block Pool

* maintain connections to other peers

## Block Store

* persists blocks to disk
