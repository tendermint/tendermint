package v0

import (
	"fmt"
	"runtime/debug"
	"sync"
	"time"

	"github.com/tendermint/tendermint/internal/blocksync"
	"github.com/tendermint/tendermint/internal/consensus"
	"github.com/tendermint/tendermint/internal/p2p"
	sm "github.com/tendermint/tendermint/internal/state"
	"github.com/tendermint/tendermint/internal/store"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/libs/service"
	tmsync "github.com/tendermint/tendermint/libs/sync"
	bcproto "github.com/tendermint/tendermint/proto/tendermint/blocksync"
	"github.com/tendermint/tendermint/types"
)

var (
	_ service.Service = (*Reactor)(nil)

	// ChannelShims contains a map of ChannelDescriptorShim objects, where each
	// object wraps a reference to a legacy p2p ChannelDescriptor and the corresponding
	// p2p proto.Message the new p2p Channel is responsible for handling.
	//
	//
	// TODO: Remove once p2p refactor is complete.
	// ref: https://github.com/tendermint/tendermint/issues/5670
	ChannelShims = map[p2p.ChannelID]*p2p.ChannelDescriptorShim{
		BlockSyncChannel: {
			MsgType: new(bcproto.Message),
			Descriptor: &p2p.ChannelDescriptor{
				ID:                  byte(BlockSyncChannel),
				Priority:            5,
				SendQueueCapacity:   1000,
				RecvBufferCapacity:  1024,
				RecvMessageCapacity: blocksync.MaxMsgSize,
				MaxSendBytes:        100,
			},
		},
	}
)

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

type consensusReactor interface {
	// For when we switch from block sync reactor to the consensus
	// machine.
	SwitchToConsensus(state sm.State, skipWAL bool)
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

	// immutable
	initialState sm.State

	blockExec   *sm.BlockExecutor
	store       *store.BlockStore
	pool        *BlockPool
	consReactor consensusReactor
	blockSync   *tmsync.AtomicBool

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
	closeCh              chan struct{}

	requestsCh <-chan BlockRequest
	errorsCh   <-chan peerError

	// poolWG is used to synchronize the graceful shutdown of the poolRoutine and
	// requestRoutine spawned goroutines when stopping the reactor and before
	// stopping the p2p Channel(s).
	poolWG sync.WaitGroup

	metrics *consensus.Metrics

	syncStartTime time.Time
}

// NewReactor returns new reactor instance.
func NewReactor(
	logger log.Logger,
	state sm.State,
	blockExec *sm.BlockExecutor,
	store *store.BlockStore,
	consReactor consensusReactor,
	blockSyncCh *p2p.Channel,
	peerUpdates *p2p.PeerUpdates,
	blockSync bool,
	metrics *consensus.Metrics,
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

	r := &Reactor{
		initialState:         state,
		blockExec:            blockExec,
		store:                store,
		pool:                 NewBlockPool(startHeight, requestsCh, errorsCh),
		consReactor:          consReactor,
		blockSync:            tmsync.NewBool(blockSync),
		requestsCh:           requestsCh,
		errorsCh:             errorsCh,
		blockSyncCh:          blockSyncCh,
		blockSyncOutBridgeCh: make(chan p2p.Envelope),
		peerUpdates:          peerUpdates,
		closeCh:              make(chan struct{}),
		metrics:              metrics,
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
func (r *Reactor) OnStart() error {
	if r.blockSync.IsSet() {
		if err := r.pool.Start(); err != nil {
			return err
		}
		r.poolWG.Add(1)
		go r.requestRoutine()

		r.poolWG.Add(1)
		go r.poolRoutine(false)
	}

	go r.processBlockSyncCh()
	go r.processPeerUpdates()

	return nil
}

// OnStop stops the reactor by signaling to all spawned goroutines to exit and
// blocking until they all exit.
func (r *Reactor) OnStop() {
	if r.blockSync.IsSet() {
		if err := r.pool.Stop(); err != nil {
			r.Logger.Error("failed to stop pool", "err", err)
		}
	}

	// wait for the poolRoutine and requestRoutine goroutines to gracefully exit
	r.poolWG.Wait()

	// Close closeCh to signal to all spawned goroutines to gracefully exit. All
	// p2p Channels should execute Close().
	close(r.closeCh)

	// Wait for all p2p Channels to be closed before returning. This ensures we
	// can easily reason about synchronization of all p2p Channels and ensure no
	// panics will occur.
	<-r.blockSyncCh.Done()
	<-r.peerUpdates.Done()
}

// respondToPeer loads a block and sends it to the requesting peer, if we have it.
// Otherwise, we'll respond saying we do not have it.
func (r *Reactor) respondToPeer(msg *bcproto.BlockRequest, peerID types.NodeID) {
	block := r.store.LoadBlock(msg.Height)
	if block != nil {
		blockProto, err := block.ToProto()
		if err != nil {
			r.Logger.Error("failed to convert msg to protobuf", "err", err)
			return
		}

		r.blockSyncCh.Out <- p2p.Envelope{
			To:      peerID,
			Message: &bcproto.BlockResponse{Block: blockProto},
		}

		return
	}

	r.Logger.Info("peer requesting a block we do not have", "peer", peerID, "height", msg.Height)
	r.blockSyncCh.Out <- p2p.Envelope{
		To:      peerID,
		Message: &bcproto.NoBlockResponse{Height: msg.Height},
	}
}

// handleBlockSyncMessage handles envelopes sent from peers on the
// BlockSyncChannel. It returns an error only if the Envelope.Message is unknown
// for this channel. This should never be called outside of handleMessage.
func (r *Reactor) handleBlockSyncMessage(envelope p2p.Envelope) error {
	logger := r.Logger.With("peer", envelope.From)

	switch msg := envelope.Message.(type) {
	case *bcproto.BlockRequest:
		r.respondToPeer(msg, envelope.From)

	case *bcproto.BlockResponse:
		block, err := types.BlockFromProto(msg.Block)
		if err != nil {
			logger.Error("failed to convert block from proto", "err", err)
			return err
		}

		r.pool.AddBlock(envelope.From, block, block.Size())

	case *bcproto.StatusRequest:
		r.blockSyncCh.Out <- p2p.Envelope{
			To: envelope.From,
			Message: &bcproto.StatusResponse{
				Height: r.store.Height(),
				Base:   r.store.Base(),
			},
		}

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
func (r *Reactor) handleMessage(chID p2p.ChannelID, envelope p2p.Envelope) (err error) {
	defer func() {
		if e := recover(); e != nil {
			err = fmt.Errorf("panic in processing message: %v", e)
			r.Logger.Error(
				"recovering from processing message panic",
				"err", err,
				"stack", string(debug.Stack()),
			)
		}
	}()

	r.Logger.Debug("received message", "message", envelope.Message, "peer", envelope.From)

	switch chID {
	case BlockSyncChannel:
		err = r.handleBlockSyncMessage(envelope)

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
func (r *Reactor) processBlockSyncCh() {
	defer r.blockSyncCh.Close()

	for {
		select {
		case envelope := <-r.blockSyncCh.In:
			if err := r.handleMessage(r.blockSyncCh.ID, envelope); err != nil {
				r.Logger.Error("failed to process message", "ch_id", r.blockSyncCh.ID, "envelope", envelope, "err", err)
				r.blockSyncCh.Error <- p2p.PeerError{
					NodeID: envelope.From,
					Err:    err,
				}
			}

		case envelope := <-r.blockSyncOutBridgeCh:
			r.blockSyncCh.Out <- envelope

		case <-r.closeCh:
			r.Logger.Debug("stopped listening on block sync channel; closing...")
			return

		}
	}
}

// processPeerUpdate processes a PeerUpdate.
func (r *Reactor) processPeerUpdate(peerUpdate p2p.PeerUpdate) {
	r.Logger.Debug("received peer update", "peer", peerUpdate.NodeID, "status", peerUpdate.Status)

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
func (r *Reactor) processPeerUpdates() {
	defer r.peerUpdates.Close()

	for {
		select {
		case peerUpdate := <-r.peerUpdates.Updates():
			r.processPeerUpdate(peerUpdate)

		case <-r.closeCh:
			r.Logger.Debug("stopped listening on peer updates channel; closing...")
			return
		}
	}
}

// SwitchToBlockSync is called by the state sync reactor when switching to fast
// sync.
func (r *Reactor) SwitchToBlockSync(state sm.State) error {
	r.blockSync.Set()
	r.initialState = state
	r.pool.height = state.LastBlockHeight + 1

	if err := r.pool.Start(); err != nil {
		return err
	}

	r.syncStartTime = time.Now()

	r.poolWG.Add(1)
	go r.requestRoutine()

	r.poolWG.Add(1)
	go r.poolRoutine(true)

	return nil
}

func (r *Reactor) requestRoutine() {
	statusUpdateTicker := time.NewTicker(statusUpdateIntervalSeconds * time.Second)
	defer statusUpdateTicker.Stop()

	defer r.poolWG.Done()

	for {
		select {
		case <-r.closeCh:
			return

		case <-r.pool.Quit():
			return

		case request := <-r.requestsCh:
			r.blockSyncOutBridgeCh <- p2p.Envelope{
				To:      request.PeerID,
				Message: &bcproto.BlockRequest{Height: request.Height},
			}

		case pErr := <-r.errorsCh:
			r.blockSyncCh.Error <- p2p.PeerError{
				NodeID: pErr.peerID,
				Err:    pErr.err,
			}

		case <-statusUpdateTicker.C:
			r.poolWG.Add(1)

			go func() {
				defer r.poolWG.Done()

				r.blockSyncOutBridgeCh <- p2p.Envelope{
					Broadcast: true,
					Message:   &bcproto.StatusRequest{},
				}
			}()
		}
	}
}

// poolRoutine handles messages from the poolReactor telling the reactor what to
// do.
//
// NOTE: Don't sleep in the FOR_LOOP or otherwise slow it down!
func (r *Reactor) poolRoutine(stateSynced bool) {
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

	defer r.poolWG.Done()

FOR_LOOP:
	for {
		select {
		case <-switchToConsensusTicker.C:
			var (
				height, numPending, lenRequesters = r.pool.GetStatus()
				lastAdvance                       = r.pool.LastAdvance()
			)

			r.Logger.Debug(
				"consensus ticker",
				"num_pending", numPending,
				"total", lenRequesters,
				"height", height,
			)

			switch {
			case r.pool.IsCaughtUp():
				r.Logger.Info("switching to consensus reactor", "height", height)

			case time.Since(lastAdvance) > syncTimeout:
				r.Logger.Error("no progress since last advance", "last_advance", lastAdvance)

			default:
				r.Logger.Info(
					"not caught up yet",
					"height", height,
					"max_peer_height", r.pool.MaxPeerHeight(),
					"timeout_in", syncTimeout-time.Since(lastAdvance),
				)
				continue
			}

			if err := r.pool.Stop(); err != nil {
				r.Logger.Error("failed to stop pool", "err", err)
			}

			r.blockSync.UnSet()

			if r.consReactor != nil {
				r.consReactor.SwitchToConsensus(state, blocksSynced > 0 || stateSynced)
			}

			break FOR_LOOP

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
				continue FOR_LOOP
			} else {
				// try again quickly next loop
				didProcessCh <- struct{}{}
			}

			var (
				firstParts         = first.MakePartSet(types.BlockPartSizeBytes)
				firstPartSetHeader = firstParts.Header()
				firstID            = types.BlockID{Hash: first.Hash(), PartSetHeader: firstPartSetHeader}
			)

			// Finally, verify the first block using the second's commit.
			//
			// NOTE: We can probably make this more efficient, but note that calling
			// first.Hash() doesn't verify the tx contents, so MakePartSet() is
			// currently necessary.
			err := state.Validators.VerifyCommitLight(chainID, firstID, first.Height, second.LastCommit)
			if err != nil {
				err = fmt.Errorf("invalid last commit: %w", err)
				r.Logger.Error(
					err.Error(),
					"last_commit", second.LastCommit,
					"block_id", firstID,
					"height", first.Height,
				)

				// NOTE: We've already removed the peer's request, but we still need
				// to clean up the rest.
				peerID := r.pool.RedoRequest(first.Height)
				r.blockSyncCh.Error <- p2p.PeerError{
					NodeID: peerID,
					Err:    err,
				}

				peerID2 := r.pool.RedoRequest(second.Height)
				if peerID2 != peerID {
					r.blockSyncCh.Error <- p2p.PeerError{
						NodeID: peerID2,
						Err:    err,
					}
				}

				continue FOR_LOOP
			} else {
				r.pool.PopRequest()

				// TODO: batch saves so we do not persist to disk every block
				r.store.SaveBlock(first, firstParts, second.LastCommit)

				var err error

				// TODO: Same thing for app - but we would need a way to get the hash
				// without persisting the state.
				state, err = r.blockExec.ApplyBlock(state, firstID, first)
				if err != nil {
					// TODO: This is bad, are we zombie?
					panic(fmt.Sprintf("failed to process committed block (%d:%X): %v", first.Height, first.Hash(), err))
				}

				r.metrics.RecordConsMetrics(first)

				blocksSynced++

				if blocksSynced%100 == 0 {
					lastRate = 0.9*lastRate + 0.1*(100/time.Since(lastHundred).Seconds())
					r.Logger.Info(
						"block sync rate",
						"height", r.pool.height,
						"max_peer_height", r.pool.MaxPeerHeight(),
						"blocks/s", lastRate,
					)

					lastHundred = time.Now()
				}
			}

			continue FOR_LOOP

		case <-r.closeCh:
			break FOR_LOOP
		case <-r.pool.Quit():
			break FOR_LOOP
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
