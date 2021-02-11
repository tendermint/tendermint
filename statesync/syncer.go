package statesync

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"time"

	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/libs/log"
	tmsync "github.com/tendermint/tendermint/libs/sync"
	"github.com/tendermint/tendermint/p2p"
	ssproto "github.com/tendermint/tendermint/proto/tendermint/statesync"
	"github.com/tendermint/tendermint/proxy"
	sm "github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/types"
)

const (
	// chunkFetchers is the number of concurrent chunk fetchers to run.
	chunkFetchers = 4
	// chunkTimeout is the timeout while waiting for the next chunk from the chunk queue.
	chunkTimeout = 2 * time.Minute
	// requestTimeout is the timeout before rerequesting a chunk, possibly from a different peer.
	chunkRequestTimeout = 10 * time.Second
)

var (
	// errAbort is returned by Sync() when snapshot restoration is aborted.
	errAbort = errors.New("state sync aborted")
	// errRetrySnapshot is returned by Sync() when the snapshot should be retried.
	errRetrySnapshot = errors.New("retry snapshot")
	// errRejectSnapshot is returned by Sync() when the snapshot is rejected.
	errRejectSnapshot = errors.New("snapshot was rejected")
	// errRejectFormat is returned by Sync() when the snapshot format is rejected.
	errRejectFormat = errors.New("snapshot format was rejected")
	// errRejectSender is returned by Sync() when the snapshot sender is rejected.
	errRejectSender = errors.New("snapshot sender was rejected")
	// errVerifyFailed is returned by Sync() when app hash or last height verification fails.
	errVerifyFailed = errors.New("verification failed")
	// errTimeout is returned by Sync() when we've waited too long to receive a chunk.
	errTimeout = errors.New("timed out waiting for chunk")
	// errNoSnapshots is returned by SyncAny() if no snapshots are found and discovery is disabled.
	errNoSnapshots = errors.New("no suitable snapshots found")
)

// syncer runs a state sync against an ABCI app. Use either SyncAny() to automatically attempt to
// sync all snapshots in the pool (pausing to discover new ones), or Sync() to sync a specific
// snapshot. Snapshots and chunks are fed via AddSnapshot() and AddChunk() as appropriate.
type syncer struct {
	logger        log.Logger
	stateProvider StateProvider
	conn          proxy.AppConnSnapshot
	connQuery     proxy.AppConnQuery
	snapshots     *snapshotPool
	snapshotCh    chan<- p2p.Envelope
	chunkCh       chan<- p2p.Envelope
	tempDir       string

	mtx    tmsync.RWMutex
	chunks *chunkQueue
}

// newSyncer creates a new syncer.
func newSyncer(
	logger log.Logger,
	conn proxy.AppConnSnapshot,
	connQuery proxy.AppConnQuery,
	stateProvider StateProvider,
	snapshotCh, chunkCh chan<- p2p.Envelope,
	tempDir string,
) *syncer {
	return &syncer{
		logger:        logger,
		stateProvider: stateProvider,
		conn:          conn,
		connQuery:     connQuery,
		snapshots:     newSnapshotPool(stateProvider),
		snapshotCh:    snapshotCh,
		chunkCh:       chunkCh,
		tempDir:       tempDir,
	}
}

// AddChunk adds a chunk to the chunk queue, if any. It returns false if the chunk has already
// been added to the queue, or an error if there's no sync in progress.
func (s *syncer) AddChunk(chunk *chunk) (bool, error) {
	s.mtx.RLock()
	defer s.mtx.RUnlock()
	if s.chunks == nil {
		return false, errors.New("no state sync in progress")
	}
	added, err := s.chunks.Add(chunk)
	if err != nil {
		return false, err
	}
	if added {
		s.logger.Debug("Added chunk to queue", "height", chunk.Height, "format", chunk.Format,
			"chunk", chunk.Index)
	} else {
		s.logger.Debug("Ignoring duplicate chunk in queue", "height", chunk.Height, "format", chunk.Format,
			"chunk", chunk.Index)
	}
	return added, nil
}

// AddSnapshot adds a snapshot to the snapshot pool. It returns true if a new, previously unseen
// snapshot was accepted and added.
func (s *syncer) AddSnapshot(peerID p2p.NodeID, snapshot *snapshot) (bool, error) {
	added, err := s.snapshots.Add(peerID, snapshot)
	if err != nil {
		return false, err
	}
	if added {
		s.logger.Info("Discovered new snapshot", "height", snapshot.Height, "format", snapshot.Format,
			"hash", snapshot.Hash)
	}
	return added, nil
}

// AddPeer adds a peer to the pool. For now we just keep it simple and send a
// single request to discover snapshots, later we may want to do retries and stuff.
func (s *syncer) AddPeer(peerID p2p.NodeID) {
	s.logger.Debug("Requesting snapshots from peer", "peer", peerID)
	s.snapshotCh <- p2p.Envelope{
		To:      peerID,
		Message: &ssproto.SnapshotsRequest{},
	}
}

// RemovePeer removes a peer from the pool.
func (s *syncer) RemovePeer(peerID p2p.NodeID) {
	s.logger.Debug("Removing peer from sync", "peer", peerID)
	s.snapshots.RemovePeer(peerID)
}

// SyncAny tries to sync any of the snapshots in the snapshot pool, waiting to discover further
// snapshots if none were found and discoveryTime > 0. It returns the latest state and block commit
// which the caller must use to bootstrap the node.
func (s *syncer) SyncAny(discoveryTime time.Duration) (sm.State, *types.Commit, error) {
	if discoveryTime > 0 {
		s.logger.Info(fmt.Sprintf("Discovering snapshots for %v", discoveryTime))
		time.Sleep(discoveryTime)
	}

	// The app may ask us to retry a snapshot restoration, in which case we need to reuse
	// the snapshot and chunk queue from the previous loop iteration.
	var (
		snapshot *snapshot
		chunks   *chunkQueue
		err      error
	)
	for {
		// If not nil, we're going to retry restoration of the same snapshot.
		if snapshot == nil {
			snapshot = s.snapshots.Best()
			chunks = nil
		}
		if snapshot == nil {
			if discoveryTime == 0 {
				return sm.State{}, nil, errNoSnapshots
			}
			s.logger.Info(fmt.Sprintf("Discovering snapshots for %v", discoveryTime))
			time.Sleep(discoveryTime)
			continue
		}
		if chunks == nil {
			chunks, err = newChunkQueue(snapshot, s.tempDir)
			if err != nil {
				return sm.State{}, nil, fmt.Errorf("failed to create chunk queue: %w", err)
			}
			defer chunks.Close() // in case we forget to close it elsewhere
		}

		newState, commit, err := s.Sync(snapshot, chunks)
		switch {
		case err == nil:
			return newState, commit, nil

		case errors.Is(err, errAbort):
			return sm.State{}, nil, err

		case errors.Is(err, errRetrySnapshot):
			chunks.RetryAll()
			s.logger.Info("Retrying snapshot", "height", snapshot.Height, "format", snapshot.Format,
				"hash", snapshot.Hash)
			continue

		case errors.Is(err, errTimeout):
			s.snapshots.Reject(snapshot)
			s.logger.Error("Timed out waiting for snapshot chunks, rejected snapshot",
				"height", snapshot.Height, "format", snapshot.Format, "hash", snapshot.Hash)

		case errors.Is(err, errRejectSnapshot):
			s.snapshots.Reject(snapshot)
			s.logger.Info("Snapshot rejected", "height", snapshot.Height, "format", snapshot.Format,
				"hash", snapshot.Hash)

		case errors.Is(err, errRejectFormat):
			s.snapshots.RejectFormat(snapshot.Format)
			s.logger.Info("Snapshot format rejected", "format", snapshot.Format)

		case errors.Is(err, errRejectSender):
			s.logger.Info("Snapshot senders rejected", "height", snapshot.Height, "format", snapshot.Format,
				"hash", snapshot.Hash)
			for _, peer := range s.snapshots.GetPeers(snapshot) {
				s.snapshots.RejectPeer(peer)
				s.logger.Info("Snapshot sender rejected", "peer", peer)
			}

		default:
			return sm.State{}, nil, fmt.Errorf("snapshot restoration failed: %w", err)
		}

		// Discard snapshot and chunks for next iteration
		err = chunks.Close()
		if err != nil {
			s.logger.Error("Failed to clean up chunk queue", "err", err)
		}
		snapshot = nil
		chunks = nil
	}
}

// Sync executes a sync for a specific snapshot, returning the latest state and block commit which
// the caller must use to bootstrap the node.
func (s *syncer) Sync(snapshot *snapshot, chunks *chunkQueue) (sm.State, *types.Commit, error) {
	s.mtx.Lock()
	if s.chunks != nil {
		s.mtx.Unlock()
		return sm.State{}, nil, errors.New("a state sync is already in progress")
	}
	s.chunks = chunks
	s.mtx.Unlock()
	defer func() {
		s.mtx.Lock()
		s.chunks = nil
		s.mtx.Unlock()
	}()

	// Offer snapshot to ABCI app.
	err := s.offerSnapshot(snapshot)
	if err != nil {
		return sm.State{}, nil, err
	}

	// Spawn chunk fetchers. They will terminate when the chunk queue is closed or context canceled.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	for i := int32(0); i < chunkFetchers; i++ {
		go s.fetchChunks(ctx, snapshot, chunks)
	}

	pctx, pcancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer pcancel()

	// Optimistically build new state, so we don't discover any light client failures at the end.
	state, err := s.stateProvider.State(pctx, snapshot.Height)
	if err != nil {
		return sm.State{}, nil, fmt.Errorf("failed to build new state: %w", err)
	}
	commit, err := s.stateProvider.Commit(pctx, snapshot.Height)
	if err != nil {
		return sm.State{}, nil, fmt.Errorf("failed to fetch commit: %w", err)
	}

	// Restore snapshot
	err = s.applyChunks(chunks)
	if err != nil {
		return sm.State{}, nil, err
	}

	// Verify app and update app version
	appVersion, err := s.verifyApp(snapshot)
	if err != nil {
		return sm.State{}, nil, err
	}
	state.Version.Consensus.App = appVersion

	// Done! 🎉
	s.logger.Info("Snapshot restored", "height", snapshot.Height, "format", snapshot.Format,
		"hash", snapshot.Hash)

	return state, commit, nil
}

// offerSnapshot offers a snapshot to the app. It returns various errors depending on the app's
// response, or nil if the snapshot was accepted.
func (s *syncer) offerSnapshot(snapshot *snapshot) error {
	s.logger.Info("Offering snapshot to ABCI app", "height", snapshot.Height,
		"format", snapshot.Format, "hash", snapshot.Hash)
	resp, err := s.conn.OfferSnapshotSync(context.Background(), abci.RequestOfferSnapshot{
		Snapshot: &abci.Snapshot{
			Height:   snapshot.Height,
			Format:   snapshot.Format,
			Chunks:   snapshot.Chunks,
			Hash:     snapshot.Hash,
			Metadata: snapshot.Metadata,
		},
		AppHash: snapshot.trustedAppHash,
	})
	if err != nil {
		return fmt.Errorf("failed to offer snapshot: %w", err)
	}
	switch resp.Result {
	case abci.ResponseOfferSnapshot_ACCEPT:
		s.logger.Info("Snapshot accepted, restoring", "height", snapshot.Height,
			"format", snapshot.Format, "hash", snapshot.Hash)
		return nil
	case abci.ResponseOfferSnapshot_ABORT:
		return errAbort
	case abci.ResponseOfferSnapshot_REJECT:
		return errRejectSnapshot
	case abci.ResponseOfferSnapshot_REJECT_FORMAT:
		return errRejectFormat
	case abci.ResponseOfferSnapshot_REJECT_SENDER:
		return errRejectSender
	default:
		return fmt.Errorf("unknown ResponseOfferSnapshot result %v", resp.Result)
	}
}

// applyChunks applies chunks to the app. It returns various errors depending on the app's
// response, or nil once the snapshot is fully restored.
func (s *syncer) applyChunks(chunks *chunkQueue) error {
	for {
		chunk, err := chunks.Next()
		if err == errDone {
			return nil
		} else if err != nil {
			return fmt.Errorf("failed to fetch chunk: %w", err)
		}

		resp, err := s.conn.ApplySnapshotChunkSync(context.Background(), abci.RequestApplySnapshotChunk{
			Index:  chunk.Index,
			Chunk:  chunk.Chunk,
			Sender: string(chunk.Sender),
		})
		if err != nil {
			return fmt.Errorf("failed to apply chunk %v: %w", chunk.Index, err)
		}
		s.logger.Info("Applied snapshot chunk to ABCI app", "height", chunk.Height,
			"format", chunk.Format, "chunk", chunk.Index, "total", chunks.Size())

		// Discard and refetch any chunks as requested by the app
		for _, index := range resp.RefetchChunks {
			err := chunks.Discard(index)
			if err != nil {
				return fmt.Errorf("failed to discard chunk %v: %w", index, err)
			}
		}

		// Reject any senders as requested by the app
		for _, sender := range resp.RejectSenders {
			if sender != "" {
				peerID := p2p.NodeID(sender)
				s.snapshots.RejectPeer(peerID)

				if err := chunks.DiscardSender(peerID); err != nil {
					return fmt.Errorf("failed to reject sender: %w", err)
				}
			}
		}

		switch resp.Result {
		case abci.ResponseApplySnapshotChunk_ACCEPT:
		case abci.ResponseApplySnapshotChunk_ABORT:
			return errAbort
		case abci.ResponseApplySnapshotChunk_RETRY:
			chunks.Retry(chunk.Index)
		case abci.ResponseApplySnapshotChunk_RETRY_SNAPSHOT:
			return errRetrySnapshot
		case abci.ResponseApplySnapshotChunk_REJECT_SNAPSHOT:
			return errRejectSnapshot
		default:
			return fmt.Errorf("unknown ResponseApplySnapshotChunk result %v", resp.Result)
		}
	}
}

// fetchChunks requests chunks from peers, receiving allocations from the chunk queue. Chunks
// will be received from the reactor via syncer.AddChunks() to chunkQueue.Add().
func (s *syncer) fetchChunks(ctx context.Context, snapshot *snapshot, chunks *chunkQueue) {
	for {
		index, err := chunks.Allocate()
		if err == errDone {
			// Keep checking until the context is canceled (restore is done), in case any
			// chunks need to be refetched.
			select {
			case <-ctx.Done():
				return
			default:
			}
			time.Sleep(2 * time.Second)
			continue
		}
		if err != nil {
			s.logger.Error("Failed to allocate chunk from queue", "err", err)
			return
		}
		s.logger.Info("Fetching snapshot chunk", "height", snapshot.Height,
			"format", snapshot.Format, "chunk", index, "total", chunks.Size())

		ticker := time.NewTicker(chunkRequestTimeout)
		defer ticker.Stop()
		s.requestChunk(snapshot, index)
		select {
		case <-chunks.WaitFor(index):
		case <-ticker.C:
			s.requestChunk(snapshot, index)
		case <-ctx.Done():
			return
		}
		ticker.Stop()
	}
}

// requestChunk requests a chunk from a peer.
func (s *syncer) requestChunk(snapshot *snapshot, chunk uint32) {
	peer := s.snapshots.GetPeer(snapshot)
	if peer == "" {
		s.logger.Error("No valid peers found for snapshot", "height", snapshot.Height,
			"format", snapshot.Format, "hash", snapshot.Hash)
		return
	}

	s.logger.Debug(
		"Requesting snapshot chunk",
		"height", snapshot.Height,
		"format", snapshot.Format,
		"chunk", chunk,
		"peer", peer,
	)

	s.chunkCh <- p2p.Envelope{
		To: peer,
		Message: &ssproto.ChunkRequest{
			Height: snapshot.Height,
			Format: snapshot.Format,
			Index:  chunk,
		},
	}
}

// verifyApp verifies the sync, checking the app hash and last block height. It returns the
// app version, which should be returned as part of the initial state.
func (s *syncer) verifyApp(snapshot *snapshot) (uint64, error) {
	resp, err := s.connQuery.InfoSync(context.Background(), proxy.RequestInfo)
	if err != nil {
		return 0, fmt.Errorf("failed to query ABCI app for appHash: %w", err)
	}

	if !bytes.Equal(snapshot.trustedAppHash, resp.LastBlockAppHash) {
		s.logger.Error("appHash verification failed",
			"expected", snapshot.trustedAppHash,
			"actual", resp.LastBlockAppHash)
		return 0, errVerifyFailed
	}

	if uint64(resp.LastBlockHeight) != snapshot.Height {
		s.logger.Error(
			"ABCI app reported unexpected last block height",
			"expected", snapshot.Height,
			"actual", resp.LastBlockHeight,
		)
		return 0, errVerifyFailed
	}

	s.logger.Info("Verified ABCI app", "height", snapshot.Height, "appHash", snapshot.trustedAppHash)
	return resp.AppVersion, nil
}
