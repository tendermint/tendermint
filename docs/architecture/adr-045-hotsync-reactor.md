# ADR 045: HotSync Refactor
Author: Fudong Bai (@guagualvcha)

## Changelog
07-20-2019: Initial draft

## Context

For now, a peer gets two methods to sync or generate blocks:

1. FastSync protocol. If a node is many blocks behind the tip of the chain, FastSync allows them to catch up quickly by downloading blocks in parallel and verifying their commits. But once a node catching up with the chain, it has to switch to consensus protocol even it is not a validator at that height. Because FastSync has to get two continuous blocks to verify the first one, and should aware the height of other peers, so that have a high delay to sync the latest block.
2. Consensus protocol. A peer decides to generate a block according to the proposal, block parts and votes it has received. There is almost no delay between validators and none-validators to decide a new block, but it pays a heavy price for this. The consensus protocol is complicated, and at least three new routines are needed for each connected peer, result in more CPU consumption. And each peer should send redundant messages to make the network aware of state, and much redundant gossip data will be sent too. Also, it should write WAL files to record received votes, block parts and round step changes, even it is useless for none-validators. And the [issue](https://github.com/tendermint/tendermint/issues/1524) of consensus reactor also bothers non-validators. 

A new protocol called HotSync to help witness/sentry/seed nodes gossip blocks in low latency, while cost less network, memory, cpu and disk resources than Tendermint consensus protocol. Even cheap hardware can easily run a full-node, and a sentry node can drive more peers than before. 

## Decision

### Precondition
We need clarify the risk of introducing a new protocol at first. The network will break into two parts running two different protocols. It means if a validator connected to all nono-validators that only run with HotSync protocol, the validator can never join consensus network. 

For a classical network, validators should connect to each other directly and are protected by sentry nodes. Based on classical network model, we would like to provide HotSync to help node outside the core network to sync blocks more easily. But for other kind of topology, HotSync is an extra option for those who understand the risk.

### Main Idea

1. A peer subscribe blocks from other peers. Also a peer only send block message to those who have subscribed block at the specified height. This will dismiss transporting redundant message.
2. The `other peers` are wisely chosen, and in most case only one `other peers` will be chosen. There are strategies to predicate which peers are most likely to have the upcoming blocks, also fairness will be considered in the strategies.
3. A peer will subscribe multiple upcoming blocks in one subscribe request, saving network resource.  
4. A peer can handle subscribe requests, and answer what it has now. Once get the message that the subscribers are interested, will publish to them immediately.
5. To achieve low delay, two kinds of message will be delivered: `ProposalBlock` and `Commit`. A peer running with consensus protocol, should subscribe event `EventDataCompleteProposal` and `EventDataMajorPrecommits` from consensus reactor. Once the peer receives complete proposal block parts, it will broadcast `ProposalBlock` to its subscribers. Once the peer receive 2/3+ precommit, it will broadcast `Commit` to its subscribers.

So that a peer may running in different `SyncPattern`--`Mute`, `Hot` and `Consensus`.

1. `Mute`: will only answer subscribe requests from others, will not sync from others or from consensus reactor. Actually, the HotSync reactor stay in `Mute` only when it is fast syncing.
2. `Hot`:  handle subscribe requests from other peers as a publisher, also subscribe block messages from other peers as a subscriber. A non-validators will stay in `Hot` when the peer have catch up after fast syncing.
3. `Consensus`: handle subscribes requests from other peers as a publisher, but get block/commit message from consensus reactor. A sentry node should stay in `Consensus`. Or a non-validator should switch from `Hot` to `Consensus` when it become a validator.

The viable transitions:
```
                                Hot --> Consensus
                                 ^    ^
                                 |   /
                                 |  /
                                Mute
```

### CandidatePool
`CandidatePool` is designed to help HotSync to decide whom it should subscribe blocks from.

There are `freshSet`, `decayedSet` and `permanentSet` in `CandidatePool`.

- `freshSet`: newly added peers will stay in `freshSet`.
- `decayedSet`: unavailable or poor performance peers will stay in `decayedSet`.
- `permanentSet`: stable and good performance peers will stay in `permanentSet`.

The choose strategy is:
1. First try to pick a peer from `permanentSet` according to the score of each peer.
2. If no permanent peer is picked, all decayed peers will be chosen. Otherwise, give a random decayed peer a try periodically(A decayed peer will have chance to prove its performance after sometime and return back to  `permanentSet`). 
3. All peers in `freshSet` will be chosen. The newly connected peers will have a chance to test its performance immediately.

Every time HotSync receive a valid block from a peer or a timeout event happened, it will fire a `sampleEvent` to `CandidatePool`. `CandidatePool` can handle `sampleEvent` to update `freshSet`, `decayedSet` and `permanentSet`, also update the score of each peer according to the delay of valid response .

### Message

- `blockSubscribeMessage`, subscribe block in range.
```go
type blockSubscribeMessage struct {
	FromHeight int64
	ToHeight   int64
}
```
- `blockUnSubscribeMessage`, send unsubscribe message to keep message spam out when the peer is considered as decayed. Because of peer pick strategy, will not subscribe continuous blocks from peer that whose performance is considered as unknown or bad, and because of reschedule strategy, the previous continuous subscription become discontinuous, `blockUnSubscribeMessage` is designed at specified height rather than in range.
```go
type blockUnSubscribeMessage struct {
	Height int64
}
```
- `blockCommitResponseMessage`, publish block or commit message to the subscriber.
```go
type blockCommitResponseMessage struct {
	Block  *types.Block
	Commit *types.Commit
}
```
- `noBlockResponseMessage`, if the subscribe height is too far or loads nothing from DB, will answer `noBlockResponseMessage`, so that subscriber will not wait for timeout.
```go
type noBlockResponseMessage struct {
	Height int64
}
```

### Key Data Structure

#### BlockState
```go
type blockState struct {
	//--- ops fields
	mux       sync.Mutex
	logger    log.Logger
	startTime time.Time
	pool   *BlockPool
	sealed bool
	//--- data fields
	height  int64
	commit  *types.Commit
	block   *types.Block
	blockId *types.BlockID
	// recently received commit and blockChainMessage
	latestCommit *types.Commit
	latestBlock  *types.Block
	pubStates map[p2p.ID]*publisherState
}
```
If a peer subscribes block at a specified height, it will create a `blockState` to track if it have received valid block at this height. 

For example `PeerA` subscribe `HeightB` from `PeerC` and `PeerD`, if `PeerA` received valid `Block` and `Commit` at `HeightB` from `PeerC` at first, the `blockState` will `seal` immediately: the block will be applied, the next `blockState` will `wakeup`. Then `PeerA` received valid `Block` and `Commit` from `PeerD`, actually will change nothing to `blockState`. The `blockState` will keep in cache for a while in case some peers may subscribe them so that saving loading two blocks from db.

#### PublisherState
```go
type publisherState struct {
	mux        sync.Mutex
	logger     log.Logger
	timeoutDur time.Duration
	bs   *blockState
	pool *BlockPool
	timeout *time.Timer
	isWake  bool
	broken bool
	sealed bool
	pid    p2p.ID
	commit *types.Commit
	block  *types.Block
}
```
If a peer subscribes a block from other peers, will create a `publisherState` for each peer to track the response from each peer. Once timeout or receive valid block from that peer, will try to seal `publisherState` and fire a `sample event` to `CandidatePool`. 

For example `PeerA` subscribe `HeightB` from `PeerC` and `PeerD`, it will create one `blockState` at `HeightB`, `publisherState_C` for `PeerC` and `publisherState_D` for `PeerD`. If `PeerA` received valid `Block` and `Commit` at `HeightB` from `PeerC` at first, `publisherState_C` and `blockState` will `seal`, the block will be applied. Then `PeerA` received valid `Block` and `Commit` from `PeerD`, only will update the `CandidatePool`.

By introducing a `selfID` to distinguish whether message come from other peers or consensus reactor, so `Hot` and `Consensus` `SyncPattern` can share the same framework.

## Status
Proposed

## Consequences


### Positive
Witness/sentry/seed nodes can gossip blocks in low latency, while cost less network, memory, cpu and disk resources than Tendermint consensus protocol. 

### Negative
Users need aware the topology of their network to decide use HotSync or not.

### Neutral

## References

[Implement](https://github.com/tendermint/tendermint/pull/3816)
