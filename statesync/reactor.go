package statesync

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/libs/service"
	tmsync "github.com/tendermint/tendermint/libs/sync"
	"github.com/tendermint/tendermint/p2p"
	ssproto "github.com/tendermint/tendermint/proto/tendermint/statesync"
	"github.com/tendermint/tendermint/proxy"
	sm "github.com/tendermint/tendermint/state"
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
			},
		},
		ChunkChannel: {
			MsgType: new(ssproto.Message),
			Descriptor: &p2p.ChannelDescriptor{
				ID:                  byte(ChunkChannel),
				Priority:            1,
				SendQueueCapacity:   4,
				RecvMessageCapacity: chunkMsgSize,
			},
		},
	}
)

const (
	// SnapshotChannel exchanges snapshot metadata
	SnapshotChannel = p2p.ChannelID(0x60)

	// ChunkChannel exchanges chunk contents
	ChunkChannel = p2p.ChannelID(0x61)

	// recentSnapshots is the number of recent snapshots to send and receive per peer.
	recentSnapshots = 10

	// snapshotMsgSize is the maximum size of a snapshotResponseMessage
	snapshotMsgSize = int(4e6)

	// chunkMsgSize is the maximum size of a chunkResponseMessage
	chunkMsgSize = int(16e6)
)

// Reactor handles state sync, both restoring snapshots for the local node and
// serving snapshots for other nodes.
type Reactor struct {
	service.BaseService

	conn        proxy.AppConnSnapshot
	connQuery   proxy.AppConnQuery
	tempDir     string
	snapshotCh  *p2p.Channel
	chunkCh     *p2p.Channel
	peerUpdates *p2p.PeerUpdates
	closeCh     chan struct{}

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
	logger log.Logger,
	conn proxy.AppConnSnapshot,
	connQuery proxy.AppConnQuery,
	snapshotCh, chunkCh *p2p.Channel,
	peerUpdates *p2p.PeerUpdates,
	tempDir string,
) *Reactor {
	r := &Reactor{
		conn:        conn,
		connQuery:   connQuery,
		snapshotCh:  snapshotCh,
		chunkCh:     chunkCh,
		peerUpdates: peerUpdates,
		closeCh:     make(chan struct{}),
		tempDir:     tempDir,
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

	go r.processPeerUpdates()

	return nil
}

// OnStop stops the reactor by signaling to all spawned goroutines to exit and
// blocking until they all exit.
func (r *Reactor) OnStop() {
	// Close closeCh to signal to all spawned goroutines to gracefully exit. All
	// p2p Channels should execute Close().
	close(r.closeCh)

	// Wait for all p2p Channels to be closed before returning. This ensures we
	// can easily reason about synchronization of all p2p Channels and ensure no
	// panics will occur.
	<-r.snapshotCh.Done()
	<-r.chunkCh.Done()
	<-r.peerUpdates.Done()
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

// handleMessage handles an Envelope sent from a peer on a specific p2p Channel.
// It will handle errors and any possible panics gracefully. A caller can handle
// any error returned by sending a PeerError on the respective channel.
func (r *Reactor) handleMessage(chID p2p.ChannelID, envelope p2p.Envelope) (err error) {
	defer func() {
		if e := recover(); e != nil {
			err = fmt.Errorf("panic in processing message: %v", e)
		}
	}()

	r.Logger.Debug("received message", "message", envelope.Message, "peer", envelope.From)

	switch chID {
	case SnapshotChannel:
		err = r.handleSnapshotMessage(envelope)

	case ChunkChannel:
		err = r.handleChunkMessage(envelope)

	default:
		err = fmt.Errorf("unknown channel ID (%d) for envelope (%v)", chID, envelope)
	}

	return err
}

// processSnapshotCh initiates a blocking process where we listen for and handle
// envelopes on the SnapshotChannel. Any error encountered during message
// execution will result in a PeerError being sent on the SnapshotChannel. When
// the reactor is stopped, we will catch the signal and close the p2p Channel
// gracefully.
func (r *Reactor) processSnapshotCh() {
	defer r.snapshotCh.Close()

	for {
		select {
		case envelope := <-r.snapshotCh.In:
			if err := r.handleMessage(r.snapshotCh.ID, envelope); err != nil {
				r.Logger.Error("failed to process message", "ch_id", r.snapshotCh.ID, "envelope", envelope, "err", err)
				r.snapshotCh.Error <- p2p.PeerError{
					NodeID: envelope.From,
					Err:    err,
				}
			}

		case <-r.closeCh:
			r.Logger.Debug("stopped listening on snapshot channel; closing...")
			return
		}
	}
}

// processChunkCh initiates a blocking process where we listen for and handle
// envelopes on the ChunkChannel. Any error encountered during message
// execution will result in a PeerError being sent on the ChunkChannel. When
// the reactor is stopped, we will catch the signal and close the p2p Channel
// gracefully.
func (r *Reactor) processChunkCh() {
	defer r.chunkCh.Close()

	for {
		select {
		case envelope := <-r.chunkCh.In:
			if err := r.handleMessage(r.chunkCh.ID, envelope); err != nil {
				r.Logger.Error("failed to process message", "ch_id", r.chunkCh.ID, "envelope", envelope, "err", err)
				r.chunkCh.Error <- p2p.PeerError{
					NodeID: envelope.From,
					Err:    err,
				}
			}

		case <-r.closeCh:
			r.Logger.Debug("stopped listening on chunk channel; closing...")
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

	if r.syncer != nil {
		switch peerUpdate.Status {
		case p2p.PeerStatusUp:
			r.syncer.AddPeer(peerUpdate.NodeID)

		case p2p.PeerStatusDown:
			r.syncer.RemovePeer(peerUpdate.NodeID)
		}
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

// Sync runs a state sync, returning the new state and last commit at the snapshot height.
// The caller must store the state and commit in the state database and block store.
func (r *Reactor) Sync(stateProvider StateProvider, discoveryTime time.Duration) (sm.State, *types.Commit, error) {
	r.mtx.Lock()
	if r.syncer != nil {
		r.mtx.Unlock()
		return sm.State{}, nil, errors.New("a state sync is already in progress")
	}

	r.syncer = newSyncer(r.Logger, r.conn, r.connQuery, stateProvider, r.snapshotCh.Out, r.chunkCh.Out, r.tempDir)
	r.mtx.Unlock()

	// request snapshots from all currently connected peers
	r.Logger.Debug("requesting snapshots from known peers")
	r.snapshotCh.Out <- p2p.Envelope{
		Broadcast: true,
		Message:   &ssproto.SnapshotsRequest{},
	}

	state, commit, err := r.syncer.SyncAny(discoveryTime)

	r.mtx.Lock()
	r.syncer = nil
	r.mtx.Unlock()

	return state, commit, err
}
