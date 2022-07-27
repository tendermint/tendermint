package statesync

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"reflect"
	"runtime/debug"
	"sort"
	"sync"
	"time"

	abciclient "github.com/tendermint/tendermint/abci/client"
	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/internal/eventbus"
	"github.com/tendermint/tendermint/internal/p2p"
	sm "github.com/tendermint/tendermint/internal/state"
	"github.com/tendermint/tendermint/internal/store"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/libs/service"
	"github.com/tendermint/tendermint/light"
	"github.com/tendermint/tendermint/light/provider"
	ssproto "github.com/tendermint/tendermint/proto/tendermint/statesync"
	"github.com/tendermint/tendermint/types"
)

var (
	_ service.Service = (*Reactor)(nil)
	_ p2p.Wrapper     = (*ssproto.Message)(nil)
)

const (
	// SnapshotChannel exchanges snapshot metadata
	SnapshotChannel = p2p.ChannelID(0x60)

	// ChunkChannel exchanges chunk contents
	ChunkChannel = p2p.ChannelID(0x61)

	// LightBlockChannel exchanges light blocks
	LightBlockChannel = p2p.ChannelID(0x62)

	// ParamsChannel exchanges consensus params
	ParamsChannel = p2p.ChannelID(0x63)

	// recentSnapshots is the number of recent snapshots to send and receive per peer.
	recentSnapshots = 10

	// snapshotMsgSize is the maximum size of a snapshotResponseMessage
	snapshotMsgSize = int(4e6) // ~4MB

	// chunkMsgSize is the maximum size of a chunkResponseMessage
	chunkMsgSize = int(16e6) // ~16MB

	// lightBlockMsgSize is the maximum size of a lightBlockResponseMessage
	lightBlockMsgSize = int(1e7) // ~1MB

	// paramMsgSize is the maximum size of a paramsResponseMessage
	paramMsgSize = int(1e5) // ~100kb

	// lightBlockResponseTimeout is how long the dispatcher waits for a peer to
	// return a light block
	lightBlockResponseTimeout = 10 * time.Second

	// consensusParamsResponseTimeout is the time the p2p state provider waits
	// before performing a secondary call
	consensusParamsResponseTimeout = 5 * time.Second

	// maxLightBlockRequestRetries is the amount of retries acceptable before
	// the backfill process aborts
	maxLightBlockRequestRetries = 20
)

func getChannelDescriptors() map[p2p.ChannelID]*p2p.ChannelDescriptor {
	return map[p2p.ChannelID]*p2p.ChannelDescriptor{
		SnapshotChannel: {
			ID:                  SnapshotChannel,
			MessageType:         new(ssproto.Message),
			Priority:            6,
			SendQueueCapacity:   10,
			RecvMessageCapacity: snapshotMsgSize,
			RecvBufferCapacity:  128,
			Name:                "snapshot",
		},
		ChunkChannel: {
			ID:                  ChunkChannel,
			Priority:            3,
			MessageType:         new(ssproto.Message),
			SendQueueCapacity:   4,
			RecvMessageCapacity: chunkMsgSize,
			RecvBufferCapacity:  128,
			Name:                "chunk",
		},
		LightBlockChannel: {
			ID:                  LightBlockChannel,
			MessageType:         new(ssproto.Message),
			Priority:            5,
			SendQueueCapacity:   10,
			RecvMessageCapacity: lightBlockMsgSize,
			RecvBufferCapacity:  128,
			Name:                "light-block",
		},
		ParamsChannel: {
			ID:                  ParamsChannel,
			MessageType:         new(ssproto.Message),
			Priority:            2,
			SendQueueCapacity:   10,
			RecvMessageCapacity: paramMsgSize,
			RecvBufferCapacity:  128,
			Name:                "params",
		},
	}

}

// Metricer defines an interface used for the rpc sync info query, please see statesync.metrics
// for the details.
type Metricer interface {
	TotalSnapshots() int64
	ChunkProcessAvgTime() time.Duration
	SnapshotHeight() int64
	SnapshotChunksCount() int64
	SnapshotChunksTotal() int64
	BackFilledBlocks() int64
	BackFillBlocksTotal() int64
}

// Reactor handles state sync, both restoring snapshots for the local node and
// serving snapshots for other nodes.
type Reactor struct {
	service.BaseService
	logger log.Logger

	chainID       string
	initialHeight int64
	cfg           config.StateSyncConfig
	stateStore    sm.Store
	blockStore    *store.BlockStore

	conn           abciclient.Client
	tempDir        string
	peerEvents     p2p.PeerEventSubscriber
	chCreator      p2p.ChannelCreator
	sendBlockError func(context.Context, p2p.PeerError) error
	postSyncHook   func(context.Context, sm.State) error

	// when true, the reactor will, during startup perform a
	// statesync for this node, and otherwise just provide
	// snapshots to other nodes.
	needsStateSync bool

	// Dispatcher is used to multiplex light block requests and responses over multiple
	// peers used by the p2p state provider and in reverse sync.
	dispatcher *Dispatcher
	peers      *peerList

	// These will only be set when a state sync is in progress. It is used to feed
	// received snapshots and chunks into the syncer and manage incoming and outgoing
	// providers.
	mtx               sync.RWMutex
	initSyncer        func() *syncer
	requestSnaphot    func() error
	syncer            *syncer
	providers         map[types.NodeID]*BlockProvider
	initStateProvider func(ctx context.Context, chainID string, initialHeight int64) error
	stateProvider     StateProvider

	eventBus           *eventbus.EventBus
	metrics            *Metrics
	backfillBlockTotal int64
	backfilledBlocks   int64
}

// NewReactor returns a reference to a new state sync reactor, which implements
// the service.Service interface. It accepts a logger, connections for snapshots
// and querying, references to p2p Channels and a channel to listen for peer
// updates on. Note, the reactor will close all p2p Channels when stopping.
func NewReactor(
	chainID string,
	initialHeight int64,
	cfg config.StateSyncConfig,
	logger log.Logger,
	conn abciclient.Client,
	channelCreator p2p.ChannelCreator,
	peerEvents p2p.PeerEventSubscriber,
	stateStore sm.Store,
	blockStore *store.BlockStore,
	tempDir string,
	ssMetrics *Metrics,
	eventBus *eventbus.EventBus,
	postSyncHook func(context.Context, sm.State) error,
	needsStateSync bool,
) *Reactor {
	r := &Reactor{
		logger:         logger,
		chainID:        chainID,
		initialHeight:  initialHeight,
		cfg:            cfg,
		conn:           conn,
		chCreator:      channelCreator,
		peerEvents:     peerEvents,
		tempDir:        tempDir,
		stateStore:     stateStore,
		blockStore:     blockStore,
		peers:          newPeerList(),
		providers:      make(map[types.NodeID]*BlockProvider),
		metrics:        ssMetrics,
		eventBus:       eventBus,
		postSyncHook:   postSyncHook,
		needsStateSync: needsStateSync,
	}

	r.BaseService = *service.NewBaseService(logger, "StateSync", r)
	return r
}

// OnStart starts separate go routines for each p2p Channel and listens for
// envelopes on each. In addition, it also listens for peer updates and handles
// messages on that p2p channel accordingly. Note, we do not launch a go-routine to
// handle individual envelopes as to not have to deal with bounding workers or pools.
// The caller must be sure to execute OnStop to ensure the outbound p2p Channels are
// closed. No error is returned.
func (r *Reactor) OnStart(ctx context.Context) error {
	// construct channels
	chDesc := getChannelDescriptors()
	snapshotCh, err := r.chCreator(ctx, chDesc[SnapshotChannel])
	if err != nil {
		return err
	}
	chunkCh, err := r.chCreator(ctx, chDesc[ChunkChannel])
	if err != nil {
		return err
	}
	blockCh, err := r.chCreator(ctx, chDesc[LightBlockChannel])
	if err != nil {
		return err
	}
	paramsCh, err := r.chCreator(ctx, chDesc[ParamsChannel])
	if err != nil {
		return err
	}

	// define constructor and helper functions, that hold
	// references to these channels for use later. This is not
	// ideal.
	r.initSyncer = func() *syncer {
		return &syncer{
			logger:        r.logger,
			stateProvider: r.stateProvider,
			conn:          r.conn,
			snapshots:     newSnapshotPool(),
			snapshotCh:    snapshotCh,
			chunkCh:       chunkCh,
			tempDir:       r.tempDir,
			fetchers:      r.cfg.Fetchers,
			retryTimeout:  r.cfg.ChunkRequestTimeout,
			metrics:       r.metrics,
		}
	}
	r.dispatcher = NewDispatcher(blockCh)
	r.requestSnaphot = func() error {
		// request snapshots from all currently connected peers
		return snapshotCh.Send(ctx, p2p.Envelope{
			Broadcast: true,
			Message:   &ssproto.SnapshotsRequest{},
		})
	}
	r.sendBlockError = blockCh.SendError

	r.initStateProvider = func(ctx context.Context, chainID string, initialHeight int64) error {
		to := light.TrustOptions{
			Period: r.cfg.TrustPeriod,
			Height: r.cfg.TrustHeight,
			Hash:   r.cfg.TrustHashBytes(),
		}
		spLogger := r.logger.With("module", "stateprovider")
		spLogger.Info("initializing state provider", "trustPeriod", to.Period,
			"trustHeight", to.Height, "useP2P", r.cfg.UseP2P)

		if r.cfg.UseP2P {
			if err := r.waitForEnoughPeers(ctx, 2); err != nil {
				return err
			}

			peers := r.peers.All()
			providers := make([]provider.Provider, len(peers))
			for idx, p := range peers {
				providers[idx] = NewBlockProvider(p, chainID, r.dispatcher)
			}

			stateProvider, err := NewP2PStateProvider(ctx, chainID, initialHeight, providers, to, paramsCh, r.logger.With("module", "stateprovider"))
			if err != nil {
				return fmt.Errorf("failed to initialize P2P state provider: %w", err)
			}
			r.stateProvider = stateProvider
			return nil
		}

		stateProvider, err := NewRPCStateProvider(ctx, chainID, initialHeight, r.cfg.RPCServers, to, spLogger)
		if err != nil {
			return fmt.Errorf("failed to initialize RPC state provider: %w", err)
		}
		r.stateProvider = stateProvider
		return nil
	}

	go r.processChannels(ctx, map[p2p.ChannelID]p2p.Channel{
		SnapshotChannel:   snapshotCh,
		ChunkChannel:      chunkCh,
		LightBlockChannel: blockCh,
		ParamsChannel:     paramsCh,
	})
	go r.processPeerUpdates(ctx, r.peerEvents(ctx))

	if r.needsStateSync {
		r.logger.Info("starting state sync")
		if _, err := r.Sync(ctx); err != nil {
			r.logger.Error("state sync failed; shutting down this node", "err", err)
			return err
		}
	}

	return nil
}

// OnStop stops the reactor by signaling to all spawned goroutines to exit and
// blocking until they all exit.
func (r *Reactor) OnStop() {
	// tell the dispatcher to stop sending any more requests
	r.dispatcher.Close()
}

// Sync runs a state sync, fetching snapshots and providing chunks to the
// application. At the close of the operation, Sync will bootstrap the state
// store and persist the commit at that height so that either consensus or
// blocksync can commence. It will then proceed to backfill the necessary amount
// of historical blocks before participating in consensus
func (r *Reactor) Sync(ctx context.Context) (sm.State, error) {
	if r.eventBus != nil {
		if err := r.eventBus.PublishEventStateSyncStatus(types.EventDataStateSyncStatus{
			Complete: false,
			Height:   r.initialHeight,
		}); err != nil {
			return sm.State{}, err
		}
	}

	// We need at least two peers (for cross-referencing of light blocks) before we can
	// begin state sync
	if err := r.waitForEnoughPeers(ctx, 2); err != nil {
		return sm.State{}, err
	}

	r.mtx.Lock()
	if r.syncer != nil {
		r.mtx.Unlock()
		return sm.State{}, errors.New("a state sync is already in progress")
	}

	if err := r.initStateProvider(ctx, r.chainID, r.initialHeight); err != nil {
		r.mtx.Unlock()
		return sm.State{}, err
	}

	r.syncer = r.initSyncer()
	r.mtx.Unlock()

	defer func() {
		r.mtx.Lock()
		// reset syncing objects at the close of Sync
		r.syncer = nil
		r.stateProvider = nil
		r.mtx.Unlock()
	}()

	state, commit, err := r.syncer.SyncAny(ctx, r.cfg.DiscoveryTime, r.requestSnaphot)
	if err != nil {
		return sm.State{}, err
	}

	if err := r.stateStore.Bootstrap(state); err != nil {
		return sm.State{}, fmt.Errorf("failed to bootstrap node with new state: %w", err)
	}

	if err := r.blockStore.SaveSeenCommit(state.LastBlockHeight, commit); err != nil {
		return sm.State{}, fmt.Errorf("failed to store last seen commit: %w", err)
	}

	if err := r.Backfill(ctx, state); err != nil {
		r.logger.Error("backfill failed. Proceeding optimistically...", "err", err)
	}

	if r.eventBus != nil {
		if err := r.eventBus.PublishEventStateSyncStatus(types.EventDataStateSyncStatus{
			Complete: true,
			Height:   state.LastBlockHeight,
		}); err != nil {
			return sm.State{}, err
		}
	}

	if r.postSyncHook != nil {
		if err := r.postSyncHook(ctx, state); err != nil {
			return sm.State{}, err
		}
	}

	return state, nil
}

// Backfill sequentially fetches, verifies and stores light blocks in reverse
// order. It does not stop verifying blocks until reaching a block with a height
// and time that is less or equal to the stopHeight and stopTime. The
// trustedBlockID should be of the header at startHeight.
func (r *Reactor) Backfill(ctx context.Context, state sm.State) error {
	params := state.ConsensusParams.Evidence
	stopHeight := state.LastBlockHeight - params.MaxAgeNumBlocks
	stopTime := state.LastBlockTime.Add(-params.MaxAgeDuration)
	// ensure that stop height doesn't go below the initial height
	if stopHeight < state.InitialHeight {
		stopHeight = state.InitialHeight
		// this essentially makes stop time a void criteria for termination
		stopTime = state.LastBlockTime
	}
	return r.backfill(
		ctx,
		state.ChainID,
		state.LastBlockHeight,
		stopHeight,
		state.InitialHeight,
		state.LastBlockID,
		stopTime,
	)
}

func (r *Reactor) backfill(
	ctx context.Context,
	chainID string,
	startHeight, stopHeight, initialHeight int64,
	trustedBlockID types.BlockID,
	stopTime time.Time,
) error {
	r.logger.Info("starting backfill process...", "startHeight", startHeight,
		"stopHeight", stopHeight, "stopTime", stopTime, "trustedBlockID", trustedBlockID)

	r.backfillBlockTotal = startHeight - stopHeight + 1
	r.metrics.BackFillBlocksTotal.Set(float64(r.backfillBlockTotal))

	const sleepTime = 1 * time.Second
	var (
		lastValidatorSet *types.ValidatorSet
		lastChangeHeight = startHeight
	)

	queue := newBlockQueue(startHeight, stopHeight, initialHeight, stopTime, maxLightBlockRequestRetries)

	// fetch light blocks across four workers. The aim with deploying concurrent
	// workers is to equate the network messaging time with the verification
	// time. Ideally we want the verification process to never have to be
	// waiting on blocks. If it takes 4s to retrieve a block and 1s to verify
	// it, then steady state involves four workers.
	for i := 0; i < int(r.cfg.Fetchers); i++ {
		ctxWithCancel, cancel := context.WithCancel(ctx)
		defer cancel()
		go func() {
			for {
				select {
				case <-ctx.Done():
					return
				case height := <-queue.nextHeight():
					// pop the next peer of the list to send a request to
					peer := r.peers.Pop(ctx)
					r.logger.Debug("fetching next block", "height", height, "peer", peer)
					subCtx, cancel := context.WithTimeout(ctxWithCancel, lightBlockResponseTimeout)
					defer cancel()
					lb, err := func() (*types.LightBlock, error) {
						defer cancel()
						// request the light block with a timeout
						return r.dispatcher.LightBlock(subCtx, height, peer)
					}()
					// once the peer has returned a value, add it back to the peer list to be used again
					r.peers.Append(peer)
					if errors.Is(err, context.Canceled) {
						return
					}
					if err != nil {
						queue.retry(height)
						if errors.Is(err, errNoConnectedPeers) {
							r.logger.Info("backfill: no connected peers to fetch light blocks from; sleeping...",
								"sleepTime", sleepTime)
							time.Sleep(sleepTime)
						} else {
							// we don't punish the peer as it might just have not responded in time
							r.logger.Info("backfill: error with fetching light block",
								"height", height, "err", err)
						}
						continue
					}
					if lb == nil {
						r.logger.Info("backfill: peer didn't have block, fetching from another peer", "height", height)
						queue.retry(height)
						// As we are fetching blocks backwards, if this node doesn't have the block it likely doesn't
						// have any prior ones, thus we remove it from the peer list.
						r.peers.Remove(peer)
						continue
					}

					// run a validate basic. This checks the validator set and commit
					// hashes line up
					err = lb.ValidateBasic(chainID)
					if err != nil || lb.Height != height {
						r.logger.Info("backfill: fetched light block failed validate basic, removing peer...",
							"err", err, "height", height)
						queue.retry(height)
						if serr := r.sendBlockError(ctx, p2p.PeerError{
							NodeID: peer,
							Err:    fmt.Errorf("received invalid light block: %w", err),
						}); serr != nil {
							return
						}
						continue
					}

					// add block to queue to be verified
					queue.add(lightBlockResponse{
						block: lb,
						peer:  peer,
					})
					r.logger.Debug("backfill: added light block to processing queue", "height", height)

				case <-queue.done():
					return
				}
			}
		}()
	}

	// verify all light blocks
	for {
		select {
		case <-ctx.Done():
			queue.close()
			return nil
		case resp := <-queue.verifyNext():
			// validate the header hash. We take the last block id of the
			// previous header (i.e. one height above) as the trusted hash which
			// we equate to. ValidatorsHash and CommitHash have already been
			// checked in the `ValidateBasic`
			if w, g := trustedBlockID.Hash, resp.block.Hash(); !bytes.Equal(w, g) {
				r.logger.Info("received invalid light block. header hash doesn't match trusted LastBlockID",
					"trustedHash", w, "receivedHash", g, "height", resp.block.Height)
				if err := r.sendBlockError(ctx, p2p.PeerError{
					NodeID: resp.peer,
					Err:    fmt.Errorf("received invalid light block. Expected hash %v, got: %v", w, g),
				}); err != nil {
					return nil
				}
				queue.retry(resp.block.Height)
				continue
			}

			// save the signed headers
			if err := r.blockStore.SaveSignedHeader(resp.block.SignedHeader, trustedBlockID); err != nil {
				return err
			}

			// check if there has been a change in the validator set
			if lastValidatorSet != nil && !bytes.Equal(resp.block.Header.ValidatorsHash, resp.block.Header.NextValidatorsHash) {
				// save all the heights that the last validator set was the same
				if err := r.stateStore.SaveValidatorSets(resp.block.Height+1, lastChangeHeight, lastValidatorSet); err != nil {
					return err
				}

				// update the lastChangeHeight
				lastChangeHeight = resp.block.Height
			}

			trustedBlockID = resp.block.LastBlockID
			queue.success()
			r.logger.Info("backfill: verified and stored light block", "height", resp.block.Height)

			lastValidatorSet = resp.block.ValidatorSet

			r.backfilledBlocks++
			r.metrics.BackFilledBlocks.Add(1)

			// The block height might be less than the stopHeight because of the stopTime condition
			// hasn't been fulfilled.
			if resp.block.Height < stopHeight {
				r.backfillBlockTotal++
				r.metrics.BackFillBlocksTotal.Set(float64(r.backfillBlockTotal))
			}

		case <-queue.done():
			if err := queue.error(); err != nil {
				return err
			}

			// save the final batch of validators
			if err := r.stateStore.SaveValidatorSets(queue.terminal.Height, lastChangeHeight, lastValidatorSet); err != nil {
				return err
			}

			r.logger.Info("successfully completed backfill process", "endHeight", queue.terminal.Height)
			return nil
		}
	}
}

// handleSnapshotMessage handles envelopes sent from peers on the
// SnapshotChannel. It returns an error only if the Envelope.Message is unknown
// for this channel. This should never be called outside of handleMessage.
func (r *Reactor) handleSnapshotMessage(ctx context.Context, envelope *p2p.Envelope, snapshotCh p2p.Channel) error {
	logger := r.logger.With("peer", envelope.From)

	switch msg := envelope.Message.(type) {
	case *ssproto.SnapshotsRequest:
		snapshots, err := r.recentSnapshots(ctx, recentSnapshots)
		if err != nil {
			logger.Error("failed to fetch snapshots", "err", err)
			return nil
		}

		for _, snapshot := range snapshots {
			logger.Info(
				"advertising snapshot",
				"height", snapshot.Height,
				"format", snapshot.Format,
				"peer", envelope.From,
			)

			if err := snapshotCh.Send(ctx, p2p.Envelope{
				To: envelope.From,
				Message: &ssproto.SnapshotsResponse{
					Height:   snapshot.Height,
					Format:   snapshot.Format,
					Chunks:   snapshot.Chunks,
					Hash:     snapshot.Hash,
					Metadata: snapshot.Metadata,
				},
			}); err != nil {
				return err
			}
		}

	case *ssproto.SnapshotsResponse:
		r.mtx.RLock()
		defer r.mtx.RUnlock()

		if r.syncer == nil {
			logger.Debug("received unexpected snapshot; no state sync in progress")
			return nil
		}

		logger.Info("received snapshot", "height", msg.Height, "format", msg.Format)
		_, err := r.syncer.AddSnapshot(envelope.From, &snapshot{
			Height:   msg.Height,
			Format:   msg.Format,
			Chunks:   msg.Chunks,
			Hash:     msg.Hash,
			Metadata: msg.Metadata,
		})
		if err != nil {
			logger.Error(
				"failed to add snapshot",
				"height", msg.Height,
				"format", msg.Format,
				"channel", envelope.ChannelID,
				"err", err,
			)
			return nil
		}
		logger.Info("added snapshot", "height", msg.Height, "format", msg.Format)

	default:
		return fmt.Errorf("received unknown message: %T", msg)
	}

	return nil
}

// handleChunkMessage handles envelopes sent from peers on the ChunkChannel.
// It returns an error only if the Envelope.Message is unknown for this channel.
// This should never be called outside of handleMessage.
func (r *Reactor) handleChunkMessage(ctx context.Context, envelope *p2p.Envelope, chunkCh p2p.Channel) error {
	switch msg := envelope.Message.(type) {
	case *ssproto.ChunkRequest:
		r.logger.Debug("received chunk request",
			"height", msg.Height,
			"format", msg.Format,
			"chunk", msg.Index,
			"peer", envelope.From)
		resp, err := r.conn.LoadSnapshotChunk(ctx, &abci.RequestLoadSnapshotChunk{
			Height: msg.Height,
			Format: msg.Format,
			Chunk:  msg.Index,
		})
		if err != nil {
			r.logger.Error("failed to load chunk",
				"height", msg.Height,
				"format", msg.Format,
				"chunk", msg.Index,
				"err", err,
				"peer", envelope.From)
			return nil
		}

		r.logger.Debug("sending chunk",
			"height", msg.Height,
			"format", msg.Format,
			"chunk", msg.Index,
			"peer", envelope.From)
		if err := chunkCh.Send(ctx, p2p.Envelope{
			To: envelope.From,
			Message: &ssproto.ChunkResponse{
				Height:  msg.Height,
				Format:  msg.Format,
				Index:   msg.Index,
				Chunk:   resp.Chunk,
				Missing: resp.Chunk == nil,
			},
		}); err != nil {
			return err
		}

	case *ssproto.ChunkResponse:
		r.mtx.RLock()
		defer r.mtx.RUnlock()

		if r.syncer == nil {
			r.logger.Debug("received unexpected chunk; no state sync in progress", "peer", envelope.From)
			return nil
		}

		r.logger.Debug("received chunk; adding to sync",
			"height", msg.Height,
			"format", msg.Format,
			"chunk", msg.Index,
			"peer", envelope.From)
		_, err := r.syncer.AddChunk(&chunk{
			Height: msg.Height,
			Format: msg.Format,
			Index:  msg.Index,
			Chunk:  msg.Chunk,
			Sender: envelope.From,
		})
		if err != nil {
			r.logger.Error("failed to add chunk",
				"height", msg.Height,
				"format", msg.Format,
				"chunk", msg.Index,
				"err", err,
				"peer", envelope.From)
			return nil
		}

	default:
		return fmt.Errorf("received unknown message: %T", msg)
	}

	return nil
}

func (r *Reactor) handleLightBlockMessage(ctx context.Context, envelope *p2p.Envelope, blockCh p2p.Channel) error {
	switch msg := envelope.Message.(type) {
	case *ssproto.LightBlockRequest:
		r.logger.Info("received light block request", "height", msg.Height)
		lb, err := r.fetchLightBlock(msg.Height)
		if err != nil {
			r.logger.Error("failed to retrieve light block", "err", err, "height", msg.Height)
			return err
		}
		if lb == nil {
			if err := blockCh.Send(ctx, p2p.Envelope{
				To: envelope.From,
				Message: &ssproto.LightBlockResponse{
					LightBlock: nil,
				},
			}); err != nil {
				return err
			}
			return nil
		}

		lbproto, err := lb.ToProto()
		if err != nil {
			r.logger.Error("marshaling light block to proto", "err", err)
			return nil
		}

		// NOTE: If we don't have the light block we will send a nil light block
		// back to the requested node, indicating that we don't have it.
		if err := blockCh.Send(ctx, p2p.Envelope{
			To: envelope.From,
			Message: &ssproto.LightBlockResponse{
				LightBlock: lbproto,
			},
		}); err != nil {
			return err
		}
	case *ssproto.LightBlockResponse:
		var height int64
		if msg.LightBlock != nil {
			height = msg.LightBlock.SignedHeader.Header.Height
		}
		r.logger.Info("received light block response", "peer", envelope.From, "height", height)
		if err := r.dispatcher.Respond(ctx, msg.LightBlock, envelope.From); err != nil {
			if errors.Is(err, context.Canceled) {
				return err
			}
			r.logger.Error("error processing light block response", "err", err, "height", height)
		}

	default:
		return fmt.Errorf("received unknown message: %T", msg)
	}

	return nil
}

func (r *Reactor) handleParamsMessage(ctx context.Context, envelope *p2p.Envelope, paramsCh p2p.Channel) error {
	switch msg := envelope.Message.(type) {
	case *ssproto.ParamsRequest:
		r.logger.Debug("received consensus params request", "height", msg.Height)
		cp, err := r.stateStore.LoadConsensusParams(int64(msg.Height))
		if err != nil {
			r.logger.Error("failed to fetch requested consensus params", "err", err, "height", msg.Height)
			return nil
		}

		cpproto := cp.ToProto()
		if err := paramsCh.Send(ctx, p2p.Envelope{
			To: envelope.From,
			Message: &ssproto.ParamsResponse{
				Height:          msg.Height,
				ConsensusParams: cpproto,
			},
		}); err != nil {
			return err
		}
	case *ssproto.ParamsResponse:
		r.mtx.RLock()
		defer r.mtx.RUnlock()
		r.logger.Debug("received consensus params response", "height", msg.Height)

		cp := types.ConsensusParamsFromProto(msg.ConsensusParams)

		if sp, ok := r.stateProvider.(*stateProviderP2P); ok {
			select {
			case sp.paramsRecvCh <- cp:
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(time.Second):
				return errors.New("failed to send consensus params, stateprovider not ready for response")
			}
		} else {
			r.logger.Debug("received unexpected params response; using RPC state provider", "peer", envelope.From)
		}

	default:
		return fmt.Errorf("received unknown message: %T", msg)
	}

	return nil
}

// handleMessage handles an Envelope sent from a peer on a specific p2p Channel.
// It will handle errors and any possible panics gracefully. A caller can handle
// any error returned by sending a PeerError on the respective channel.
func (r *Reactor) handleMessage(ctx context.Context, envelope *p2p.Envelope, chans map[p2p.ChannelID]p2p.Channel) (err error) {
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

	r.logger.Debug("received message", "message", reflect.TypeOf(envelope.Message), "peer", envelope.From)

	switch envelope.ChannelID {
	case SnapshotChannel:
		err = r.handleSnapshotMessage(ctx, envelope, chans[SnapshotChannel])
	case ChunkChannel:
		err = r.handleChunkMessage(ctx, envelope, chans[ChunkChannel])
	case LightBlockChannel:
		err = r.handleLightBlockMessage(ctx, envelope, chans[LightBlockChannel])
	case ParamsChannel:
		err = r.handleParamsMessage(ctx, envelope, chans[ParamsChannel])
	default:
		err = fmt.Errorf("unknown channel ID (%d) for envelope (%v)", envelope.ChannelID, envelope)
	}

	return err
}

// processCh routes state sync messages to their respective handlers. Any error
// encountered during message execution will result in a PeerError being sent on
// the respective channel. When the reactor is stopped, we will catch the signal
// and close the p2p Channel gracefully.
func (r *Reactor) processChannels(ctx context.Context, chanTable map[p2p.ChannelID]p2p.Channel) {
	// make sure tht the iterator gets cleaned up in case of error
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	chs := make([]p2p.Channel, 0, len(chanTable))
	for key := range chanTable {
		chs = append(chs, chanTable[key])
	}

	iter := p2p.MergedChannelIterator(ctx, chs...)
	for iter.Next(ctx) {
		envelope := iter.Envelope()
		if err := r.handleMessage(ctx, envelope, chanTable); err != nil {
			ch, ok := chanTable[envelope.ChannelID]
			if !ok {
				r.logger.Error("received impossible message",
					"envelope_from", envelope.From,
					"envelope_ch", envelope.ChannelID,
					"num_chs", len(chanTable),
					"err", err,
				)
				return
			}
			r.logger.Error("failed to process message",
				"err", err,
				"channel", ch.String(),
				"ch_id", envelope.ChannelID,
				"envelope", envelope)
			if serr := ch.SendError(ctx, p2p.PeerError{
				NodeID: envelope.From,
				Err:    err,
			}); serr != nil {
				return
			}
		}
	}
}

// processPeerUpdate processes a PeerUpdate, returning an error upon failing to
// handle the PeerUpdate or if a panic is recovered.
func (r *Reactor) processPeerUpdate(ctx context.Context, peerUpdate p2p.PeerUpdate) {
	r.logger.Info("received peer update", "peer", peerUpdate.NodeID, "status", peerUpdate.Status)

	switch peerUpdate.Status {
	case p2p.PeerStatusUp:
		if peerUpdate.Channels.Contains(SnapshotChannel) &&
			peerUpdate.Channels.Contains(ChunkChannel) &&
			peerUpdate.Channels.Contains(LightBlockChannel) &&
			peerUpdate.Channels.Contains(ParamsChannel) {

			r.peers.Append(peerUpdate.NodeID)

		} else {
			r.logger.Error("could not use peer for statesync", "peer", peerUpdate.NodeID)
		}

	case p2p.PeerStatusDown:
		r.peers.Remove(peerUpdate.NodeID)
	}

	r.mtx.Lock()
	defer r.mtx.Unlock()

	if r.syncer == nil {
		return
	}

	switch peerUpdate.Status {
	case p2p.PeerStatusUp:
		newProvider := NewBlockProvider(peerUpdate.NodeID, r.chainID, r.dispatcher)

		r.providers[peerUpdate.NodeID] = newProvider
		err := r.syncer.AddPeer(ctx, peerUpdate.NodeID)
		if err != nil {
			r.logger.Error("error adding peer to syncer", "error", err)
			return
		}
		if sp, ok := r.stateProvider.(*stateProviderP2P); ok {
			// we do this in a separate routine to not block whilst waiting for the light client to finish
			// whatever call it's currently executing
			go sp.addProvider(newProvider)
		}

	case p2p.PeerStatusDown:
		delete(r.providers, peerUpdate.NodeID)
		r.syncer.RemovePeer(peerUpdate.NodeID)
	}
	r.logger.Info("processed peer update", "peer", peerUpdate.NodeID, "status", peerUpdate.Status)
}

// processPeerUpdates initiates a blocking process where we listen for and handle
// PeerUpdate messages. When the reactor is stopped, we will catch the signal and
// close the p2p PeerUpdatesCh gracefully.
func (r *Reactor) processPeerUpdates(ctx context.Context, peerUpdates *p2p.PeerUpdates) {
	for {
		select {
		case <-ctx.Done():
			return
		case peerUpdate := <-peerUpdates.Updates():
			r.processPeerUpdate(ctx, peerUpdate)
		}
	}
}

// recentSnapshots fetches the n most recent snapshots from the app
func (r *Reactor) recentSnapshots(ctx context.Context, n uint32) ([]*snapshot, error) {
	resp, err := r.conn.ListSnapshots(ctx, &abci.RequestListSnapshots{})
	if err != nil {
		return nil, err
	}

	sort.Slice(resp.Snapshots, func(i, j int) bool {
		a := resp.Snapshots[i]
		b := resp.Snapshots[j]

		switch {
		case a.Height > b.Height:
			return true
		case a.Height == b.Height && a.Format > b.Format:
			return true
		default:
			return false
		}
	})

	snapshots := make([]*snapshot, 0, n)
	for i, s := range resp.Snapshots {
		if i >= recentSnapshots {
			break
		}

		snapshots = append(snapshots, &snapshot{
			Height:   s.Height,
			Format:   s.Format,
			Chunks:   s.Chunks,
			Hash:     s.Hash,
			Metadata: s.Metadata,
		})
	}

	return snapshots, nil
}

// fetchLightBlock works out whether the node has a light block at a particular
// height and if so returns it so it can be gossiped to peers
func (r *Reactor) fetchLightBlock(height uint64) (*types.LightBlock, error) {
	h := int64(height)

	blockMeta := r.blockStore.LoadBlockMeta(h)
	if blockMeta == nil {
		return nil, nil
	}

	commit := r.blockStore.LoadBlockCommit(h)
	if commit == nil {
		return nil, nil
	}

	vals, err := r.stateStore.LoadValidators(h)
	if err != nil {
		return nil, err
	}
	if vals == nil {
		return nil, nil
	}

	return &types.LightBlock{
		SignedHeader: &types.SignedHeader{
			Header: &blockMeta.Header,
			Commit: commit,
		},
		ValidatorSet: vals,
	}, nil
}

func (r *Reactor) waitForEnoughPeers(ctx context.Context, numPeers int) error {
	startAt := time.Now()
	t := time.NewTicker(100 * time.Millisecond)
	defer t.Stop()
	logT := time.NewTicker(time.Minute)
	defer logT.Stop()
	var iter int
	for r.peers.Len() < numPeers {
		iter++
		select {
		case <-ctx.Done():
			return fmt.Errorf("operation canceled while waiting for peers after %.2fs [%d/%d]",
				time.Since(startAt).Seconds(), r.peers.Len(), numPeers)
		case <-t.C:
			continue
		case <-logT.C:
			r.logger.Info("waiting for sufficient peers to start statesync",
				"duration", time.Since(startAt).String(),
				"target", numPeers,
				"peers", r.peers.Len(),
				"iters", iter,
			)
			continue
		}
	}
	return nil
}

func (r *Reactor) TotalSnapshots() int64 {
	r.mtx.RLock()
	defer r.mtx.RUnlock()

	if r.syncer != nil && r.syncer.snapshots != nil {
		return int64(len(r.syncer.snapshots.snapshots))
	}
	return 0
}

func (r *Reactor) ChunkProcessAvgTime() time.Duration {
	r.mtx.RLock()
	defer r.mtx.RUnlock()

	if r.syncer != nil {
		return time.Duration(r.syncer.avgChunkTime)
	}
	return time.Duration(0)
}

func (r *Reactor) SnapshotHeight() int64 {
	r.mtx.RLock()
	defer r.mtx.RUnlock()

	if r.syncer != nil {
		return r.syncer.lastSyncedSnapshotHeight
	}
	return 0
}
func (r *Reactor) SnapshotChunksCount() int64 {
	r.mtx.RLock()
	defer r.mtx.RUnlock()

	if r.syncer != nil && r.syncer.chunks != nil {
		return int64(r.syncer.chunks.numChunksReturned())
	}
	return 0
}

func (r *Reactor) SnapshotChunksTotal() int64 {
	r.mtx.RLock()
	defer r.mtx.RUnlock()

	if r.syncer != nil && r.syncer.processingSnapshot != nil {
		return int64(r.syncer.processingSnapshot.Chunks)
	}
	return 0
}

func (r *Reactor) BackFilledBlocks() int64 {
	r.mtx.RLock()
	defer r.mtx.RUnlock()

	return r.backfilledBlocks
}

func (r *Reactor) BackFillBlocksTotal() int64 {
	r.mtx.RLock()
	defer r.mtx.RUnlock()

	return r.backfillBlockTotal
}
