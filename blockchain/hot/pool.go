package hot

import (
	"bytes"
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/tendermint/tendermint/blockchain"
	cmn "github.com/tendermint/tendermint/libs/common"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/p2p"
	st "github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/types"
)

const (
	// SyncPattern is the work pattern of BlockPool.
	// 1. Mute: will only answer subscribe requests from others, will not sync from others or from consensus channel.
	// 2. Hot:  handle subscribe requests from other peer as a publisher, also subscribe block messages
	//    from other peers as a subscriber.
	// 3. Consensus: handle subscribe requests from other peer as a publisher, but subscribe block message from
	//    consensus channel.
	// The viable transitions between are:
	//                                Hot --> Consensus
	//                                 ^    ^
	//                                 |   /
	//                                 |  /
	//                                Mute
	Mute SyncPattern = iota
	Hot
	Consensus
)

const (
	// the time interval to correct current state.
	tryRepairInterval = 1 * time.Second

	maxCachedSealedBlock = 100

	// the max num of blocks can subscribed in advance.
	maxSubscribeForesight = 40
	// the max num of blocks other peers can subscribe from us ahead of current height.
	// should greater than maxSubscribeForesight.
	maxPublishForesight = 80

	eventBusSubscribeCap = 1000

	subscriber = "HotSyncService"
	selfId     = p2p.ID("self")
)

type SyncPattern uint

type BlockPool struct {
	cmn.BaseService
	mtx       sync.Mutex
	st        SyncPattern
	eventBus  *types.EventBus
	store     *blockchain.BlockStore
	blockExec *st.BlockExecutor

	blockTimeout time.Duration

	//--- state ---
	// the verified blockState received recently
	blockStates        map[int64]*blockState
	subscriberPeerSets map[int64]map[p2p.ID]struct{}
	state              st.State
	blocksSynced       int32
	subscribedHeight   int64
	height             int64

	//--- internal communicate ---
	blockStateSealCh     chan *blockState
	publisherStateSealCh chan *publisherState
	decayedPeers         chan decayedPeer
	messagesCh           chan<- Message

	//--- peer metrics ---
	candidatePool *CandidatePool
	sampleStream  chan<- metricsEvent

	//--- other metrics --
	metrics *Metrics

	//--- for switch ---
	switchCh chan struct{}
	switchWg sync.WaitGroup
}

func NewBlockPool(store *blockchain.BlockStore, blockExec *st.BlockExecutor, state st.State, messagesCh chan<- Message, st SyncPattern, blockTimeout time.Duration) *BlockPool {
	const capacity = 1000
	sampleStream := make(chan metricsEvent, capacity)
	candidates := NewCandidatePool(sampleStream)
	bp := &BlockPool{
		store:                store,
		height:               state.LastBlockHeight,
		blockExec:            blockExec,
		state:                state,
		subscribedHeight:     state.LastBlockHeight,
		blockStates:          make(map[int64]*blockState, maxCachedSealedBlock),
		subscriberPeerSets:   make(map[int64]map[p2p.ID]struct{}, 0),
		publisherStateSealCh: make(chan *publisherState, capacity),
		messagesCh:           messagesCh,
		candidatePool:        candidates,
		sampleStream:         sampleStream,
		switchCh:             make(chan struct{}, 1),
		decayedPeers:         make(chan decayedPeer, capacity),
		blockStateSealCh:     make(chan *blockState, capacity),
		blockTimeout:         blockTimeout,
		st:                   st,
		metrics:              NopMetrics(),
	}
	bp.BaseService = *cmn.NewBaseService(nil, "HotBlockPool", bp)
	return bp
}

func (pool *BlockPool) setEventBus(eventBus *types.EventBus) {
	pool.eventBus = eventBus
}

func (pool *BlockPool) setMetrics(metrics *Metrics) {
	pool.metrics = metrics
	pool.candidatePool.metrics = metrics
}

//---------------Service implement ----------------
func (pool *BlockPool) OnStart() error {
	if pool.st == Consensus {
		pool.Logger.Info("block pool start in consensus pattern")
		go pool.consensusSyncRoutine()
	} else if pool.st == Hot {
		pool.Logger.Info("block pool start in hotSync pattern")
		pool.switchWg.Add(1)
		pool.candidatePool.Start()
		go pool.hotSyncRoutine()
	} else {
		pool.Logger.Info("block pool start in mute pattern")
	}
	return nil
}

func (pool *BlockPool) OnStop() {
	if pool.candidatePool.IsRunning() {
		pool.candidatePool.Stop()
	}
}

func (pool *BlockPool) SetLogger(l log.Logger) {
	pool.BaseService.Logger = l.With("module", "blockPool")
	pool.candidatePool.SetLogger(l.With("module", "candidatePool"))
}

func (pool *BlockPool) AddPeer(peer p2p.Peer) {
	pool.mtx.Lock()
	defer pool.mtx.Unlock()
	// no matter what sync pattern now, need maintain candidatePool.
	pool.candidatePool.AddPeer(peer.ID())
}

func (pool *BlockPool) RemovePeer(peer p2p.Peer) {
	pool.mtx.Lock()
	defer pool.mtx.Unlock()
	// no matter what sync pattern now, need maintain candidatePool.
	pool.candidatePool.RemovePeer(peer.ID())
	if pool.st == Hot {
		pool.decayedPeers <- decayedPeer{peerId: peer.ID()}
	}
	for _, subscribers := range pool.subscriberPeerSets {
		delete(subscribers, peer.ID())
	}
}

// ------  switch logic -----
func (pool *BlockPool) SwitchToHotSync(state st.State, blockSynced int32) {
	pool.mtx.Lock()
	defer pool.mtx.Unlock()
	if pool.st != Mute {
		panic(fmt.Sprintf("can't switch to hotsync pattern, current sync pattern is %d", pool.st))
	}
	pool.Logger.Info("switch to hot sync pattern")
	pool.st = Hot
	pool.state = state
	pool.subscribedHeight = state.LastBlockHeight
	pool.blocksSynced = blockSynced
	pool.height = state.LastBlockHeight

	pool.switchWg.Add(1)
	pool.candidatePool.Start()
	go pool.hotSyncRoutine()
}

func (pool *BlockPool) SwitchToConsensusSync(state *st.State) {
	pool.mtx.Lock()
	defer pool.mtx.Unlock()
	if pool.st == Consensus {
		panic("already in consensus sync, can't switch to consensus sync")
	}
	var copyState st.State
	// means switch from hot sync
	if state == nil {
		copyState = pool.state
	} else {
		copyState = state.Copy()
	}
	pool.switchCh <- struct{}{}
	// wait until hotSyncRoutine ends.
	pool.switchWg.Wait()
	// clean block states from expecting height.
	pool.resetBlockStates()

	pool.Logger.Info("switch to consensus sync pattern")
	pool.st = Consensus
	pool.state = copyState
	pool.height = copyState.LastBlockHeight
	pool.subscribedHeight = copyState.LastBlockHeight

	if pool.candidatePool.IsRunning() {
		pool.candidatePool.Stop()
	}
	go pool.consensusSyncRoutine()
}

//------- handle request from other peer -----
func (pool *BlockPool) handleSubscribeBlock(fromHeight, toHeight int64, src p2p.Peer) {
	pool.mtx.Lock()
	defer pool.mtx.Unlock()
	if pool.st == Mute {
		for height := fromHeight; height <= toHeight; height++ {
			if height < pool.store.Height() {
				pool.sendBlockFromStore(height, src)
			} else {
				// Won't answer no block message immediately.
				// Afraid of the peer will subscribe again immediately, which
				// will cause message burst.
				return
			}
		}
		return
	}
	for height := fromHeight; height <= toHeight; height++ {
		blockState := pool.blockStates[height]
		if blockState != nil && blockState.isSealed() {
			pool.sendMessages(src.ID(), &blockCommitResponseMessage{blockState.block, blockState.commit})
		} else if height <= pool.currentHeight() {
			// Todo: load blockChainMessage from store takes time, should we introduce go pool and do asynchronously?
			pool.sendBlockFromStore(height, src)
		} else if height < pool.currentHeight()+maxPublishForesight {
			// even if the peer have subscribe at the height before, in consideration of
			// network/protocol robustness, still sent what we have again.
			if blockState != nil {
				if blockState.latestBlock != nil {
					pool.sendMessages(src.ID(), &blockCommitResponseMessage{Block: blockState.latestBlock})
				}
				if blockState.latestCommit != nil {
					pool.sendMessages(src.ID(), &blockCommitResponseMessage{Commit: blockState.latestCommit})
				}
			}
			pool.Logger.Debug("handle peer subscribe block", "peer", src.ID(), "height", height)
			if peers, exist := pool.subscriberPeerSets[height]; exist {
				peers[src.ID()] = struct{}{}
			} else {
				pool.subscriberPeerSets[height] = map[p2p.ID]struct{}{src.ID(): {}}
			}
		} else {
			// will not handle higher subscribe request
			pool.sendMessages(src.ID(), &noBlockResponseMessage{height})
			return
		}
	}
	return
}

func (pool *BlockPool) handleUnSubscribeBlock(height int64, src p2p.Peer) {
	pool.mtx.Lock()
	defer pool.mtx.Unlock()
	if pool.st == Mute {
		return
	}
	pool.Logger.Debug("handle peer unsubscribe block", "peer", src.ID(), "height", height)
	if subscribers, exist := pool.subscriberPeerSets[height]; exist {
		delete(subscribers, src.ID())
	}
}

func (pool *BlockPool) handleBlockCommit(block *types.Block, commit *types.Commit, src p2p.Peer) {
	pool.mtx.Lock()
	defer pool.mtx.Unlock()
	if pool.st != Hot {
		return
	}
	var height int64
	// have checked, block and commit can't both be nil
	if block != nil {
		height = block.Height
	} else {
		height = commit.Height()
	}
	if ps := pool.getPublisherAtHeight(height, src.ID()); ps != nil {
		if block != nil {
			ps.receiveBlock(block)
		}
		if commit != nil {
			ps.receiveCommit(commit)
		}
	} else {
		pool.Logger.Info("receive block/commit that is not expected", "peer", src.ID(), "height", height)
	}
}

func (pool *BlockPool) handleNoBlock(height int64, src p2p.Peer) {
	pool.mtx.Lock()
	defer pool.mtx.Unlock()
	if pool.st != Hot {
		return
	}
	if ps := pool.getPublisherAtHeight(height, src.ID()); ps != nil {
		ps.receiveNoBlock()
	} else {
		pool.Logger.Info("receive no block message that is not expected", "peer", src.ID(), "height", height)
	}
}

func (pool *BlockPool) handleBlockFromSelf(height int64, block *types.Block) {
	pool.mtx.Lock()
	defer pool.mtx.Unlock()
	bc, exist := pool.blockStates[height]
	if !exist {
		bc = pool.newBlockStateAtHeight(height)
	}
	ps, exist := bc.pubStates[selfId]
	if !exist {
		ps = NewPeerPublisherState(bc, selfId, pool.Logger, pool.blockTimeout, pool)
		bc.pubStates[selfId] = ps
	}
	ps.receiveBlock(block)
}

func (pool *BlockPool) handleCommitFromSelf(height int64, commit *types.Commit) {
	pool.mtx.Lock()
	defer pool.mtx.Unlock()
	bc, exist := pool.blockStates[height]
	if !exist {
		bc = pool.newBlockStateAtHeight(height)
	}
	ps, exist := bc.pubStates[selfId]
	if !exist {
		ps = NewPeerPublisherState(bc, selfId, pool.Logger, pool.blockTimeout, pool)
		bc.pubStates[selfId] = ps
	}
	ps.receiveCommit(commit)
}

//  -------------------------------------------------
func (pool *BlockPool) hotSyncRoutine() {
	tryRepairTicker := time.NewTicker(tryRepairInterval)
	defer tryRepairTicker.Stop()
	pool.subscribeBlockInForesightExclusive()
	for {
		if !pool.IsRunning() {
			break
		}
		select {
		case bs := <-pool.blockStateSealCh:
			pool.compensatePublish(bs)
			pool.applyBlock(bs)
			pool.recordMetrics(bs.block)
			pool.incBlocksSynced()
			pool.incCurrentHeight()
			pool.trimStaleState()
			// try subscribe more
			pool.subscribeBlockInForesightExclusive()
			pool.wakeupNextBlockState()
		case ps := <-pool.publisherStateSealCh:
			pool.updatePeerMetrics(ps)
			if ps.broken {
				pool.decayedPeers <- decayedPeer{ps.pid, ps.bs.height}
			}
		case peer := <-pool.decayedPeers:
			pool.tryReschedule(peer)
		case <-tryRepairTicker.C:
			pool.tryRepair()
		case <-pool.switchCh:
			pool.Logger.Info("stopping hotsync routine")
			pool.switchWg.Done()
			return
		}
	}
}

func (pool *BlockPool) consensusSyncRoutine() {
	blockSub, err := pool.eventBus.Subscribe(context.Background(), subscriber, types.EventQueryCompleteProposal, eventBusSubscribeCap)
	if err != nil {
		panic(err)
	}
	commitSub, err := pool.eventBus.Subscribe(context.Background(), subscriber, types.EventQueryMajorPrecommits, eventBusSubscribeCap)
	if err != nil {
		panic(err)
	}
	for {
		if !pool.IsRunning() {
			break
		}
		select {
		case bs := <-pool.blockStateSealCh:
			// there is no guarantee that this routine happens before consensus reactor start.
			// should tolerate miss some blocks at first.
			// so we use setCurrentHeight instead of incHeight.
			pool.setCurrentHeight(bs.height)
			pool.compensatePublish(bs)
			pool.trimStaleState()
		case <-pool.publisherStateSealCh:
			// just drain the channel
		case blockData := <-blockSub.Out():
			blockProposal := blockData.Data().(types.EventDataCompleteProposal)
			block := blockProposal.Block
			height := blockProposal.Height
			pool.handleBlockFromSelf(height, &block)
		case commitData := <-commitSub.Out():
			precommit := commitData.Data().(types.EventDataMajorPrecommits)
			// if maj23 is zero, means will have another round.
			if maj23, ok := precommit.Votes.TwoThirdsMajority(); ok && !maj23.IsZero() {
				height := precommit.Height
				commit := precommit.Votes.MakeCommit()
				pool.handleCommitFromSelf(height, commit)
			}
		}
	}

}

//-------------------------------------
func (pool *BlockPool) resetBlockStates() {
	for height := pool.expectingHeight(); height <= pool.subscribedHeight; height++ {
		if pubs := pool.getPublishersAtHeight(height); pubs != nil {
			for _, ps := range pubs {
				if ps.timeout != nil {
					ps.timeout.Stop()
				}
				if !ps.sealed {
					pool.sendMessages(ps.pid, &blockUnSubscribeMessage{Height: ps.bs.height})
				}
			}
		}
		delete(pool.blockStates, height)
	}
}

func (pool *BlockPool) tryReschedule(peer decayedPeer) {
	pool.mtx.Lock()
	defer pool.mtx.Unlock()
	height := peer.fromHeight
	if height == 0 {
		height = pool.expectingHeight()
	}
	pool.Logger.Info("try reschedule for block", "from_height", height, "to_height", pool.subscribedHeight, "peer", peer.peerId)
	for ; height <= pool.subscribedHeight; height++ {
		if pubStates := pool.getPublishersAtHeight(height); pubStates != nil {
			if ps, exist := pubStates[peer.peerId]; exist {
				if ps.timeout != nil {
					ps.timeout.Stop()
				}
				// if the peer is removed, try send unsubscribeMessage actually will do nothing.
				pool.sendMessages(peer.peerId, &blockUnSubscribeMessage{Height: height})
				delete(pubStates, ps.pid)
				// is is possible the current height blockstate is sealed, but current height is not update yet.
				// if it is sealed, just skip
				if !ps.bs.isSealed() && len(pubStates) == 0 {
					// empty now, choose new peers for this height.
					pool.subscribeBlockAtHeight(height, nil)
				}
			}
		}
	}
}

func (pool *BlockPool) tryRepair() {
	pool.mtx.Lock()
	defer pool.mtx.Unlock()
	// have't block any blocks yet.
	if pool.currentHeight() == pool.subscribedHeight {
		pool.subscribeBlockInForesight()
	} else {
		// reschedule may failed because of no peer available, need subscribe again during repair
		for h := pool.expectingHeight(); h <= pool.subscribedHeight; h++ {
			if pubStates := pool.getPublishersAtHeight(h); len(pubStates) == 0 {
				pool.Logger.Info("try resubscribe for block", "height", h)
				pool.subscribeBlockAtHeight(h, nil)
			}
		}
	}
}

func (pool *BlockPool) wakeupNextBlockState() {
	pool.mtx.Lock()
	defer pool.mtx.Unlock()
	pool.Logger.Info("Wake next block", "height", pool.expectingHeight())
	if bs := pool.blockStates[pool.expectingHeight()]; bs != nil {
		// new round, sent new start time
		bs.startTime = time.Now()
		if pubs := pool.getPublishersAtHeight(pool.expectingHeight()); pubs != nil {
			for _, ps := range pubs {
				ps.wakeup()
			}
		}
	}
}

func (pool *BlockPool) sendBlockFromStore(height int64, src p2p.Peer) {
	block := pool.store.LoadBlock(height)
	if block == nil {
		pool.sendMessages(src.ID(), &noBlockResponseMessage{height})
		return
	}
	blockAfter := pool.store.LoadBlock(height + 1)
	if blockAfter == nil {
		pool.sendMessages(src.ID(), &noBlockResponseMessage{height})
		return
	}
	pool.sendMessages(src.ID(), &blockCommitResponseMessage{Block: block, Commit: blockAfter.LastCommit})
}

// applyBlock takes most of time, make thin lock in updateState.
func (pool *BlockPool) applyBlock(bs *blockState) {
	pool.Logger.Info("Apply block", "height", bs.block.Height)
	blockParts := bs.block.MakePartSet(types.BlockPartSizeBytes)
	pool.store.SaveBlock(bs.block, blockParts, bs.commit)
	// get the hash without persisting the state
	newState, err := pool.blockExec.ApplyBlock(pool.state, *bs.blockId, bs.block)
	if err != nil {
		panic(fmt.Sprintf("Failed to process committed blockChainMessage (%d:%X): %v", bs.block.Height, bs.block.Hash(), err))
	}
	pool.updateState(newState)
}

func (pool *BlockPool) updateState(state st.State) {
	pool.mtx.Lock()
	defer pool.mtx.Unlock()
	pool.state = state
}

// the subscribe strategy:
//
// one candidate:
// 1. if `subscribedHeight-currentHeight > maxSubscribeForesight/2`, means have subscribe many blocks now, do nothing
// 2. else [subscribedHeight + 1, currentHeight+maxSubscribeForesight] subscribe request will be send.
//
// more than one candidates: only block at `subscribedHeight + 1` will be subscribed.
func (pool *BlockPool) subscribeBlockInForesight() {
	peers := pool.candidatePool.PickCandidates()
	if len(peers) == 0 {
		pool.Logger.Error("no peers is available", "height", pool.currentHeight())
		return
	}
	var fromHeight, toHeight int64
	if len(peers) == 1 {
		peer := peers[0]
		if pool.subscribedHeight-pool.currentHeight() > maxSubscribeForesight/2 {
			return
		} else {
			fromHeight = pool.subscribedHeight + 1
			toHeight = pool.currentHeight() + maxSubscribeForesight
		}
		for h := fromHeight; h <= toHeight; h++ {
			bc := pool.newBlockStateAtHeight(h)
			bc.pubStates[*peer] = NewPeerPublisherState(bc, *peer, pool.Logger, pool.blockTimeout, pool)
		}
		pool.sendMessages(*peer, &blockSubscribeMessage{fromHeight, toHeight})
		pool.subscribedHeight = toHeight
		return
	} else if len(peers) > 1 {
		if pool.subscribedHeight+1 <= pool.currentHeight()+maxSubscribeForesight {
			height := pool.subscribedHeight + 1
			pool.subscribeBlockAtHeight(height, peers)
			pool.subscribedHeight = height
		}
		return
	}
}

func (pool *BlockPool) subscribeBlockInForesightExclusive() {
	pool.mtx.Lock()
	defer pool.mtx.Unlock()
	pool.subscribeBlockInForesight()

}

func (pool *BlockPool) newBlockStateAtHeight(height int64) *blockState {
	newBc := NewBlockState(height, pool.Logger, pool)
	pool.blockStates[height] = newBc
	return newBc
}

func (pool *BlockPool) subscribeBlockAtHeight(height int64, peers []*p2p.ID) {
	if peers == nil {
		peers = pool.candidatePool.PickCandidates()
	}
	if len(peers) == 0 {
		pool.Logger.Error("no peers is available", "height", pool.currentHeight())
		return
	}
	messages := make([]Message, 0, len(peers))
	bc := pool.newBlockStateAtHeight(height)
	for _, peer := range peers {
		ps := NewPeerPublisherState(bc, *peer, pool.Logger, pool.blockTimeout, pool)
		bc.pubStates[*peer] = ps
		messages = append(messages, Message{&blockSubscribeMessage{height, height}, *peer})
	}
	for _, m := range messages {
		pool.sendMessages(m.peerId, m.blockChainMessage)
	}
}

// May publish order and commit in incorrect order, try correct it once we know the true block and commit.
// It is expensive to maintain all received blocks before receiving a
// valid commit, this is a trade off between complexity, memory and network.
func (pool *BlockPool) compensatePublish(bs *blockState) {
	pool.mtx.Lock()
	defer pool.mtx.Unlock()
	if !bytes.Equal(bs.block.Hash(), bs.latestBlock.Hash()) {
		bs.pool.publishBlock(bs.height, bs.block)
	}
	if !bytes.Equal(bs.commit.Hash(), bs.latestCommit.Hash()) {
		bs.pool.publishCommit(bs.height, bs.commit)
	}
	delete(bs.pool.subscriberPeerSets, bs.height)
}

func (pool *BlockPool) publishBlock(height int64, block *types.Block) {
	if subscribers := pool.subscriberPeerSets[height]; len(subscribers) > 0 {
		for p := range subscribers {
			pool.sendMessages(p, &blockCommitResponseMessage{Block: block})
		}
	}
}

func (pool *BlockPool) publishCommit(height int64, commit *types.Commit) {
	if subscribers := pool.subscriberPeerSets[height]; len(subscribers) > 0 {
		for p := range subscribers {
			pool.sendMessages(p, &blockCommitResponseMessage{Commit: commit})
		}
	}
}

func (pool *BlockPool) trimStaleState() {
	pool.mtx.Lock()
	defer pool.mtx.Unlock()
	trimHeight := pool.currentHeight() - maxCachedSealedBlock
	if trimHeight > 0 {
		if pubStates := pool.getPublishersAtHeight(trimHeight); pubStates != nil {
			for _, ps := range pubStates {
				if pool.st == Hot {
					ps.tryExpire()
				}
			}
		}
		delete(pool.blockStates, trimHeight)
	}
}

func (pool *BlockPool) getPublishersAtHeight(height int64) map[p2p.ID]*publisherState {
	if bs, exist := pool.blockStates[height]; exist {
		return bs.pubStates
	}
	return nil
}

func (pool *BlockPool) getPublisherAtHeight(height int64, pid p2p.ID) *publisherState {
	if pubs := pool.getPublishersAtHeight(height); pubs != nil {
		return pubs[pid]
	}
	return nil
}

//  where use currentHeight have already locked
func (pool *BlockPool) currentHeight() int64 {
	return pool.height
}

func (pool *BlockPool) incCurrentHeight() {
	pool.mtx.Lock()
	defer pool.mtx.Unlock()
	pool.height++
}

func (pool *BlockPool) setCurrentHeight(height int64) {
	pool.mtx.Lock()
	defer pool.mtx.Unlock()
	pool.height = height
}

// most of the usage have locked. Only newBlockStateAtHeight do not, but the usage can't
// currently excuted with incCurrentHeight, it is safe.
func (pool *BlockPool) expectingHeight() int64 {
	return pool.height + 1
}

func (pool *BlockPool) incBlocksSynced() int32 {
	return atomic.AddInt32(&pool.blocksSynced, 1)
}

func (pool *BlockPool) getBlockSynced() int32 {
	return atomic.LoadInt32(&pool.blocksSynced)
}

func (pool *BlockPool) getSyncPattern() SyncPattern {
	pool.mtx.Lock()
	defer pool.mtx.Unlock()
	return pool.st
}

func (pool *BlockPool) verifyCommit(blockID types.BlockID, commit *types.Commit) error {
	return pool.state.Validators.VerifyCommit(pool.state.ChainID, blockID, pool.currentHeight()+1, commit)
}

func (pool *BlockPool) sendMessages(pid p2p.ID, msgs ...BlockchainMessage) {
	for _, msg := range msgs {
		select {
		case pool.messagesCh <- Message{msg, pid}:
		default:
			pool.Logger.Error("Failed to send commit/blockChainMessage since messagesCh is full", "peer", pid)
		}
	}
}

func (pool *BlockPool) updatePeerMetrics(ps *publisherState) {
	var et eventType
	var dur int64
	if ps.broken {
		et = Bad
	} else {
		et = Good
		dur = time.Now().Sub(ps.bs.startTime).Nanoseconds()
		// this should not happened, but defend it.
		if dur <= 0 {
			dur = 1
		}
	}
	select {
	case ps.pool.sampleStream <- metricsEvent{et, ps.pid, dur}:
	default:
		ps.logger.Error("failed to send good sample event", "peer", ps.pid, "height", ps.bs.height)
	}
	return
}

func (pool *BlockPool) recordMetrics(block *types.Block) {
	height := block.Height
	if height > 1 {
		var lastBlockTime time.Time
		if lastBs, exist := pool.blockStates[height-1]; exist {
			lastBlockTime = lastBs.block.Time
		} else {
			lastBlockTime = pool.store.LoadBlockMeta(height - 1).Header.Time
		}
		pool.metrics.BlockIntervalSeconds.Set(
			block.Time.Sub(lastBlockTime).Seconds(),
		)
	}
	pool.metrics.NumTxs.Set(float64(block.NumTxs))
	pool.metrics.BlockSizeBytes.Set(float64(block.Size()))
	pool.metrics.TotalTxs.Set(float64(block.TotalTxs))
	pool.metrics.CommittedHeight.Set(float64(height))
	pool.metrics.Height.Set(float64(height + 1))
}

//----------------------------------------
// blockState track the response of multi peers about block/commit at specified
// height that blockPool subscribed.
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

func NewBlockState(height int64, logger log.Logger, pool *BlockPool) *blockState {
	bc := &blockState{
		height:    height,
		logger:    logger,
		pool:      pool,
		pubStates: make(map[p2p.ID]*publisherState, 0),
	}
	if pool.expectingHeight() == height {
		bc.startTime = time.Now()
	}
	return bc
}

func (bs *blockState) setLatestBlock(block *types.Block) {
	bs.mux.Lock()
	defer bs.mux.Unlock()
	if bs.sealed {
		return
	}
	if bs.latestBlock == nil || !bytes.Equal(bs.latestBlock.Hash(), block.Hash()) {
		bs.latestBlock = block
		// notice blockChainMessage state change
		bs.pool.publishBlock(bs.height, block)
	}
}

func (bs *blockState) setLatestCommit(commit *types.Commit) {
	bs.mux.Lock()
	defer bs.mux.Unlock()
	if bs.sealed {
		return
	}
	if bs.latestCommit == nil || !bytes.Equal(bs.latestCommit.Hash(), commit.Hash()) {
		bs.latestCommit = commit
		// notice blockChainMessage state change
		bs.pool.publishCommit(bs.height, commit)
	}
}

func (bs *blockState) seal() {
	bs.mux.Lock()
	defer bs.mux.Unlock()
	if bs.sealed == true {
		return
	}
	bs.sealed = true
	bs.logger.Debug("sealing blockChainMessage commit", "height", bs.height)
	bs.pool.blockStateSealCh <- bs
}

func (bs *blockState) isSealed() bool {
	bs.mux.Lock()
	defer bs.mux.Unlock()
	return bs.sealed
}

//----------------------------------------
// publisherState is used to track the continuous response of a peer about block/commit at specified
// height that blockpool subscribed. The peer plays as a publisher.
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

func NewPeerPublisherState(bc *blockState, pid p2p.ID, logger log.Logger, timeoutDur time.Duration, pool *BlockPool) *publisherState {
	ps := &publisherState{
		logger:     logger,
		pid:        pid,
		bs:         bc,
		pool:       pool,
		timeoutDur: timeoutDur,
	}
	if pool.expectingHeight() == bc.height || pid == selfId {
		if pool.st == Hot {
			ps.startTimer()
		}
		ps.isWake = true
	}
	return ps
}

func (ps *publisherState) wakeup() {
	ps.mux.Lock()
	defer ps.mux.Unlock()
	ps.startTimer()
	ps.isWake = true
	ps.trySeal()
}

// if the ps still not trig timeout, but is going to trim,
// consider it as bad peer, and update metrics by itself.
func (ps *publisherState) tryExpire() {
	ps.mux.Lock()
	defer ps.mux.Unlock()
	if !ps.sealed {
		// stop the timer in case it leaks
		if ps.timeout != nil {
			ps.timeout.Stop()
		}
		ps.broken = true
		ps.pool.updatePeerMetrics(ps)
	}
}

func (ps *publisherState) onTimeout() {
	ps.mux.Lock()
	defer ps.mux.Unlock()
	if ps.sealed {
		return
	}
	ps.broken = true
	ps.seal()
}

func (ps *publisherState) receiveBlock(block *types.Block) {
	ps.mux.Lock()
	defer ps.mux.Unlock()
	if ps.sealed {
		ps.logger.Debug("received blockChainMessage that is already sealed", "peer", ps.pid, "height", ps.bs.height)
		return
	}
	ps.block = block
	ps.bs.setLatestBlock(block)
	ps.trySeal()
}

func (ps *publisherState) receiveNoBlock() {
	ps.mux.Lock()
	defer ps.mux.Unlock()
	if ps.sealed {
		return
	}
	ps.broken = true
	ps.seal()
}

func (ps *publisherState) receiveCommit(commit *types.Commit) {
	ps.mux.Lock()
	defer ps.mux.Unlock()
	if ps.sealed {
		ps.logger.Debug("received commit that is already sealed", "peer", ps.pid, "height", ps.bs.height)
		return
	}
	ps.commit = commit
	ps.bs.setLatestCommit(commit)
	ps.trySeal()
}

func (ps *publisherState) trySeal() {
	if ps.bs.isSealed() {
		ps.tryLaterSeal()
	} else {
		ps.tryFirstSeal()
	}
}

func (ps *publisherState) tryLaterSeal() {
	if ps.block == nil || ps.commit == nil {
		return
	}
	blockId := makeBlockID(ps.block)
	// For a later sealed request, just choose to compare the the blockChainMessage id to
	// decide whether it is a good peer. Fully verification is costly and
	// no strong need to do so since we only used to judge if the peer is good or not.
	if blockId.Equals(ps.commit.BlockID) && blockId.Equals(*ps.bs.blockId) {
		ps.seal()
		return
	} else {
		// maybe blockChainMessage is late, won't seal.
		ps.logger.Info("received inconsistent blockChainMessage/commit", "peer", ps.pid, "height", ps.bs.height)
		return
	}
}

func (ps *publisherState) tryFirstSeal() {
	if ps.block == nil || ps.commit == nil {
		return
	}
	// not now
	if !ps.isWake {
		return
	}
	blockId := makeBlockID(ps.block)
	if ps.pid == selfId {
		if !blockId.Equals(ps.commit.BlockID) {
			return
		}
	} else if err := ps.pool.verifyCommit(*blockId, ps.commit); err != nil {
		// maybe blockChainMessage is late, won't seal.
		ps.logger.Info("received inconsistent blockChainMessage/commit", "peer", ps.pid, ps.bs.height)
		return
	}
	ps.bs.commit = ps.commit
	ps.bs.block = ps.block
	ps.bs.blockId = blockId
	ps.seal()
	ps.bs.seal()
	return
}

// only start timer when blockpool reach at this height.
func (ps *publisherState) startTimer() {
	if ps.timeout != nil {
		ps.timeout.Reset(ps.timeoutDur)
	} else {
		ps.timeout = time.AfterFunc(ps.timeoutDur, ps.onTimeout)
	}
}

func (ps *publisherState) seal() {
	if ps.timeout != nil {
		ps.timeout.Stop()
	}
	ps.sealed = true
	ps.logger.Debug("publisher state sealing", "peer", ps.pid, "height", ps.bs.height)
	ps.pool.publisherStateSealCh <- ps
}

// -----------------------
type Message struct {
	blockChainMessage BlockchainMessage
	peerId            p2p.ID
}

type decayedPeer struct {
	peerId     p2p.ID
	fromHeight int64
}

func makeBlockID(block *types.Block) *types.BlockID {
	blockParts := block.MakePartSet(types.BlockPartSizeBytes)
	blockPartsHeader := blockParts.Header()
	return &types.BlockID{Hash: block.Hash(), PartsHeader: blockPartsHeader}
}
