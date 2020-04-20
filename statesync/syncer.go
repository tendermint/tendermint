package statesync

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"

	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/p2p"
	"github.com/tendermint/tendermint/proxy"
	sm "github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/types"
	"github.com/tendermint/tendermint/version"
)

const (
	// chunkFetchers is the number of concurrent chunk fetchers to run.
	chunkFetchers = 4
	// chunkTimeout is the timeout before refetching a chunk, possibly with a different peer.
	chunkTimeout = 10 * time.Second
)

// errNoSnapshots is returned by Sync() when no viable snapshots are found in the pool.
var errNoSnapshots = errors.New("no viable snapshots found")

// syncer runs a state sync against an ABCI app. Typical usage:
//
// 1) Create the syncer: newSyncer(...)
// 2) Discover snapshots, and feed them via syncer.AddSnapshot()
// 3) Start a sync via Sync(), which will request chunks from peers
// 4) Concurrently feed chunks via syncer.AddChunk()
// 5) Bootstrap node with the returned state and commit
type syncer struct {
	logger      log.Logger
	stateSource StateSource
	conn        proxy.AppConnSnapshot
	connQuery   proxy.AppConnQuery
	snapshots   *snapshotPool

	mtx      sync.RWMutex
	snapshot *snapshot
	chunks   *chunkQueue
}

// newSyncer creates a new syncer.
func newSyncer(logger log.Logger, conn proxy.AppConnSnapshot, connQuery proxy.AppConnQuery,
	stateSource StateSource) *syncer {
	return &syncer{
		logger:      logger,
		stateSource: stateSource,
		conn:        conn,
		connQuery:   connQuery,
		snapshots:   newSnapshotPool(stateSource),
	}
}

// AddChunk adds a chunk to the chunk queue, if any. It returns false if the chunk has already
// been added to the queue, or an error if there's no sync in progress. The height and format
// are used as a sanity-check, in case the reactor receives unsolicited chunks for whatever reason.
func (s *syncer) AddChunk(chunk *chunk) (bool, error) {
	s.mtx.RLock()
	defer s.mtx.RUnlock()
	if s.snapshot == nil || s.chunks == nil {
		return false, errors.New("no state sync in progress")
	}
	added, err := s.chunks.Add(chunk)
	if err != nil {
		return false, err
	}
	if added {
		s.logger.Debug("Received chunk", "chunk", chunk.Index)
	} else {
		s.logger.Debug("Ignoring duplicate chunk", "chunk", chunk.Index)
	}
	return added, nil
}

// AddSnapshot adds a snapshot to the snapshot pool. It returns true if a new, previously unseen
// snapshot was accepted and added.
func (s *syncer) AddSnapshot(peer p2p.Peer, snapshot *snapshot) (bool, error) {
	added, err := s.snapshots.Add(peer, snapshot)
	if err != nil {
		return false, err
	}
	if added {
		s.logger.Info("Discovered new snapshot", "height", snapshot.Height, "format", snapshot.Format)
	}
	return added, nil
}

// AddPeer adds a peer to the sync. For now we just keep it simple and send a single request
// for snapshots, later we may want to do retries and stuff.
func (s *syncer) AddPeer(peer p2p.Peer) {
	s.logger.Debug("Requesting snapshots from peer", "peer", peer.ID())
	peer.Send(SnapshotChannel, cdc.MustMarshalBinaryBare(&snapshotsRequestMessage{}))
}

// RemovePeer removes a peer from the pool.
func (s *syncer) RemovePeer(peer p2p.Peer) {
	s.logger.Debug("Removing peer from sync", "peer", peer.ID())
	s.snapshots.RemovePeer(peer)
}

// Sync executes a sync, returning the latest state and block commit which the caller must use to
// bootstrap the node. It returns errNoSnapshots if no viable snapshots are found.
func (s *syncer) Sync() (sm.State, *types.Commit, error) {
	// Start state sync.
	snapshot, chunks, err := s.startSync()
	if err != nil {
		return sm.State{}, nil, err
	}
	defer s.endSync()

	// Spawn chunk fetchers.
	pending := make(chan uint32, len(snapshot.ChunkHashes))
	for i := uint32(0); i < uint32(len(snapshot.ChunkHashes)); i++ {
		pending <- i
	}
	for i := 0; i < chunkFetchers; i++ {
		go func() {
			for index := range pending {
				s.requestChunk(snapshot, index)
				ticker := time.NewTicker(chunkTimeout)
				select {
				case <-chunks.WaitFor(index):
				case <-ticker.C:
					s.requestChunk(snapshot, index)
				}
				ticker.Stop()
			}
		}()
	}

	// Optimistically build new state, so we don't discover any light client failures at the end.
	state, err := s.stateSource.State(snapshot.Height)
	if err != nil {
		return sm.State{}, nil, fmt.Errorf("failed to build new state: %w", err)
	}
	commit, err := s.stateSource.Commit(snapshot.Height)
	if err != nil {
		return sm.State{}, nil, fmt.Errorf("failed to fetch commit: %w", err)
	}

	// Apply chunks.
	for {
		chunk, err := chunks.Next()
		if err == errDone {
			break
		} else if err != nil {
			return sm.State{}, nil, fmt.Errorf("failed to fetch chunk: %w", err)
		}
		s.logger.Info("Applying chunk to ABCI app", "chunk", chunk.Index, "total", len(snapshot.ChunkHashes))
		resp, err := s.conn.ApplySnapshotChunkSync(abci.RequestApplySnapshotChunk{Chunk: chunk.Body})
		if err != nil {
			return sm.State{}, nil, fmt.Errorf("failed to apply chunk %v: %w", chunk.Index, err)
		}
		if !resp.Applied {
			return sm.State{}, nil, fmt.Errorf("failed to apply chunk %v", chunk.Index)
		}
	}

	// Snapshot should now be restored, verify it and update the state app version.
	resp, err := s.connQuery.InfoSync(proxy.RequestInfo)
	if err != nil {
		return sm.State{}, nil, fmt.Errorf("failed to query ABCI app for app_hash: %w", err)
	}
	if !bytes.Equal(snapshot.trustedAppHash, resp.LastBlockAppHash) {
		return sm.State{}, nil, fmt.Errorf("app_hash verification failed, expected %x got %x",
			snapshot.trustedAppHash, resp.LastBlockAppHash)
	}
	s.logger.Debug("Verified state snapshot app hash", "height", snapshot.Height,
		"format", snapshot.Format, "hash", hex.EncodeToString(resp.LastBlockAppHash))
	if uint64(resp.LastBlockHeight) != snapshot.Height {
		return sm.State{}, nil, fmt.Errorf("ABCI app reported last block height %v, expected %v",
			resp.LastBlockHeight, snapshot.Height)
	}
	state.Version.Consensus.App = version.Protocol(resp.AppVersion)

	// Done! 🎉
	s.logger.Info("Snapshot restored", "height", snapshot.Height, "format", snapshot.Format)

	return state, commit, nil
}

// startSync attempts to start a sync, if able (i.e. if any valid snapshots are available).
// It sets up a chunk queue in s.chunks, or returns errNoSnapshots if no viable snapshots were found.
func (s *syncer) startSync() (*snapshot, *chunkQueue, error) {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	if s.snapshot != nil {
		return nil, nil, errors.New("a state sync is already in progress")
	}
	var snapshot *snapshot
	for {
		snapshot = s.snapshots.Best()
		if snapshot == nil {
			return nil, nil, errNoSnapshots
		}
		s.logger.Info("Offering snapshot to ABCI app", "height", snapshot.Height, "format", snapshot.Format)
		resp, err := s.conn.OfferSnapshotSync(abci.RequestOfferSnapshot{
			Snapshot: &abci.Snapshot{
				Height:      snapshot.Height,
				Format:      snapshot.Format,
				ChunkHashes: snapshot.ChunkHashes,
				Metadata:    snapshot.Metadata,
			},
			AppHash: snapshot.trustedAppHash,
		})
		if err != nil {
			return nil, nil, fmt.Errorf("failed to offer snapshot: %w", err)
		}
		if resp.Accepted {
			s.logger.Info("Snapshot accepted, restoring", "height", snapshot.Height,
				"format", snapshot.Format, "chunks", len(snapshot.ChunkHashes))
			break
		}
		switch resp.Reason {
		case abci.ResponseOfferSnapshot_invalid_height:
			s.logger.Info("Snapshot height rejected", "height", snapshot.Height)
			s.snapshots.RejectHeight(snapshot.Height)
		case abci.ResponseOfferSnapshot_invalid_format:
			s.logger.Info("Snapshot format rejected", "format", snapshot.Format)
			s.snapshots.RejectFormat(snapshot.Format)
		default:
			s.logger.Info("Snapshot rejected", "height", snapshot.Height, "format", snapshot.Format)
			s.snapshots.Reject(snapshot)
		}
	}

	chunks, err := newChunkQueue(snapshot)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create chunk queue: %w", err)
	}
	s.snapshot = snapshot
	s.chunks = chunks
	return snapshot, chunks, nil
}

// endSync ends the sync.
func (s *syncer) endSync() {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	s.snapshot = nil
	if s.chunks != nil {
		err := s.chunks.Close()
		if err != nil {
			s.logger.Error("Failed to close syncer", "err", err)
		}
		s.chunks = nil
	}
}

// requestChunk requests a chunk from a peer.
func (s *syncer) requestChunk(snapshot *snapshot, chunk uint32) {
	peer := s.snapshots.GetPeer(snapshot)
	if peer == nil {
		s.logger.Info("No peers found for snapshot", "height", snapshot.Height, "format", snapshot.Format)
		return
	}
	s.logger.Debug("Requesting chunk", "height", snapshot.Height,
		"format", snapshot.Format, "chunk", chunk, "peer", peer.ID())
	peer.Send(ChunkChannel, cdc.MustMarshalBinaryBare(&chunkRequestMessage{
		Height: snapshot.Height,
		Format: snapshot.Format,
		Index:  chunk,
	}))
}
