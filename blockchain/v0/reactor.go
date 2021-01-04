package v0

import (
	"fmt"
	"time"

	"github.com/gogo/protobuf/proto"
	bc "github.com/tendermint/tendermint/blockchain"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/libs/service"
	"github.com/tendermint/tendermint/p2p"
	bcproto "github.com/tendermint/tendermint/proto/tendermint/blockchain"
	sm "github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/store"
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
		BlockchainChannel: {
			MsgType: new(bcproto.Message),
			Descriptor: &p2p.ChannelDescriptor{
				ID:                  byte(BlockchainChannel),
				Priority:            5,
				SendQueueCapacity:   1000,
				RecvBufferCapacity:  50 * 4096,
				RecvMessageCapacity: bc.MaxMsgSize,
			},
		},
	}
)

const (
	// BlockchainChannel is a channel for blocks and status updates
	BlockchainChannel = p2p.ChannelID(0x40)

	trySyncIntervalMS = 10

	// ask for best height every 10s
	statusUpdateIntervalSeconds = 10

	// check if we should switch to consensus reactor
	switchToConsensusIntervalSeconds = 1

	// switch to consensus after this duration of inactivity
	syncTimeout = 60 * time.Second
)

type consensusReactor interface {
	// for when we switch from blockchain reactor and fast sync to
	// the consensus machine
	SwitchToConsensus(state sm.State, skipWAL bool)
}

type peerError struct {
	err    error
	peerID p2p.NodeID
}

func (e peerError) Error() string {
	return fmt.Sprintf("error with peer %v: %s", e.peerID, e.err.Error())
}

// BlockchainReactor handles long-term catchup syncing.
type Reactor struct {
	service.BaseService

	// immutable
	initialState sm.State

	blockExec *sm.BlockExecutor
	store     *store.BlockStore
	pool      *BlockPool
	fastSync  bool

	blockchainCh *p2p.Channel
	peerUpdates  *p2p.PeerUpdatesCh
	closeCh      chan struct{}

	requestsCh <-chan BlockRequest
	errorsCh   <-chan peerError
}

// NewReactor returns new reactor instance.
func NewReactor(
	logger log.Logger,
	state sm.State,
	blockExec *sm.BlockExecutor,
	store *store.BlockStore,
	blockchainCh *p2p.Channel,
	peerUpdates *p2p.PeerUpdatesCh,
	fastSync bool,
) *Reactor {
	if state.LastBlockHeight != store.Height() {
		panic(fmt.Sprintf("state (%v) and store (%v) height mismatch", state.LastBlockHeight, store.Height()))
	}

	startHeight := store.Height() + 1
	if startHeight == 1 {
		startHeight = state.InitialHeight
	}

	requestsCh := make(chan BlockRequest, maxTotalRequesters)
	errorsCh := make(chan peerError, 1000) // NOTE: The capacity should be larger than the peer count.

	r := &Reactor{
		initialState: state,
		blockExec:    blockExec,
		store:        store,
		pool:         NewBlockPool(startHeight, requestsCh, errorsCh),
		fastSync:     fastSync,
		requestsCh:   requestsCh,
		errorsCh:     errorsCh,
		blockchainCh: blockchainCh,
		peerUpdates:  peerUpdates,
		closeCh:      make(chan struct{}),
	}

	r.BaseService = *service.NewBaseService(logger, "Blockchain", r)
	return r
}

// OnStart starts separate go routines for each p2p Channel and listens for
// envelopes on each. In addition, it also listens for peer updates and handles
// messages on that p2p channel accordingly. The caller must be sure to execute
// OnStop to ensure the outbound p2p Channels are closed.
//
// If fastSync is enabled, we also start the pool and the pool processing
// goroutine. If the pool fails to start, an error is returned.
func (r *Reactor) OnStart() error {
	if r.fastSync {
		if err := r.pool.Start(); err != nil {
			return err
		}

		go r.poolRoutine(false)
	}

	go r.processBlockchainCh()
	go r.processPeerUpdates()

	return nil
}

// OnStop stops the reactor by signaling to all spawned goroutines to exit and
// blocking until they all exit.
func (r *Reactor) OnStop() {
	if r.fastSync {
		if err := r.pool.Stop(); err != nil {
			r.Logger.Error("failed to stop pool", "err", err)
		}
	}

	// Close closeCh to signal to all spawned goroutines to gracefully exit. All
	// p2p Channels should execute Close().
	close(r.closeCh)

	// Wait for all p2p Channels to be closed before returning. This ensures we
	// can easily reason about synchronization of all p2p Channels and ensure no
	// panics will occur.
	<-r.blockchainCh.Done()
	<-r.peerUpdates.Done()
}

// respondToPeer loads a block and sends it to the requesting peer, if we have it.
// Otherwise, we'll respond saying we do not have it.
func (r *Reactor) respondToPeer(msg *bcproto.BlockRequest, peerID p2p.NodeID) {
	block := r.store.LoadBlock(msg.Height)
	if block != nil {
		blockProto, err := block.ToProto()
		if err != nil {
			r.Logger.Error("failed to convert msg to protobuf", "err", err)
			return
		}

		r.blockchainCh.Out() <- p2p.Envelope{
			To:      peerID,
			Message: &bcproto.BlockResponse{Block: blockProto},
		}

		return
	}

	r.Logger.Info("peer requesting a block we do not have", "peer", peerID, "height", msg.Height)
	r.blockchainCh.Out() <- p2p.Envelope{
		To:      peerID,
		Message: &bcproto.NoBlockResponse{Height: msg.Height},
	}
}

// handleBlockchainMessage handles enevelopes sent from peers on the
// BlockchainChannel. It returns an error only if the Envelope.Message is unknown
// for this channel. This should never be called outside of handleMessage.
func (r *Reactor) handleBlockchainMessage(envelope p2p.Envelope) error {
	switch msg := envelope.Message.(type) {
	case *bcproto.BlockRequest:
		r.respondToPeer(msg, envelope.From)

	case *bcproto.BlockResponse:
		block, err := types.BlockFromProto(msg.Block)
		if err != nil {
			r.Logger.Error("failed to convert block from proto", "err", err)
			return err
		}

		protoMsg := new(bcproto.Message)
		if err := protoMsg.Wrap(msg); err != nil {
			r.Logger.Error("failed to wrap proto message", "err", err)
			return nil
		}

		bz, err := proto.Marshal(protoMsg)
		if err != nil {
			r.Logger.Error("failed to proto encode message", "err", err)
			return nil
		}

		r.pool.AddBlock(envelope.From, block, len(bz))

	case *bcproto.StatusRequest:
		// send peer our state
		r.blockchainCh.Out() <- p2p.Envelope{
			To: envelope.From,
			Message: &bcproto.StatusResponse{
				Height: r.store.Height(),
				Base:   r.store.Base(),
			},
		}

	case *bcproto.StatusResponse:
		// received an unverified peer status
		r.pool.SetPeerRange(envelope.From, msg.Base, msg.Height)

	case *bcproto.NoBlockResponse:
		r.Logger.Debug("peer does not have the requested block", "height", msg.Height)

	default:
		r.Logger.Error("received unknown message", "msg", msg, "peer", envelope.From)
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
			r.Logger.Error("recovering from processing message panic", "err", err)
		}
	}()

	switch chID {
	case BlockchainChannel:
		err = r.handleBlockchainMessage(envelope)

	default:
		err = fmt.Errorf("unknown channel ID (%d) for envelope (%v)", chID, envelope)
	}

	return err
}

// processBlockchainCh initiates a blocking process where we listen for and handle
// envelopes on the BlockchainChannel. Any error encountered during message
// execution will result in a PeerError being sent on the BlockchainChannel. When
// the reactor is stopped, we will catch the signal and close the p2p Channel
// gracefully.
func (r *Reactor) processBlockchainCh() {
	defer r.blockchainCh.Close()

	for {
		select {
		case envelope := <-r.blockchainCh.In():
			if err := r.handleMessage(r.blockchainCh.ID(), envelope); err != nil {
				r.Logger.Error("failed to process message", "ch_id", r.blockchainCh.ID(), "envelope", envelope, "err", err)
				r.blockchainCh.Error() <- p2p.PeerError{
					PeerID:   envelope.From,
					Err:      err,
					Severity: p2p.PeerErrorSeverityLow,
				}
			}

		case <-r.closeCh:
			r.Logger.Debug("stopped listening on blockchain channel; closing...")
			return
		}
	}
}

// processPeerUpdate processes a PeerUpdate, returning an error upon failing to
// handle the PeerUpdate or if a panic is recovered.
func (r *Reactor) processPeerUpdate(peerUpdate p2p.PeerUpdate) (err error) {
	defer func() {
		if e := recover(); e != nil {
			err = fmt.Errorf("panic in processing peer update: %v", e)
			r.Logger.Error("recovering from processing peer update panic", "err", err)
		}
	}()

	r.Logger.Debug("received peer update", "peer", peerUpdate.PeerID, "status", peerUpdate.Status)

	switch peerUpdate.Status {
	case p2p.PeerStatusNew, p2p.PeerStatusUp:
		// send a status update the newly added peer
		r.blockchainCh.Out() <- p2p.Envelope{
			To: peerUpdate.PeerID,
			Message: &bcproto.StatusResponse{
				Base:   r.store.Base(),
				Height: r.store.Height(),
			},
		}

	case p2p.PeerStatusDown, p2p.PeerStatusRemoved, p2p.PeerStatusBanned:
		r.pool.RemovePeer(peerUpdate.PeerID)
	}

	return err
}

// processPeerUpdates initiates a blocking process where we listen for and handle
// PeerUpdate messages. When the reactor is stopped, we will catch the signal and
// close the p2p PeerUpdatesCh gracefully.
func (r *Reactor) processPeerUpdates() {
	defer r.peerUpdates.Close()

	for {
		select {
		case peerUpdate := <-r.peerUpdates.Updates():
			_ = r.processPeerUpdate(peerUpdate)

		case <-r.closeCh:
			r.Logger.Debug("stopped listening on peer updates channel; closing...")
			return
		}
	}
}

// ============================================================================
// ============================================================================
// ============================================================================

// SwitchToFastSync is called by the state sync reactor when switching to fast sync.
func (bcR *BlockchainReactor) SwitchToFastSync(state sm.State) error {
	bcR.fastSync = true
	bcR.initialState = state

	bcR.pool.height = state.LastBlockHeight + 1
	err := bcR.pool.Start()
	if err != nil {
		return err
	}
	go bcR.poolRoutine(true)
	return nil
}

// Handle messages from the poolReactor telling the reactor what to do.
// NOTE: Don't sleep in the FOR_LOOP or otherwise slow it down!
func (r *Reactor) poolRoutine(stateSynced bool) {
	var (
		trySyncTicker           = time.NewTicker(trySyncIntervalMS * time.Millisecond)
		statusUpdateTicker      = time.NewTicker(statusUpdateIntervalSeconds * time.Second)
		switchToConsensusTicker = time.NewTicker(switchToConsensusIntervalSeconds * time.Second)

		blocksSynced = uint64(0)

		chainID = bcR.initialState.ChainID
		state   = bcR.initialState

		lastHundred = time.Now()
		lastRate    = 0.0

		didProcessCh = make(chan struct{}, 1)
	)

	go func() {
		for {
			select {

			case <-bcR.Quit():
				return

			case <-bcR.pool.Quit():
				return

			case request := <-bcR.requestsCh:
				peer := bcR.Switch.Peers().Get(request.PeerID)
				if peer == nil {
					bcR.Logger.Debug("Can't send request: no peer", "peer_id", request.PeerID)
					continue
				}
				msgBytes, err := bc.EncodeMsg(&bcproto.BlockRequest{Height: request.Height})
				if err != nil {
					bcR.Logger.Error("could not convert BlockRequest to proto", "err", err)
					continue
				}

				queued := peer.TrySend(BlockchainChannel, msgBytes)
				if !queued {
					bcR.Logger.Debug("Send queue is full, drop block request", "peer", peer.ID(), "height", request.Height)
				}

			case err := <-bcR.errorsCh:
				peer := bcR.Switch.Peers().Get(err.peerID)
				if peer != nil {
					bcR.Switch.StopPeerForError(peer, err)
				}

			case <-statusUpdateTicker.C:
				// ask for status updates
				go bcR.BroadcastStatusRequest()
			}
		}
	}()

FOR_LOOP:
	for {
		select {

		case <-switchToConsensusTicker.C:
			var (
				height, numPending, lenRequesters = bcR.pool.GetStatus()
				outbound, inbound, _              = bcR.Switch.NumPeers()
				lastAdvance                       = bcR.pool.LastAdvance()
			)

			bcR.Logger.Debug("Consensus ticker",
				"numPending", numPending,
				"total", lenRequesters)

			switch {
			case bcR.pool.IsCaughtUp():
				bcR.Logger.Info("Time to switch to consensus reactor!", "height", height)
			case time.Since(lastAdvance) > syncTimeout:
				bcR.Logger.Error(fmt.Sprintf("No progress since last advance: %v", lastAdvance))
			default:
				bcR.Logger.Info("Not caught up yet",
					"height", height, "max_peer_height", bcR.pool.MaxPeerHeight(),
					"num_peers", outbound+inbound,
					"timeout_in", syncTimeout-time.Since(lastAdvance))
				continue
			}

			if err := bcR.pool.Stop(); err != nil {
				bcR.Logger.Error("Error stopping pool", "err", err)
			}
			conR, ok := bcR.Switch.Reactor("CONSENSUS").(consensusReactor)
			if ok {
				conR.SwitchToConsensus(state, blocksSynced > 0 || stateSynced)
			}

			break FOR_LOOP

		case <-trySyncTicker.C: // chan time
			select {
			case didProcessCh <- struct{}{}:
			default:
			}

		case <-didProcessCh:
			// NOTE: It is a subtle mistake to process more than a single block
			// at a time (e.g. 10) here, because we only TrySend 1 request per
			// loop.  The ratio mismatch can result in starving of blocks, a
			// sudden burst of requests and responses, and repeat.
			// Consequently, it is better to split these routines rather than
			// coupling them as it's written here.  TODO uncouple from request
			// routine.

			// See if there are any blocks to sync.
			first, second := bcR.pool.PeekTwoBlocks()
			// bcR.Logger.Info("TrySync peeked", "first", first, "second", second)
			if first == nil || second == nil {
				// We need both to sync the first block.
				continue FOR_LOOP
			} else {
				// Try again quickly next loop.
				didProcessCh <- struct{}{}
			}

			var (
				firstParts         = first.MakePartSet(types.BlockPartSizeBytes)
				firstPartSetHeader = firstParts.Header()
				firstID            = types.BlockID{Hash: first.Hash(), PartSetHeader: firstPartSetHeader}
			)

			// Finally, verify the first block using the second's commit
			// NOTE: we can probably make this more efficient, but note that calling
			// first.Hash() doesn't verify the tx contents, so MakePartSet() is
			// currently necessary.
			err := state.Validators.VerifyCommitLight(chainID, firstID, first.Height, second.LastCommit)
			if err != nil {
				err = fmt.Errorf("invalid last commit: %w", err)
				bcR.Logger.Error(err.Error(),
					"last_commit", second.LastCommit, "block_id", firstID, "height", first.Height)

				peerID := bcR.pool.RedoRequest(first.Height)
				peer := bcR.Switch.Peers().Get(peerID)
				if peer != nil {
					// NOTE: we've already removed the peer's request, but we still need
					// to clean up the rest.
					bcR.Switch.StopPeerForError(peer, err)
				}

				peerID2 := bcR.pool.RedoRequest(second.Height)
				if peerID2 != peerID {
					if peer2 := bcR.Switch.Peers().Get(peerID2); peer2 != nil {
						bcR.Switch.StopPeerForError(peer2, err)
					}
				}

				continue FOR_LOOP
			} else {
				bcR.pool.PopRequest()

				// TODO: batch saves so we dont persist to disk every block
				bcR.store.SaveBlock(first, firstParts, second.LastCommit)

				// TODO: same thing for app - but we would need a way to get the hash
				// without persisting the state.
				var err error
				state, _, err = bcR.blockExec.ApplyBlock(state, firstID, first)
				if err != nil {
					// TODO This is bad, are we zombie?
					panic(fmt.Sprintf("Failed to process committed block (%d:%X): %v", first.Height, first.Hash(), err))
				}
				blocksSynced++

				if blocksSynced%100 == 0 {
					lastRate = 0.9*lastRate + 0.1*(100/time.Since(lastHundred).Seconds())
					bcR.Logger.Info("Fast Sync Rate",
						"height", bcR.pool.height, "max_peer_height", bcR.pool.MaxPeerHeight(), "blocks/s", lastRate)
					lastHundred = time.Now()
				}
			}
			continue FOR_LOOP

		case <-bcR.Quit():
			break FOR_LOOP
		}
	}
}

// BroadcastStatusRequest broadcasts `BlockStore` base and height.
func (bcR *BlockchainReactor) BroadcastStatusRequest() {
	bm, err := bc.EncodeMsg(&bcproto.StatusRequest{})
	if err != nil {
		bcR.Logger.Error("could not convert StatusRequest to proto", "err", err)
		return
	}

	// We don't care about whenever broadcast is successful or not.
	_ = bcR.Switch.Broadcast(BlockchainChannel, bm)
}
