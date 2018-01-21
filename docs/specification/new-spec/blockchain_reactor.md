# Blockchain Reactor

The Blockchain Reactor's high level responsibility is to enable peers who are
far behind the current state of the consensus to quickly catch up by downloading
many blocks in parallel, verifying their commits, and executing them against the
ABCI application.

Tendermint full nodes run the Blockchain Reactor as a service to provide blocks
to new nodes. New nodes run the Blockchain Reactor in "fast_sync" mode,
where they actively make requests for more blocks until they sync up.
Once caught up, they disable "fast_sync" mode, and turn on the Consensus Reactor.

## Message Types

```go
const (
    msgTypeBlockRequest    = byte(0x10)
    msgTypeBlockResponse   = byte(0x11)
    msgTypeNoBlockResponse = byte(0x12)
    msgTypeStatusResponse  = byte(0x20)
    msgTypeStatusRequest   = byte(0x21)
)
```

```go
type bcBlockRequestMessage struct {
    Height int64
}

type bcNoBlockResponseMessage struct {
    Height int64
}

type bcBlockResponseMessage struct {
    Block Block
}

type bcStatusRequestMessage struct {
    Height int64

type bcStatusResponseMessage struct {
    Height int64
}
```

## Blockchain Reactor

* coordinates the pool for syncing
* coordinates the store for persistence
* coordinates the playing of blocks towards the app using a sm.BlockExecutor
* handles switching between fastsync and consensus
* it is a p2p.BaseReactor
* starts the pool.Start() and its poolRoutine()
* registers all the concrete types and interfaces for serialisation

### poolRoutine

* listens to these channels:
  * pool requests blocks from a specific peer by posting to requestsCh, block reactor then sends
    a &bcBlockRequestMessage for a specific height
  * pool signals timeout of a specific peer by posting to timeoutsCh
  * switchToConsensusTicker to periodically try and switch to consensus
  * trySyncTicker to periodically check if we have fallen behind and then catch-up sync
    * if there aren't any new blocks available on the pool it skips syncing
* tries to sync the app by taking downloaded blocks from the pool, gives them to the app and stores
  them on disk
* implements Receive which is called by the switch/peer
  * calls AddBlock on the pool when it receives a new block from a peer

## Block Pool

* responsible for downloading blocks from peers
* makeRequestersRoutine()
  * removes timeout peers
  * starts new requesters by calling makeNextRequester()
* requestRoutine():
  * picks a peer and sends the request, then blocks until:
    * pool is stopped by listening to pool.Quit
    * requester is stopped by listening to Quit
    * request is redone
    * we receive a block
      * gotBlockCh is strange

## Block Store

* persists blocks to disk

# TODO

* How does the switch from bcR to conR happen? Does conR persist blocks to disk too?
* What is the interaction between the consensus and blockchain reactors?
