package statesync

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"time"

	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/config"
	tmsync "github.com/tendermint/tendermint/internal/libs/sync"
	"github.com/tendermint/tendermint/internal/p2p"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/libs/service"
	ssproto "github.com/tendermint/tendermint/proto/tendermint/statesync"
	"github.com/tendermint/tendermint/proxy"
	sm "github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/store"
	"github.com/tendermint/tendermint/types"
)

var (
	_ service.Service = (*Reactor)(nil)
	_ p2p.Wrapper     = (*ssproto.Message)(nil)

	// ChannelShims contains a map of ChannelDescriptorShim objects, where each
	// object wraps a reference to a legacy p2p ChannelDescriptor and the corresponding
	// p2p proto.Message the new p2p Channel is responsible for handling.
	//
	//
	// TODO: Remove once p2p refactor is complete.
	// ref: https://github.com/tendermint/tendermint/issues/5670
	ChannelShims = map[p2p.ChannelID]*p2p.ChannelDescriptorShim{
		SnapshotChannel: {
			MsgType: new(ssproto.Message),
			Descriptor: &p2p.ChannelDescriptor{
				ID:                  byte(SnapshotChannel),
				Priority:            5,
				SendQueueCapacity:   10,
				RecvMessageCapacity: snapshotMsgSize,

				MaxSendBytes: 400,
			},
		},
		ChunkChannel: {
			MsgType: new(ssproto.Message),
			Descriptor: &p2p.ChannelDescriptor{
				ID:                  byte(ChunkChannel),
				Priority:            1,
				SendQueueCapacity:   4,
				RecvMessageCapacity: chunkMsgSize,

				MaxSendBytes: 400,
			},
		},
		LightBlockChannel: {
			MsgType: new(ssproto.Message),
			Descriptor: &p2p.ChannelDescriptor{
				ID:                  byte(LightBlockChannel),
				Priority:            1,
				SendQueueCapacity:   10,
				RecvMessageCapacity: lightBlockMsgSize,

				MaxSendBytes: 400,
			},
		},
	}
)

const (
	// SnapshotChannel exchanges snapshot metadata
	SnapshotChannel = p2p.ChannelID(0x60)

	// ChunkChannel exchanges chunk contents
	ChunkChannel = p2p.ChannelID(0x61)

	// LightBlockChannel exchanges light blocks
	LightBlockChannel = p2p.ChannelID(0x62)

	// recentSnapshots is the number of recent snapshots to send and receive per peer.
	recentSnapshots = 10

	// snapshotMsgSize is the maximum size of a snapshotResponseMessage
	snapshotMsgSize = int(4e6)

	// chunkMsgSize is the maximum size of a chunkResponseMessage
	chunkMsgSize = int(16e6)

	// lightBlockMsgSize is the maximum size of a lightBlockResponseMessage
	lightBlockMsgSize = int(1e7)

	// lightBlockResponseTimeout is how long the dispatcher waits for a peer to
	// return a light block
	lightBlockResponseTimeout = 10 * time.Second

	// maxLightBlockRequestRetries is the amount of retries acceptable before
	// the backfill process aborts
	maxLightBlockRequestRetries = 20

	// the amount of processes fetching light blocks - this should be roughly calculated
	// as the time to fetch a block / time to verify a block
	lightBlockFetchers = 4
)

// Reactor handles state sync, both restoring snapshots for the local node and
// serving snapshots for other nodes.
type Reactor struct {
	service.BaseService

	cfg        config.StateSyncConfig
	stateStore sm.Store
	blockStore *store.BlockStore

	conn        proxy.AppConnSnapshot
	connQuery   proxy.AppConnQuery
	tempDir     string
	snapshotCh  *p2p.Channel
	chunkCh     *p2p.Channel
	blockCh     *p2p.Channel
	peerUpdates *p2p.PeerUpdates
	closeCh     chan struct{}

	dispatcher *dispatcher

	// This will only be set when a state sync is in progress. It is used to feed
	// received snapshots and chunks into the sync.
	mtx    tmsync.RWMutex
	syncer *syncer
}

// NewReactor returns a reference to a new state sync reactor, which implements
// the service.Service interface. It accepts a logger, connections for snapshots
// and querying, references to p2p Channels and a channel to listen for peer
// updates on. Note, the reactor will close all p2p Channels when stopping.
func NewReactor(
	cfg config.StateSyncConfig,
	logger log.Logger,
	conn proxy.AppConnSnapshot,
	connQuery proxy.AppConnQuery,
	snapshotCh, chunkCh, blockCh *p2p.Channel,
	peerUpdates *p2p.PeerUpdates,
	stateStore sm.Store,
	blockStore *store.BlockStore,
	tempDir string,
) *Reactor {
	r := &Reactor{
		cfg:         cfg,
		conn:        conn,
		connQuery:   connQuery,
		snapshotCh:  snapshotCh,
		chunkCh:     chunkCh,
		blockCh:     blockCh,
		peerUpdates: peerUpdates,
		closeCh:     make(chan struct{}),
		tempDir:     tempDir,
		dispatcher:  newDispatcher(blockCh.Out, lightBlockResponseTimeout),
		stateStore:  stateStore,
		blockStore:  blockStore,
	}

	r.BaseService = *service.NewBaseService(logger, "StateSync", r)
	return r
}

// OnStart starts separate go routines for each p2p Channel and listens for
// envelopes on each. In addition, it also listens for peer updates and handles
// messages on that p2p channel accordingly. The caller must be sure to execute
// OnStop to ensure the outbound p2p Channels are closed. No error is returned.
func (r *Reactor) OnStart() error {
	// Listen for envelopes on the snapshot p2p Channel in a separate go-routine
	// as to not block or cause IO contention with the chunk p2p Channel. Note,
	// we do not launch a go-routine to handle individual envelopes as to not
	// have to deal with bounding workers or pools.
	go r.processSnapshotCh()

	// Listen for envelopes on the chunk p2p Channel in a separate go-routine
	// as to not block or cause IO contention with the snapshot p2p Channel. Note,
	// we do not launch a go-routine to handle individual envelopes as to not
	// have to deal with bounding workers or pools.
	go r.processChunkCh()

	go r.processBlockCh()

	go r.processPeerUpdates()

	r.dispatcher.start()

	return nil
}

// OnStop stops the reactor by signaling to all spawned goroutines to exit and
// blocking until they all exit.
func (r *Reactor) OnStop() {
	// tell the dispatcher to stop sending any more requests
	r.dispatcher.stop()

	// Close closeCh to signal to all spawned goroutines to gracefully exit. All
	// p2p Channels should execute Close().
	close(r.closeCh)

	// Wait for all p2p Channels to be closed before returning. This ensures we
	// can easily reason about synchronization of all p2p Channels and ensure no
	// panics will occur.
	<-r.snapshotCh.Done()
	<-r.chunkCh.Done()
	<-r.blockCh.Done()
	<-r.peerUpdates.Done()
}

// Sync runs a state sync, fetching snapshots and providing chunks to the
// application. It also saves tendermint state and runs a backfill process to
// retrieve the necessary amount of headers, commits and validators sets to be
// able to process evidence and participate in consensus.
func (r *Reactor) Sync(stateProvider StateProvider, discoveryTime time.Duration) (sm.State, error) {
	r.mtx.Lock()
	if r.syncer != nil {
		r.mtx.Unlock()
		return sm.State{}, errors.New("a state sync is already in progress")
	}

	r.syncer = newSyncer(
		r.cfg,
		r.Logger,
		r.conn,
		r.connQuery,
		stateProvider,
		r.snapshotCh.Out,
		r.chunkCh.Out,
		r.tempDir,
	)
	r.mtx.Unlock()

	hook := func() {
		// request snapshots from all currently connected peers
		r.Logger.Debug("requesting snapshots from known peers")
		r.snapshotCh.Out <- p2p.Envelope{
			Broadcast: true,
			Message:   &ssproto.SnapshotsRequest{},
		}
	}

	hook()

	state, commit, err := r.syncer.SyncAny(discoveryTime, hook)
	if err != nil {
		return sm.State{}, err
	}

	r.mtx.Lock()
	r.syncer = nil
	r.mtx.Unlock()

	err = r.stateStore.Bootstrap(state)
	if err != nil {
		return sm.State{}, fmt.Errorf("failed to bootstrap node with new state: %w", err)
	}

	err = r.blockStore.SaveSeenCommit(state.LastBlockHeight, commit)
	if err != nil {
		return sm.State{}, fmt.Errorf("failed to store last seen commit: %w", err)
	}

	return state, nil
}

// Backfill sequentially fetches, verifies and stores light blocks in reverse
// order. It does not stop verifying blocks until reaching a block with a height
// and time that is less or equal to the stopHeight and stopTime. The
// trustedBlockID should be of the header at startHeight.
func (r *Reactor) Backfill(state sm.State) error {
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
		context.Background(),
		state.ChainID,
		state.LastBlockHeight, stopHeight,
		state.LastBlockID,
		stopTime,
	)
}

func (r *Reactor) backfill(
	ctx context.Context,
	chainID string,
	startHeight, stopHeight int64,
	trustedBlockID types.BlockID,
	stopTime time.Time,
) error {
	r.Logger.Info("starting backfill process...", "startHeight", startHeight,
		"stopHeight", stopHeight, "trustedBlockID", trustedBlockID)

	const sleepTime = 1 * time.Second
	var (
		lastValidatorSet *types.ValidatorSet
		lastChangeHeight int64 = startHeight
	)

	queue := newBlockQueue(startHeight, stopHeight, stopTime, maxLightBlockRequestRetries)

	// fetch light blocks across four workers. The aim with deploying concurrent
	// workers is to equate the network messaging time with the verification
	// time. Ideally we want the verification process to never have to be
	// waiting on blocks. If it takes 4s to retrieve a block and 1s to verify
	// it, then steady state involves four workers.
	for i := 0; i < lightBlockFetchers; i++ {
		go func() {
			for {
				select {
				case height := <-queue.nextHeight():
					r.Logger.Debug("fetching next block", "height", height)
					lb, peer, err := r.dispatcher.LightBlock(ctx, height)
					if err != nil {
						queue.retry(height)
						if errors.Is(err, errNoConnectedPeers) {
							r.Logger.Info("backfill: no connected peers to fetch light blocks from; sleeping...",
								"sleepTime", sleepTime)
							time.Sleep(sleepTime)
						} else {
							// we don't punish the peer as it might just have not responded in time
							r.Logger.Info("backfill: error with fetching light block",
								"height", height, "err", err)
						}
						continue
					}
					if lb == nil {
						r.Logger.Info("backfill: peer didn't have block, fetching from another peer", "height", height)
						queue.retry(height)
						// as we are fetching blocks backwards, if this node doesn't have the block it likely doesn't
						// have any prior ones, thus we remove it from the peer list
						r.dispatcher.removePeer(peer)
						continue
					}

					// run a validate basic. This checks the validator set and commit
					// hashes line up
					err = lb.ValidateBasic(chainID)
					if err != nil || lb.Height != height {
						r.Logger.Info("backfill: fetched light block failed validate basic, removing peer...",
							"err", err, "height", height)
						queue.retry(height)
						r.blockCh.Error <- p2p.PeerError{
							NodeID: peer,
							Err:    fmt.Errorf("received invalid light block: %w", err),
						}
						continue
					}

					// add block to queue to be verified
					queue.add(lightBlockResponse{
						block: lb,
						peer:  peer,
					})
					r.Logger.Debug("backfill: added light block to processing queue", "height", height)

				case <-queue.done():
					return
				}
			}
		}()
	}

	// verify all light blocks
	for {
		select {
		case <-r.closeCh:
			queue.close()
			return nil
		case <-ctx.Done():
			queue.close()
			return nil
		case resp := <-queue.verifyNext():
			// validate the header hash. We take the last block id of the
			// previous header (i.e. one height above) as the trusted hash which
			// we equate to. ValidatorsHash and CommitHash have already been
			// checked in the `ValidateBasic`
			if w, g := trustedBlockID.Hash, resp.block.Hash(); !bytes.Equal(w, g) {
				r.Logger.Info("received invalid light block. header hash doesn't match trusted LastBlockID",
					"trustedHash", w, "receivedHash", g, "height", resp.block.Height)
				r.blockCh.Error <- p2p.PeerError{
					NodeID: resp.peer,
					Err:    fmt.Errorf("received invalid light block. Expected hash %v, got: %v", w, g),
				}
				queue.retry(resp.block.Height)
				continue
			}

			// save the signed headers
			err := r.blockStore.SaveSignedHeader(resp.block.SignedHeader, trustedBlockID)
			if err != nil {
				return err
			}

			// check if there has been a change in the validator set
			if lastValidatorSet != nil && !bytes.Equal(resp.block.Header.ValidatorsHash, resp.block.Header.NextValidatorsHash) {
				// save all the heights that the last validator set was the same
				err = r.stateStore.SaveValidatorSets(resp.block.Height+1, lastChangeHeight, lastValidatorSet)
				if err != nil {
					return err
				}

				// update the lastChangeHeight
				lastChangeHeight = resp.block.Height
			}

			trustedBlockID = resp.block.LastBlockID
			queue.success(resp.block.Height)
			r.Logger.Info("backfill: verified and stored light block", "height", resp.block.Height)

			lastValidatorSet = resp.block.ValidatorSet

		case <-queue.done():
			if err := queue.error(); err != nil {
				return err
			}

			// save the final batch of validators
			if err := r.stateStore.SaveValidatorSets(queue.terminal.Height, lastChangeHeight, lastValidatorSet); err != nil {
				return err
			}

			r.Logger.Info("successfully completed backfill process", "endHeight", queue.terminal.Height)
			return nil
		}
	}
}

// Dispatcher exposes the dispatcher so that a state provider can use it for
// light client verification
func (r *Reactor) Dispatcher() *dispatcher { //nolint:golint
	return r.dispatcher
}

// handleSnapshotMessage handles envelopes sent from peers on the
// SnapshotChannel. It returns an error only if the Envelope.Message is unknown
// for this channel. This should never be called outside of handleMessage.
func (r *Reactor) handleSnapshotMessage(envelope p2p.Envelope) error {
	logger := r.Logger.With("peer", envelope.From)

	switch msg := envelope.Message.(type) {
	case *ssproto.SnapshotsRequest:
		snapshots, err := r.recentSnapshots(recentSnapshots)
		if err != nil {
			logger.Error("failed to fetch snapshots", "err", err)
			return nil
		}

		for _, snapshot := range snapshots {
			logger.Debug(
				"advertising snapshot",
				"height", snapshot.Height,
				"format", snapshot.Format,
			)
			r.snapshotCh.Out <- p2p.Envelope{
				To: envelope.From,
				Message: &ssproto.SnapshotsResponse{
					Height:   snapshot.Height,
					Format:   snapshot.Format,
					Chunks:   snapshot.Chunks,
					Hash:     snapshot.Hash,
					Metadata: snapshot.Metadata,
				},
			}
		}

	case *ssproto.SnapshotsResponse:
		r.mtx.RLock()
		defer r.mtx.RUnlock()

		if r.syncer == nil {
			logger.Debug("received unexpected snapshot; no state sync in progress")
			return nil
		}

		logger.Debug("received snapshot", "height", msg.Height, "format", msg.Format)
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
				"err", err,
				"channel", r.snapshotCh.ID,
			)
			return nil
		}

	default:
		return fmt.Errorf("received unknown message: %T", msg)
	}

	return nil
}

// handleChunkMessage handles envelopes sent from peers on the ChunkChannel.
// It returns an error only if the Envelope.Message is unknown for this channel.
// This should never be called outside of handleMessage.
func (r *Reactor) handleChunkMessage(envelope p2p.Envelope) error {
	switch msg := envelope.Message.(type) {
	case *ssproto.ChunkRequest:
		r.Logger.Debug(
			"received chunk request",
			"height", msg.Height,
			"format", msg.Format,
			"chunk", msg.Index,
			"peer", envelope.From,
		)
		resp, err := r.conn.LoadSnapshotChunkSync(context.Background(), abci.RequestLoadSnapshotChunk{
			Height: msg.Height,
			Format: msg.Format,
			Chunk:  msg.Index,
		})
		if err != nil {
			r.Logger.Error(
				"failed to load chunk",
				"height", msg.Height,
				"format", msg.Format,
				"chunk", msg.Index,
				"err", err,
				"peer", envelope.From,
			)
			return nil
		}

		r.Logger.Debug(
			"sending chunk",
			"height", msg.Height,
			"format", msg.Format,
			"chunk", msg.Index,
			"peer", envelope.From,
		)
		r.chunkCh.Out <- p2p.Envelope{
			To: envelope.From,
			Message: &ssproto.ChunkResponse{
				Height:  msg.Height,
				Format:  msg.Format,
				Index:   msg.Index,
				Chunk:   resp.Chunk,
				Missing: resp.Chunk == nil,
			},
		}

	case *ssproto.ChunkResponse:
		r.mtx.RLock()
		defer r.mtx.RUnlock()

		if r.syncer == nil {
			r.Logger.Debug("received unexpected chunk; no state sync in progress", "peer", envelope.From)
			return nil
		}

		r.Logger.Debug(
			"received chunk; adding to sync",
			"height", msg.Height,
			"format", msg.Format,
			"chunk", msg.Index,
			"peer", envelope.From,
		)
		_, err := r.syncer.AddChunk(&chunk{
			Height: msg.Height,
			Format: msg.Format,
			Index:  msg.Index,
			Chunk:  msg.Chunk,
			Sender: envelope.From,
		})
		if err != nil {
			r.Logger.Error(
				"failed to add chunk",
				"height", msg.Height,
				"format", msg.Format,
				"chunk", msg.Index,
				"err", err,
				"peer", envelope.From,
			)
			return nil
		}

	default:
		return fmt.Errorf("received unknown message: %T", msg)
	}

	return nil
}

func (r *Reactor) handleLightBlockMessage(envelope p2p.Envelope) error {
	switch msg := envelope.Message.(type) {
	case *ssproto.LightBlockRequest:
		r.Logger.Info("received light block request", "height", msg.Height)
		lb, err := r.fetchLightBlock(msg.Height)
		if err != nil {
			r.Logger.Error("failed to retrieve light block", "err", err, "height", msg.Height)
			return err
		}

		lbproto, err := lb.ToProto()
		if err != nil {
			r.Logger.Error("marshaling light block to proto", "err", err)
			return nil
		}

		// NOTE: If we don't have the light block we will send a nil light block
		// back to the requested node, indicating that we don't have it.
		r.blockCh.Out <- p2p.Envelope{
			To: envelope.From,
			Message: &ssproto.LightBlockResponse{
				LightBlock: lbproto,
			},
		}

	case *ssproto.LightBlockResponse:
		if err := r.dispatcher.respond(msg.LightBlock, envelope.From); err != nil {
			r.Logger.Error("error processing light block response", "err", err)
			return err
		}

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
		}
	}()

	r.Logger.Debug("received message", "message", reflect.TypeOf(envelope.Message), "peer", envelope.From)

	switch chID {
	case SnapshotChannel:
		err = r.handleSnapshotMessage(envelope)

	case ChunkChannel:
		err = r.handleChunkMessage(envelope)

	case LightBlockChannel:
		err = r.handleLightBlockMessage(envelope)

	default:
		err = fmt.Errorf("unknown channel ID (%d) for envelope (%v)", chID, envelope)
	}

	return err
}

// processSnapshotCh initiates a blocking process where we listen for and handle
// envelopes on the SnapshotChannel.
func (r *Reactor) processSnapshotCh() {
	r.processCh(r.snapshotCh, "snapshot")
}

// processChunkCh initiates a blocking process where we listen for and handle
// envelopes on the ChunkChannel.
func (r *Reactor) processChunkCh() {
	r.processCh(r.chunkCh, "chunk")
}

// processBlockCh initiates a blocking process where we listen for and handle
// envelopes on the LightBlockChannel.
func (r *Reactor) processBlockCh() {
	r.processCh(r.blockCh, "light block")
}

// processCh routes state sync messages to their respective handlers. Any error
// encountered during message execution will result in a PeerError being sent on
// the respective channel. When the reactor is stopped, we will catch the signal
// and close the p2p Channel gracefully.
func (r *Reactor) processCh(ch *p2p.Channel, chName string) {
	defer ch.Close()

	for {
		select {
		case envelope := <-ch.In:
			if err := r.handleMessage(ch.ID, envelope); err != nil {
				r.Logger.Error(fmt.Sprintf("failed to process %s message", chName),
					"ch_id", ch.ID, "envelope", envelope, "err", err)
				ch.Error <- p2p.PeerError{
					NodeID: envelope.From,
					Err:    err,
				}
			}

		case <-r.closeCh:
			r.Logger.Debug(fmt.Sprintf("stopped listening on %s channel; closing...", chName))
			return
		}
	}
}

// processPeerUpdate processes a PeerUpdate, returning an error upon failing to
// handle the PeerUpdate or if a panic is recovered.
func (r *Reactor) processPeerUpdate(peerUpdate p2p.PeerUpdate) {
	r.Logger.Debug("received peer update", "peer", peerUpdate.NodeID, "status", peerUpdate.Status)

	r.mtx.RLock()
	defer r.mtx.RUnlock()

	switch peerUpdate.Status {
	case p2p.PeerStatusUp:
		if r.syncer != nil {
			r.syncer.AddPeer(peerUpdate.NodeID)
		}
		r.dispatcher.addPeer(peerUpdate.NodeID)

	case p2p.PeerStatusDown:
		if r.syncer != nil {
			r.syncer.RemovePeer(peerUpdate.NodeID)
		}
		r.dispatcher.removePeer(peerUpdate.NodeID)
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

// recentSnapshots fetches the n most recent snapshots from the app
func (r *Reactor) recentSnapshots(n uint32) ([]*snapshot, error) {
	resp, err := r.conn.ListSnapshotsSync(context.Background(), abci.RequestListSnapshots{})
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
