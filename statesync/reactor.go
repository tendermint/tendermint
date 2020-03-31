package statesync

import (
	"sort"
	"sync"

	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/p2p"
	"github.com/tendermint/tendermint/proxy"
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

	// These will only be set when doing a state sync
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
	r.Logger.Debug("Requesting snapshots from peer", "peer", peer.ID())
	peer.Send(SnapshotChannel, cdc.MustMarshalBinaryBare(&snapshotsRequestMessage{}))
}

// RemovePeer implements p2p.Reactor.
func (r *Reactor) RemovePeer(peer p2p.Peer, reason interface{}) {
	r.Logger.Debug("Removing peer from pool", "peer", peer.ID())
	r.snapshots.RemovePeer(peer)
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
				src.Send(chID, cdc.MustMarshalBinaryBare(&snapshotsResponseMessage{
					Height:      snapshot.Height,
					Format:      snapshot.Format,
					ChunkHashes: snapshot.ChunkHashes,
					Metadata:    snapshot.Metadata,
				}))
			}

		case *snapshotsResponseMessage:
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
			resp, err := r.conn.LoadSnapshotChunkSync(abci.RequestLoadSnapshotChunk{
				Height: msg.Height,
				Format: msg.Format,
				Chunk:  msg.Chunk,
			})
			if err != nil {
				r.Logger.Error("Failed to load snapshot chunk", "height", msg.Height,
					"format", msg.Format, "chunk", msg.Chunk, "err", err)
				return
			}
			src.Send(ChunkChannel, cdc.MustMarshalBinaryBare(&chunkResponseMessage{
				Height:  msg.Height,
				Format:  msg.Format,
				Chunk:   msg.Chunk,
				Body:    resp.Chunk,
				Missing: resp.Chunk == nil,
			}))

		case *chunkResponseMessage:
			if r.chunks == nil {
				r.Logger.Error("received unexpected chunk", "height", msg.Height,
					"format", msg.Format, "chunk", msg.Chunk)
				return
			}
			err := r.chunks.Add(&chunk{
				Index: msg.Chunk,
				Body:  msg.Body,
			})
			if err != nil {
				r.Logger.Error("failed to add chunk", "height", msg.Height,
					"format", msg.Format, "chunk", msg.Chunk, "err", err)
				return
			}

		default:
			r.Logger.Error("Received unknown message %T", msg)
		}

	default:
		r.Logger.Error("Received message on invalid channel %v", chID)
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
