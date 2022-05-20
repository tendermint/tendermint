### Data structures

There are four core components of the blocksync reactor: the reactor itself, a block pool, requesters and peers. 

The reactor verifies received blocks, executes them against the application and commits them into the blockstore of the node. It also sends out requests to peers asking for more blocks and contains the logic to switch from blocksync to consenus.
It contains a pointer to the block pool.

```go 
type Reactor struct {
	service.BaseService
	logger log.Logger

	// immutable
	initialState sm.State
	// store
	stateStore sm.Store

	blockExec   *sm.BlockExecutor
	store       sm.BlockStore
	pool        *BlockPool
	consReactor consensusReactor
	blockSync   *atomicBool

	chCreator  p2p.ChannelCreator
	peerEvents p2p.PeerEventSubscriber

	requestsCh <-chan BlockRequest
	errorsCh   <-chan peerError

	metrics  *consensus.Metrics
	eventBus *eventbus.EventBus

	syncStartTime time.Time

  lastTrustedBlock *TrustedBlockData
}
```

`TrustedBlockData` contains the last synced and verified block along with the commit used to verify it.

```go
type TrustedBlockData struct {
	block  *types.Block
	commit *types.Commit
}
```
The block pool stores the last executed block(`height`), keeps track of peers connected to a node, the current height for each peer, along with the number of pending requestes for each peer and  assigns requests to peers (by creating `requesters`).

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

Each requester is used to track the assignement of a request for a `block` at position `height` to a peer whose id equals to `peerID`. 

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

Each `Peer` data structure stores for each peer its current `height` and number of pending requests sent to the peer (`numPending`), etc. When a block is processed, this number is decremented. 

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

