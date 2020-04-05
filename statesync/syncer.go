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
	lite "github.com/tendermint/tendermint/lite2"
	"github.com/tendermint/tendermint/p2p"
	"github.com/tendermint/tendermint/proxy"
	sm "github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/types"
)

const (
	// chunkFetchers is the number of concurrent chunk fetchers to run.
	chunkFetchers = 4
	// chunkTimeout is the timeout before refetching a chunk, possibly with a different peer.
	chunkTimeout = 10 * time.Second
)

var (
	// errNoSnapshots is returned by Sync() when no viable snapshots are found in the pool.
	errNoSnapshots = errors.New("no viable snapshots found")
)

// syncer runs a state sync against an ABCI app. It relies on the reactor for peer communication.
// Typical usage goes something like this:
//
// 1) Create the syncer: newSyncer(...)
// 2) Discover snapshots, and feed them via syncer.AddSnapshot()
// 3) Start a sync via Sync()
// 4) Concurrently feed chunks via syncer.AddChunk()
// 5) When done, bootstrap node with new state and commit
type syncer struct {
	logger    log.Logger
	lc        *lite.Client
	conn      proxy.AppConnSnapshot
	snapshots *snapshotPool

	mtx    sync.RWMutex
	chunks *chunkQueue // once a sync is started, this will be non-nil and used to feed chunks
}

// newSyncer creates a new syncer.
func newSyncer(logger log.Logger, conn proxy.AppConnSnapshot, lc *lite.Client) *syncer {
	return &syncer{
		logger:    logger,
		lc:        lc,
		conn:      conn,
		snapshots: newSnapshotPool(lc),
		chunks:    nil,
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
	return s.chunks.Add(chunk)
}

// AddSnapshot adds a snapshot to the snapshot pool. It returns true if a new, previously unseen
// snapshot was accepted and added.
func (s *syncer) AddSnapshot(peer p2p.Peer, snapshot *snapshot) (bool, error) {
	return s.snapshots.Add(peer, snapshot)
}

// Sync executes a sync, returning the latest state and block header. The caller must bootstrap the
// node with the new state and latest commit.
//
// The caller must give the syncer time to discover new snapshots via AddSnapshot() before starting
// the sync, and concurrently feed chunks to the syncer via AddChunk. It returns errNoSnapshots if
// no viable snapshots were found, in which case the caller may want to continue discovery.
func (s *syncer) Sync(state sm.State, query proxy.AppConnQuery) (sm.State, *types.Commit, error) {
	// Start state sync.
	snapshot, chunks, err := s.startSync()
	if err != nil {
		return state, nil, err
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

	// Apply chunks.
	for {
		chunk, err := chunks.Next()
		if err == Done {
			break
		} else if err != nil {
			return state, nil, fmt.Errorf("failed to fetch chunk: %w", err)
		}
		s.logger.Info("Applying chunk to ABCI app", "chunk", chunk.Index, "total", chunks.Total())
		resp, err := s.conn.ApplySnapshotChunkSync(abci.RequestApplySnapshotChunk{Chunk: chunk.Body})
		if err != nil {
			return state, nil, fmt.Errorf("failed to apply chunk %v: %w", chunk.Index, err)
		}
		if !resp.Applied {
			return state, nil, fmt.Errorf("failed to apply chunk %v", chunk.Index)
		}
	}

	// Snapshot should now be restored, verify it.
	resp, err := query.InfoSync(proxy.RequestInfo)
	if err != nil {
		return state, nil, fmt.Errorf("failed to query ABCI app for app_hash: %w", err)
	}
	if !bytes.Equal(snapshot.trustedAppHash, resp.LastBlockAppHash) {
		return state, nil, fmt.Errorf("app_hash verification failed, expected %x got %x",
			snapshot.trustedAppHash, resp.LastBlockAppHash)
	}
	s.logger.Debug("Verified state snapshot app hash", "height", snapshot.Height,
		"format", snapshot.Format, "hash", hex.EncodeToString(resp.LastBlockAppHash))

	// Build new state.
	state, commit, err := s.buildState(state, int64(snapshot.Height))
	if err != nil {
		return state, nil, fmt.Errorf("failed to build new state: %w", err)
	}

	// Done! 🎉
	s.logger.Info("Snapshot restored", "height", snapshot.Height, "format", snapshot.Format)

	return state, commit, nil
}

// startSync attempts to start a sync, if able (i.e. if any valid snapshots are available).
// It sets up a chunk queue in s.chunks, or returns errNoSnapshots if no viable snapshots were found.
func (s *syncer) startSync() (*snapshot, *chunkQueue, error) {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	if s.chunks != nil {
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
			s.logger.Info("Snapshot accepted", "height", snapshot.Height,
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
		return nil, nil, fmt.Errorf("failed to create snapshot chunk queue: %w", err)
	}
	s.chunks = chunks
	return snapshot, chunks, nil
}

// endSync ends the sync.
func (s *syncer) endSync() {
	s.mtx.Lock()
	defer s.mtx.Unlock()
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
		Chunk:  chunk,
	}))
}

// buildState builds a new state using the light client. It does not modify the given state.
func (s *syncer) buildState(state sm.State, height int64) (sm.State, *types.Commit, error) {
	header, err := s.lc.VerifyHeaderAtHeight(height, time.Now())
	if err != nil {
		return state, nil, err
	}
	nextHeader, err := s.lc.VerifyHeaderAtHeight(height+1, time.Now())
	if err != nil {
		return state, nil, err
	}

	state = state.Copy()
	state.LastBlockHeight = header.Height
	state.LastBlockTime = header.Time
	state.LastBlockID = header.Commit.BlockID

	// FIXME Check that these heights are correct.
	state.LastValidators, _, err = s.lc.TrustedValidatorSet(height)
	if err != nil {
		return state, nil, err
	}
	state.Validators, _, err = s.lc.TrustedValidatorSet(height + 1)
	if err != nil {
		return state, nil, err
	}
	state.NextValidators = state.Validators
	state.LastHeightValidatorsChanged = height

	state.AppHash = nextHeader.AppHash
	state.LastResultsHash = nextHeader.LastResultsHash

	return state, header.Commit, nil
}
