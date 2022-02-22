package blocksync

import (
	"context"
	"errors"
	"fmt"
	"runtime/debug"
	"sync/atomic"
	"time"

	"github.com/tendermint/tendermint/internal/consensus"
	"github.com/tendermint/tendermint/internal/eventbus"
	"github.com/tendermint/tendermint/internal/p2p"
	sm "github.com/tendermint/tendermint/internal/state"
	"github.com/tendermint/tendermint/internal/store"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/libs/service"
	bcproto "github.com/tendermint/tendermint/proto/tendermint/blocksync"
	"github.com/tendermint/tendermint/types"
)

var _ service.Service = (*Reactor)(nil)

const (
	// BlockSyncChannel is a channel for blocks and status updates
	BlockSyncChannel = p2p.ChannelID(0x40)

	trySyncIntervalMS = 10

	// ask for best height every 10s
	statusUpdateIntervalSeconds = 10

	// check if we should switch to consensus reactor
	switchToConsensusIntervalSeconds = 1

	// switch to consensus after this duration of inactivity
	syncTimeout = 60 * time.Second
)

func GetChannelDescriptor() *p2p.ChannelDescriptor {
	return &p2p.ChannelDescriptor{
		ID:                  BlockSyncChannel,
		MessageType:         new(bcproto.Message),
		Priority:            5,
		SendQueueCapacity:   1000,
		RecvBufferCapacity:  1024,
		RecvMessageCapacity: MaxMsgSize,
	}
}

type consensusReactor interface {
	// For when we switch from block sync reactor to the consensus
	// machine.
	SwitchToConsensus(ctx context.Context, state sm.State, skipWAL bool)
}

type peerError struct {
	err    error
	peerID types.NodeID
}

func (e peerError) Error() string {
	return fmt.Sprintf("error with peer %v: %s", e.peerID, e.err.Error())
}

// Reactor handles long-term catchup syncing.
type Reactor struct {
	service.BaseService
	logger log.Logger

	// immutable
	initialState sm.State

	blockExec   *sm.BlockExecutor
	store       *store.BlockStore
	pool        *BlockPool
	consReactor consensusReactor
	blockSync   *atomicBool

	blockSyncCh *p2p.Channel
	// blockSyncOutBridgeCh defines a channel that acts as a bridge between sending Envelope
	// messages that the reactor will consume in processBlockSyncCh and receiving messages
	// from the peer updates channel and other goroutines. We do this instead of directly
	// sending on blockSyncCh.Out to avoid race conditions in the case where other goroutines
	// send Envelopes directly to the to blockSyncCh.Out channel, since processBlockSyncCh
	// may close the blockSyncCh.Out channel at the same time that other goroutines send to
	// blockSyncCh.Out.
	blockSyncOutBridgeCh chan p2p.Envelope
	peerUpdates          *p2p.PeerUpdates

	requestsCh <-chan BlockRequest
	errorsCh   <-chan peerError

	metrics  *consensus.Metrics
	eventBus *eventbus.EventBus

	syncStartTime time.Time
}

// NewReactor returns new reactor instance.
func NewReactor(
	ctx context.Context,
	logger log.Logger,
	state sm.State,
	blockExec *sm.BlockExecutor,
	store *store.BlockStore,
	consReactor consensusReactor,
	channelCreator p2p.ChannelCreator,
	peerUpdates *p2p.PeerUpdates,
	blockSync bool,
	metrics *consensus.Metrics,
	eventBus *eventbus.EventBus,
) (*Reactor, error) {

	if state.LastBlockHeight != store.Height() {
		return nil, fmt.Errorf("state (%v) and store (%v) height mismatch", state.LastBlockHeight, store.Height())
	}

	startHeight := store.Height() + 1
	if startHeight == 1 {
		startHeight = state.InitialHeight
	}

	requestsCh := make(chan BlockRequest, maxTotalRequesters)
	errorsCh := make(chan peerError, maxPeerErrBuffer) // NOTE: The capacity should be larger than the peer count.

	blockSyncCh, err := channelCreator(ctx, GetChannelDescriptor())
	if err != nil {
		return nil, err
	}

	r := &Reactor{
		logger:               logger,
		initialState:         state,
		blockExec:            blockExec,
		store:                store,
		pool:                 NewBlockPool(logger, startHeight, requestsCh, errorsCh),
		consReactor:          consReactor,
		blockSync:            newAtomicBool(blockSync),
		requestsCh:           requestsCh,
		errorsCh:             errorsCh,
		blockSyncCh:          blockSyncCh,
		blockSyncOutBridgeCh: make(chan p2p.Envelope),
		peerUpdates:          peerUpdates,
		metrics:              metrics,
		eventBus:             eventBus,
		syncStartTime:        time.Time{},
	}

	r.BaseService = *service.NewBaseService(logger, "BlockSync", r)
	return r, nil
}

// OnStart starts separate go routines for each p2p Channel and listens for
// envelopes on each. In addition, it also listens for peer updates and handles
// messages on that p2p channel accordingly. The caller must be sure to execute
// OnStop to ensure the outbound p2p Channels are closed.
//
// If blockSync is enabled, we also start the pool and the pool processing
// goroutine. If the pool fails to start, an error is returned.
func (r *Reactor) OnStart(ctx context.Context) error {
	if r.blockSync.IsSet() {
		if err := r.pool.Start(ctx); err != nil {
			return err
		}
		go r.requestRoutine(ctx)

		go r.poolRoutine(ctx, false)
	}

	go r.processBlockSyncCh(ctx)
	go r.processBlockSyncBridge(ctx)
	go r.processPeerUpdates(ctx)

	return nil
}

// OnStop stops the reactor by signaling to all spawned goroutines to exit and
// blocking until they all exit.
func (r *Reactor) OnStop() {
	if r.blockSync.IsSet() {
		r.pool.Stop()
	}
}

// respondToPeer loads a block and sends it to the requesting peer, if we have it.
// Otherwise, we'll respond saying we do not have it.
func (r *Reactor) respondToPeer(ctx context.Context, msg *bcproto.BlockRequest, peerID types.NodeID) error {
	block := r.store.LoadBlock(msg.Height)
	if block != nil {
		blockProto, err := block.ToProto()
		if err != nil {
			r.logger.Error("failed to convert msg to protobuf", "err", err)
			return err
		}

		return r.blockSyncCh.Send(ctx, p2p.Envelope{
			To:      peerID,
			Message: &bcproto.BlockResponse{Block: blockProto},
		})
	}

	r.logger.Info("peer requesting a block we do not have", "peer", peerID, "height", msg.Height)

	return r.blockSyncCh.Send(ctx, p2p.Envelope{
		To:      peerID,
		Message: &bcproto.NoBlockResponse{Height: msg.Height},
	})
}

// handleBlockSyncMessage handles envelopes sent from peers on the
// BlockSyncChannel. It returns an error only if the Envelope.Message is unknown
// for this channel. This should never be called outside of handleMessage.
func (r *Reactor) handleBlockSyncMessage(ctx context.Context, envelope *p2p.Envelope) error {
	logger := r.logger.With("peer", envelope.From)

	switch msg := envelope.Message.(type) {
	case *bcproto.BlockRequest:
		return r.respondToPeer(ctx, msg, envelope.From)
	case *bcproto.BlockResponse:
		block, err := types.BlockFromProto(msg.Block)
		if err != nil {
			logger.Error("failed to convert block from proto", "err", err)
			return err
		}

		r.pool.AddBlock(envelope.From, block, block.Size())

	case *bcproto.StatusRequest:
		return r.blockSyncCh.Send(ctx, p2p.Envelope{
			To: envelope.From,
			Message: &bcproto.StatusResponse{
				Height: r.store.Height(),
				Base:   r.store.Base(),
			},
		})
	case *bcproto.StatusResponse:
		r.pool.SetPeerRange(envelope.From, msg.Base, msg.Height)

	case *bcproto.NoBlockResponse:
		logger.Debug("peer does not have the requested block", "height", msg.Height)

	default:
		return fmt.Errorf("received unknown message: %T", msg)
	}

	return nil
}

// handleMessage handles an Envelope sent from a peer on a specific p2p Channel.
// It will handle errors and any possible panics gracefully. A caller can handle
// any error returned by sending a PeerError on the respective channel.
func (r *Reactor) handleMessage(ctx context.Context, chID p2p.ChannelID, envelope *p2p.Envelope) (err error) {
	defer func() {
		if e := recover(); e != nil {
			err = fmt.Errorf("panic in processing message: %v", e)
			r.logger.Error(
				"recovering from processing message panic",
				"err", err,
				"stack", string(debug.Stack()),
			)
		}
	}()

	r.logger.Debug("received message", "message", envelope.Message, "peer", envelope.From)

	switch chID {
	case BlockSyncChannel:
		err = r.handleBlockSyncMessage(ctx, envelope)

	default:
		err = fmt.Errorf("unknown channel ID (%d) for envelope (%v)", chID, envelope)
	}

	return err
}

// processBlockSyncCh initiates a blocking process where we listen for and handle
// envelopes on the BlockSyncChannel and blockSyncOutBridgeCh. Any error encountered during
// message execution will result in a PeerError being sent on the BlockSyncChannel.
// When the reactor is stopped, we will catch the signal and close the p2p Channel
// gracefully.
func (r *Reactor) processBlockSyncCh(ctx context.Context) {
	iter := r.blockSyncCh.Receive(ctx)
	for iter.Next(ctx) {
		envelope := iter.Envelope()
		if err := r.handleMessage(ctx, r.blockSyncCh.ID, envelope); err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return
			}

			r.logger.Error("failed to process message", "ch_id", r.blockSyncCh.ID, "envelope", envelope, "err", err)
			if serr := r.blockSyncCh.SendError(ctx, p2p.PeerError{
				NodeID: envelope.From,
				Err:    err,
			}); serr != nil {
				return
			}
		}
	}
}

func (r *Reactor) processBlockSyncBridge(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case envelope := <-r.blockSyncOutBridgeCh:
			if err := r.blockSyncCh.Send(ctx, envelope); err != nil {
				return
			}
		}
	}
}

// processPeerUpdate processes a PeerUpdate.
func (r *Reactor) processPeerUpdate(peerUpdate p2p.PeerUpdate) {
	r.logger.Debug("received peer update", "peer", peerUpdate.NodeID, "status", peerUpdate.Status)

	// XXX: Pool#RedoRequest can sometimes give us an empty peer.
	if len(peerUpdate.NodeID) == 0 {
		return
	}

	switch peerUpdate.Status {
	case p2p.PeerStatusUp:
		// send a status update the newly added peer
		r.blockSyncOutBridgeCh <- p2p.Envelope{
			To: peerUpdate.NodeID,
			Message: &bcproto.StatusResponse{
				Base:   r.store.Base(),
				Height: r.store.Height(),
			},
		}

	case p2p.PeerStatusDown:
		r.pool.RemovePeer(peerUpdate.NodeID)
	}
}

// processPeerUpdates initiates a blocking process where we listen for and handle
// PeerUpdate messages. When the reactor is stopped, we will catch the signal and
// close the p2p PeerUpdatesCh gracefully.
func (r *Reactor) processPeerUpdates(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case peerUpdate := <-r.peerUpdates.Updates():
			r.processPeerUpdate(peerUpdate)
		}
	}
}

// SwitchToBlockSync is called by the state sync reactor when switching to fast
// sync.
func (r *Reactor) SwitchToBlockSync(ctx context.Context, state sm.State) error {
	r.blockSync.Set()
	r.initialState = state
	r.pool.height = state.LastBlockHeight + 1

	if err := r.pool.Start(ctx); err != nil {
		return err
	}

	r.syncStartTime = time.Now()

	go r.requestRoutine(ctx)

	go r.poolRoutine(ctx, true)

	return nil
}

func (r *Reactor) requestRoutine(ctx context.Context) {
	statusUpdateTicker := time.NewTicker(statusUpdateIntervalSeconds * time.Second)
	defer statusUpdateTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case request := <-r.requestsCh:
			select {
			case <-ctx.Done():
				return
			case r.blockSyncOutBridgeCh <- p2p.Envelope{
				To:      request.PeerID,
				Message: &bcproto.BlockRequest{Height: request.Height},
			}:
			}

		case pErr := <-r.errorsCh:
			if err := r.blockSyncCh.SendError(ctx, p2p.PeerError{
				NodeID: pErr.peerID,
				Err:    pErr.err,
			}); err != nil {
				return
			}
		case <-statusUpdateTicker.C:
			go func() {
				select {
				case <-ctx.Done():
					return
				case r.blockSyncOutBridgeCh <- p2p.Envelope{
					Broadcast: true,
					Message:   &bcproto.StatusRequest{},
				}:
				}
			}()
		}
	}
}

// poolRoutine handles messages from the poolReactor telling the reactor what to
// do.
//
// NOTE: Don't sleep in the FOR_LOOP or otherwise slow it down!
func (r *Reactor) poolRoutine(ctx context.Context, stateSynced bool) {
	var (
		trySyncTicker           = time.NewTicker(trySyncIntervalMS * time.Millisecond)
		switchToConsensusTicker = time.NewTicker(switchToConsensusIntervalSeconds * time.Second)

		blocksSynced = uint64(0)

		chainID = r.initialState.ChainID
		state   = r.initialState

		lastHundred = time.Now()
		lastRate    = 0.0

		didProcessCh = make(chan struct{}, 1)
	)

	defer trySyncTicker.Stop()
	defer switchToConsensusTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-r.pool.exitedCh:
			return
		case <-switchToConsensusTicker.C:
			var (
				height, numPending, lenRequesters = r.pool.GetStatus()
				lastAdvance                       = r.pool.LastAdvance()
			)

			r.logger.Debug(
				"consensus ticker",
				"num_pending", numPending,
				"total", lenRequesters,
				"height", height,
			)

			switch {
			case r.pool.IsCaughtUp():
				r.logger.Info("switching to consensus reactor", "height", height)

			case time.Since(lastAdvance) > syncTimeout:
				r.logger.Error("no progress since last advance", "last_advance", lastAdvance)

			default:
				r.logger.Info(
					"not caught up yet",
					"height", height,
					"max_peer_height", r.pool.MaxPeerHeight(),
					"timeout_in", syncTimeout-time.Since(lastAdvance),
				)
				continue
			}

			r.pool.Stop()

			r.blockSync.UnSet()

			if r.consReactor != nil {
				r.consReactor.SwitchToConsensus(ctx, state, blocksSynced > 0 || stateSynced)
			}

			return

		case <-trySyncTicker.C:
			select {
			case didProcessCh <- struct{}{}:
			default:
			}
		case <-didProcessCh:
			// NOTE: It is a subtle mistake to process more than a single block at a
			// time (e.g. 10) here, because we only send one BlockRequest per loop
			// iteration. The ratio mismatch can result in starving of blocks, i.e. a
			// sudden burst of requests and responses, and repeat. Consequently, it is
			// better to split these routines rather than coupling them as it is
			// written here.
			//
			// TODO: Uncouple from request routine.

			// see if there are any blocks to sync
			first, second := r.pool.PeekTwoBlocks()
			if first == nil || second == nil {
				// we need both to sync the first block
				continue
			} else {
				// try again quickly next loop
				didProcessCh <- struct{}{}
			}

			firstParts, err := first.MakePartSet(types.BlockPartSizeBytes)
			if err != nil {
				r.logger.Error("failed to make ",
					"height", first.Height,
					"err", err.Error())
				return
			}

			var (
				firstPartSetHeader = firstParts.Header()
				firstID            = types.BlockID{Hash: first.Hash(), PartSetHeader: firstPartSetHeader}
			)

			// Finally, verify the first block using the second's commit.
			//
			// NOTE: We can probably make this more efficient, but note that calling
			// first.Hash() doesn't verify the tx contents, so MakePartSet() is
			// currently necessary.
			if err = state.Validators.VerifyCommitLight(chainID, firstID, first.Height, second.LastCommit); err != nil {
				err = fmt.Errorf("invalid last commit: %w", err)
				r.logger.Error(
					err.Error(),
					"last_commit", second.LastCommit,
					"block_id", firstID,
					"height", first.Height,
				)

				// NOTE: We've already removed the peer's request, but we still need
				// to clean up the rest.
				peerID := r.pool.RedoRequest(first.Height)
				if serr := r.blockSyncCh.SendError(ctx, p2p.PeerError{
					NodeID: peerID,
					Err:    err,
				}); serr != nil {
					return
				}

				peerID2 := r.pool.RedoRequest(second.Height)
				if peerID2 != peerID {
					if serr := r.blockSyncCh.SendError(ctx, p2p.PeerError{
						NodeID: peerID2,
						Err:    err,
					}); serr != nil {
						return
					}
				}
			} else {
				r.pool.PopRequest()

				// TODO: batch saves so we do not persist to disk every block
				r.store.SaveBlock(first, firstParts, second.LastCommit)

				var err error

				// TODO: Same thing for app - but we would need a way to get the hash
				// without persisting the state.
				state, err = r.blockExec.ApplyBlock(ctx, state, firstID, first)
				if err != nil {
					// TODO: This is bad, are we zombie?
					panic(fmt.Sprintf("failed to process committed block (%d:%X): %v", first.Height, first.Hash(), err))
				}

				r.metrics.RecordConsMetrics(first)

				blocksSynced++

				if blocksSynced%100 == 0 {
					lastRate = 0.9*lastRate + 0.1*(100/time.Since(lastHundred).Seconds())
					r.logger.Info(
						"block sync rate",
						"height", r.pool.height,
						"max_peer_height", r.pool.MaxPeerHeight(),
						"blocks/s", lastRate,
					)

					lastHundred = time.Now()
				}
			}
		}
	}
}

func (r *Reactor) GetMaxPeerBlockHeight() int64 {
	return r.pool.MaxPeerHeight()
}

func (r *Reactor) GetTotalSyncedTime() time.Duration {
	if !r.blockSync.IsSet() || r.syncStartTime.IsZero() {
		return time.Duration(0)
	}
	return time.Since(r.syncStartTime)
}

func (r *Reactor) GetRemainingSyncTime() time.Duration {
	if !r.blockSync.IsSet() {
		return time.Duration(0)
	}

	targetSyncs := r.pool.targetSyncBlocks()
	currentSyncs := r.store.Height() - r.pool.startHeight + 1
	lastSyncRate := r.pool.getLastSyncRate()
	if currentSyncs < 0 || lastSyncRate < 0.001 {
		return time.Duration(0)
	}

	remain := float64(targetSyncs-currentSyncs) / lastSyncRate

	return time.Duration(int64(remain * float64(time.Second)))
}

func (r *Reactor) PublishStatus(ctx context.Context, event types.EventDataBlockSyncStatus) error {
	if r.eventBus == nil {
		return errors.New("event bus is not configured")
	}
	return r.eventBus.PublishEventBlockSyncStatus(ctx, event)
}

// atomicBool is an atomic Boolean, safe for concurrent use by multiple
// goroutines.
type atomicBool int32

// newAtomicBool creates an atomicBool with given initial value.
func newAtomicBool(ok bool) *atomicBool {
	ab := new(atomicBool)
	if ok {
		ab.Set()
	}
	return ab
}

// Set sets the Boolean to true.
func (ab *atomicBool) Set() { atomic.StoreInt32((*int32)(ab), 1) }

// UnSet sets the Boolean to false.
func (ab *atomicBool) UnSet() { atomic.StoreInt32((*int32)(ab), 0) }

// IsSet returns whether the Boolean is true.
func (ab *atomicBool) IsSet() bool { return atomic.LoadInt32((*int32)(ab))&1 == 1 }
