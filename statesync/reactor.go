package statesync

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	abci "github.com/tendermint/tendermint/abci/types"
	lite "github.com/tendermint/tendermint/lite2"
	"github.com/tendermint/tendermint/p2p"
	"github.com/tendermint/tendermint/proxy"
	"github.com/tendermint/tendermint/types"
)

const (
	// SnapshotChannel exchanges snapshot metadata
	SnapshotChannel = byte(0x60)
	// ChunkChannel exchanges chunk contents
	ChunkChannel = byte(0x61)
)

// Reactor handles state sync, both restoring snapshots for the local node and serving snapshots
// for other nodes.
type Reactor struct {
	p2p.BaseReactor

	conn proxy.AppConnSnapshot

	// These will only be set when a state sync is in progress. They are used by the reactor
	// to record incoming snapshots and chunks from peers.
	mtx       sync.RWMutex
	snapshots *snapshotPool
	chunks    *chunkPool
}

// NewReactor creates a new state sync reactor.
func NewReactor(conn proxy.AppConnSnapshot) *Reactor {
	r := &Reactor{
		conn: conn,
	}
	r.BaseReactor = *p2p.NewBaseReactor("StateSync", r)
	return r
}

// GetChannels implements p2p.Reactor.
func (r *Reactor) GetChannels() []*p2p.ChannelDescriptor {
	return []*p2p.ChannelDescriptor{
		{
			ID:                  SnapshotChannel,
			Priority:            3,
			SendQueueCapacity:   10,
			RecvMessageCapacity: snapshotMsgSize,
		},
		{
			ID:                  ChunkChannel,
			Priority:            1,
			SendQueueCapacity:   4,
			RecvMessageCapacity: chunkMsgSize,
		},
	}
}

// OnStart implements p2p.Reactor.
func (r *Reactor) OnStart() error {
	return nil
}

// AddPeer implements p2p.Reactor.
func (r *Reactor) AddPeer(peer p2p.Peer) {
	r.mtx.RLock()
	defer r.mtx.RUnlock()

	if r.snapshots != nil {
		r.Logger.Debug("Requesting snapshots from peer", "peer", peer.ID())
		peer.Send(SnapshotChannel, cdc.MustMarshalBinaryBare(&snapshotsRequestMessage{}))
	}
}

// RemovePeer implements p2p.Reactor.
func (r *Reactor) RemovePeer(peer p2p.Peer, reason interface{}) {
	r.mtx.RLock()
	defer r.mtx.RUnlock()

	if r.snapshots != nil {
		r.Logger.Debug("Removing peer from pool", "peer", peer.ID())
		r.snapshots.RemovePeer(peer)
	}
}

// Receive implements p2p.Reactor.
func (r *Reactor) Receive(chID byte, src p2p.Peer, msgBytes []byte) {
	if !r.IsRunning() {
		return
	}

	msg, err := decodeMsg(msgBytes)
	if err != nil {
		r.Logger.Error("Error decoding message", "src", src, "chId", chID, "msg", msg, "err", err, "bytes", msgBytes)
		r.Switch.StopPeerForError(src, err)
		return
	}
	err = msg.ValidateBasic()
	if err != nil {
		r.Logger.Error("Invalid message", "peer", src, "msg", msg, "err", err)
		r.Switch.StopPeerForError(src, err)
		return
	}

	switch chID {
	case SnapshotChannel:
		switch msg := msg.(type) {
		case *snapshotsRequestMessage:
			snapshots, err := r.recentSnapshots(recentSnapshots)
			if err != nil {
				r.Logger.Error("Failed to fetch snapshots", "err", err)
				return
			}
			for _, snapshot := range snapshots {
				r.Logger.Debug("Advertising snapshot", "height", snapshot.Height, "format", snapshot.Format, "peer", src.ID())
				src.Send(chID, cdc.MustMarshalBinaryBare(&snapshotsResponseMessage{
					Height:      snapshot.Height,
					Format:      snapshot.Format,
					ChunkHashes: snapshot.ChunkHashes,
					Metadata:    snapshot.Metadata,
				}))
			}

		case *snapshotsResponseMessage:
			r.mtx.RLock()
			defer r.mtx.RUnlock()
			if r.snapshots == nil {
				r.Logger.Debug("Received unexpected snapshot, no state sync in progress")
				return
			}
			r.Logger.Debug("Received snapshot", "height", msg.Height, "format", msg.Format, "peer", src.ID())
			if r.snapshots.Add(src, &snapshot{
				Height:      msg.Height,
				Format:      msg.Format,
				ChunkHashes: msg.ChunkHashes,
				Metadata:    msg.Metadata,
			}) {
				r.Logger.Info("Discovered new snapshot", "height", msg.Height, "format", msg.Format)
			}

		default:
			r.Logger.Error("Received unknown message %T", msg)
		}

	case ChunkChannel:
		switch msg := msg.(type) {
		case *chunkRequestMessage:
			r.Logger.Debug("Received chunk request", "height", msg.Height, "format", msg.Format,
				"chunk", msg.Chunk, "peer", src.ID())
			resp, err := r.conn.LoadSnapshotChunkSync(abci.RequestLoadSnapshotChunk{
				Height: msg.Height,
				Format: msg.Format,
				Chunk:  msg.Chunk,
			})
			if err != nil {
				r.Logger.Error("Failed to load chunk", "height", msg.Height, "format", msg.Format, "chunk", msg.Chunk, "err", err)
				return
			}
			r.Logger.Debug("Sending chunk", "height", msg.Height, "format", msg.Format, "chunk", msg.Chunk, "peer", src.ID())
			src.Send(ChunkChannel, cdc.MustMarshalBinaryBare(&chunkResponseMessage{
				Height:  msg.Height,
				Format:  msg.Format,
				Chunk:   msg.Chunk,
				Body:    resp.Chunk,
				Missing: resp.Chunk == nil,
			}))

		case *chunkResponseMessage:
			r.mtx.RLock()
			defer r.mtx.RUnlock()
			if r.chunks == nil {
				r.Logger.Debug("Received unexpected chunk, no state sync in progress", "peer", src.ID())
				return
			}
			added, err := r.chunks.Add(&chunk{
				Index: msg.Chunk,
				Body:  msg.Body,
			})
			if err != nil {
				r.Logger.Error("Failed to add chunk", "height", msg.Height, "format", msg.Format,
					"chunk", msg.Chunk, "err", err)
				return
			}
			if added {
				r.Logger.Info("Received chunk", "height", msg.Height, "format", msg.Format,
					"chunk", msg.Chunk, "peer", src.ID())
			} else {
				r.Logger.Debug("Ignoring duplicate chunk", "height", msg.Height, "format", msg.Format,
					"chunk", msg.Chunk, "peer", src.ID())
			}

		default:
			r.Logger.Error("Received unknown message %T", msg)
		}

	default:
		r.Logger.Error("Received message on invalid channel %x", chID)
	}
}

// recentSnapshots fetches the n most recent snapshots from the app
func (r *Reactor) recentSnapshots(n uint32) ([]*snapshot, error) {
	resp, err := r.conn.ListSnapshotsSync(abci.RequestListSnapshots{})
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
			Height:      s.Height,
			Format:      s.Format,
			ChunkHashes: s.ChunkHashes,
			Metadata:    s.Metadata,
		})
	}
	return snapshots, nil
}

// sync runs a state sync
func (r *Reactor) Sync(lc *lite.Client, query proxy.AppConnQuery) error {
	r.Logger.Info("Starting state sync")

	snapshots := newSnapshotPool()
	r.mtx.Lock()
	if r.snapshots != nil {
		r.mtx.Unlock()
		return errors.New("a state sync is already in progress")
	}
	r.snapshots = snapshots
	r.mtx.Unlock()

	// discover snapshots
	r.Logger.Info("Discovering snapshots")
	time.Sleep(10 * time.Second)

	var snapshot *snapshot
	var header *types.SignedHeader
	var err error
	for {
		snapshot = snapshots.Best()
		if snapshot == nil {
			r.Logger.Info("No suitable snapshots found, still looking")
			time.Sleep(10 * time.Second)
			continue
		}
		r.Logger.Info("Fetching snapshot app hash using light client", "height", snapshot.Height, "format", snapshot.Format)
		header, err = lc.VerifyHeaderAtHeight(int64(snapshot.Height+1), time.Now())
		if err != nil {
			r.Logger.Error("Failed to verify header, ignoring height", "height", snapshot.Height, "err", err)
			snapshots.RejectHeight(snapshot.Height)
			continue
		}
		r.Logger.Info("Offering snapshot to ABCI app", "height", snapshot.Height, "format", snapshot.Format)
		resp, err := r.conn.OfferSnapshotSync(abci.RequestOfferSnapshot{
			Snapshot: &abci.Snapshot{
				Height:      snapshot.Height,
				Format:      snapshot.Format,
				ChunkHashes: snapshot.ChunkHashes,
				Metadata:    snapshot.Metadata,
			},
			AppHash: header.AppHash.Bytes(), // FIXME Light client verification
		})
		if err != nil {
			return fmt.Errorf("failed to offer snapshot at height %v format %v: %w",
				snapshot.Height, snapshot.Format, err)
		}
		if resp.Accepted {
			r.Logger.Info("Snapshot accepted, restoring", "height", snapshot.Height,
				"format", snapshot.Format, "chunks", len(snapshot.ChunkHashes))
			break
		}
		r.Logger.Info("Snapshot rejected", "height", snapshot.Height, "format", snapshot.Format)
		switch resp.Reason {
		case abci.ResponseOfferSnapshot_invalid_height:
			snapshots.RejectHeight(snapshot.Height)
		case abci.ResponseOfferSnapshot_invalid_format:
			snapshots.RejectFormat(snapshot.Format)
		default:
			snapshots.Reject(snapshot)
		}
	}

	// fetch chunks
	chunks, err := newChunkPool(snapshot)
	if err != nil {
		return fmt.Errorf("failed to create chunk pool: %w", err)
	}
	defer chunks.Close()
	r.mtx.Lock()
	r.chunks = chunks
	r.mtx.Unlock()

	pending := make(chan uint32, len(snapshot.ChunkHashes))
	for i := uint32(0); i < uint32(len(snapshot.ChunkHashes)); i++ {
		pending <- i
	}
	for i := 0; i < 4; i++ {
		go func() {
			for index := range pending {
				for !chunks.Has(index) {
					peer := snapshots.GetPeer(snapshot)
					if peer == nil {
						r.Logger.Info("No peers found for snapshot", "height", snapshot.Height,
							"format", snapshot.Format)
					} else {
						r.Logger.Debug("Requesting chunk", "height", snapshot.Height,
							"format", snapshot.Format, "chunk", index, "peer", peer.ID())
						peer.Send(ChunkChannel, cdc.MustMarshalBinaryBare(&chunkRequestMessage{
							Height: snapshot.Height,
							Format: snapshot.Format,
							Chunk:  index,
						}))
					}
					// wait for chunk to be returned
					time.Sleep(10 * time.Second)
				}
			}
		}()
	}

	// Feed chunks to app
	for {
		chunk, err := chunks.Next()
		if err == Done {
			break
		} else if err != nil {
			return fmt.Errorf("failed to fetch chunks: %w", err)
		}
		r.Logger.Info("Applying chunk to ABCI app", "height", snapshot.Height,
			"format", snapshot.Format, "chunk", chunk.Index, "total", len(snapshot.ChunkHashes))
		resp, err := r.conn.ApplySnapshotChunkSync(abci.RequestApplySnapshotChunk{
			Chunk: chunk.Body,
		})
		if err != nil {
			return fmt.Errorf("failed to apply chunk %v: %w", chunk.Index, err)
		}
		if !resp.Applied {
			return fmt.Errorf("failed to apply chunk %v", chunk.Index)
		}
	}

	// Verify app hash.
	resp, err := query.InfoSync(proxy.RequestInfo)
	if err != nil {
		return fmt.Errorf("failed to query ABCI app for app_hash: %w", err)
	}
	if !bytes.Equal(header.AppHash.Bytes(), resp.LastBlockAppHash) {
		return fmt.Errorf("app_hash verification failed, expected %x got %x",
			header.AppHash.Bytes(), resp.LastBlockAppHash)
	}
	r.Logger.Debug("Verified state snapshot app hash", "height", snapshot.Height, "format", snapshot.Format,
		"hash", hex.EncodeToString(resp.LastBlockAppHash))

	// Done!
	r.Logger.Info("Snapshot restored", "height", snapshot.Height, "format", snapshot.Format)

	return nil
}
