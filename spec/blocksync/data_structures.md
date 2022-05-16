### Data structures

These are the core data structures necessary to provide the Blocksync Reactor logic.

The requester data structure is used to track the assignment of a request for a `block` at position `height` to a peer whose id equals to `peerID`.

```go
type bpRequester {
  mtx          Mutex
  block        *types.Block
  height       int64
  peerID       types.nodeID
  redoChannel  chan type.nodeID //redo may send multi-time; peerId is used to identify repeat
  goBlockCh chan struct{}{}
}
```
Pool is a core data structure that stores last executed block (`height`), assignment of requests to peers (`requesters`), current height for each peer and number of pending requests for each peer (`peers`), maximum peer height, etc.

```go
type BlockPool {
  mtx                Mutex
  requesters         map[int64]*bpRequester
  height             int64
  peers              map[p2p.ID]*bpPeer
  maxPeerHeight      int64
  numPending         int32
  store              BlockStore
  requestsChannel    chan<- BlockRequest
  errorsChannel      chan<- peerError
}
```

The `Peer` data structure stores for each peer its current `height` and number of pending requests sent to the peer (`numPending`), etc.

```go
type bpPeer struct {
  id           p2p.ID
  height       int64
  base         int64
  numPending   int32
  timeout      *time.Timer
  didTimeout   bool
  pool          *blockPool
}
```

BlockRequest is an internal data structure used to denote current mapping of request for a block at some `height` to a peer (`PeerID`).

```go
type BlockRequest {
  Height int64
  PeerID p2p.ID
}
```

