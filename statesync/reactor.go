package statesync

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/gogo/protobuf/proto"
	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/libs/log"
	tmsync "github.com/tendermint/tendermint/libs/sync"
	"github.com/tendermint/tendermint/p2p"
	ssproto "github.com/tendermint/tendermint/proto/tendermint/statesync"
	"github.com/tendermint/tendermint/proxy"
	sm "github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/types"
)

var _ p2p.MessageValidator = (*Reactor)(nil)

const (
	// SnapshotChannel exchanges snapshot metadata
	SnapshotChannel = byte(0x60)
	// ChunkChannel exchanges chunk contents
	ChunkChannel = byte(0x61)
	// recentSnapshots is the number of recent snapshots to send and receive per peer.
	recentSnapshots = 10
)

// Reactor handles state sync, both restoring snapshots for the local node and serving snapshots
// for other nodes.
//
// TODO: Remove any fields or embedded types that are no longer needed once the
// new p2p router is fully implemented.
// ref: https://github.com/tendermint/tendermint/issues/5670
type Reactor struct {
	// XXX: Keep a reference to the legacy p2p Switch in order to implement
	// MessageValidator and maintain current p2p Receive behavior.
	p2pSwitch *p2p.Switch

	logger      log.Logger
	conn        proxy.AppConnSnapshot
	connQuery   proxy.AppConnQuery
	tempDir     string
	snapshotCh  *p2p.Channel
	chunkCh     *p2p.Channel
	peerUpdates p2p.PeerUpdates

	// This will only be set when a state sync is in progress. It is used to feed
	// received snapshots and chunks into the sync.
	mtx    tmsync.RWMutex
	syncer *syncer
}

// NewReactor returns a reference to a new state-sync reactor. It accepts a Context
// which will be used for cancellations and deadlines when executing the reactor,
// a p2p Channel used to receive and send messages, and a router that will be
// used to get updates on peers.
func NewReactor(
	logger log.Logger,
	p2pSwitch *p2p.Switch,
	conn proxy.AppConnSnapshot,
	connQuery proxy.AppConnQuery,
	snapshotCh, chunkCh *p2p.Channel,
	peerUpdates p2p.PeerUpdates,
	tempDir string,
) *Reactor {
	return &Reactor{
		logger:      logger,
		p2pSwitch:   p2pSwitch,
		conn:        conn,
		connQuery:   connQuery,
		snapshotCh:  snapshotCh,
		chunkCh:     chunkCh,
		peerUpdates: peerUpdates,
		tempDir:     tempDir,
	}
}

// Run starts a blocking process for the state sync reactor. It listens for
// Envelope messages on a snapshot p2p Channel and chunk p2p Channel, in order
// of preference. In addition, the reactor will also listen for peer updates
// sent from the Router and respond to those updates accordingly. It returns when
// the reactor's context is cancelled.
func (r *Reactor) Run(ctx context.Context) {
	// Listen for envelopes on the snapshot p2p Channel in a separate go-routine
	// as to not block or cause IO contention with the chunk p2p Channel. Note,
	// we do not launch a go-routine to handle individual envelopes as to not
	// have to deal with bounding workers or pools.
	go func() {
		r.processSnapshotCh(ctx)
	}()

	// Listen for envelopes on the chunk p2p Channel in a separate go-routine
	// as to not block or cause IO contention with the snapshot p2p Channel. Note,
	// we do not launch a go-routine to handle individual envelopes as to not
	// have to deal with bounding workers or pools.
	go func() {
		r.processChunkCh(ctx)
	}()

	for {
		select {
		case peerUpdate := <-r.peerUpdates:
			r.logger.Debug("peer update", "peer", peerUpdate.PeerID.String())
			r.handlePeerUpdate(peerUpdate)

		case <-ctx.Done():
			return
		}
	}
}

func (r *Reactor) processSnapshotCh(ctx context.Context) {
	for {
		select {
		case envelope := <-r.snapshotCh.In:
			switch msg := envelope.Message.(type) {
			case *ssproto.SnapshotsRequest:
				snapshots, err := r.recentSnapshots(recentSnapshots)
				if err != nil {
					r.logger.Error("failed to fetch snapshots", "err", err)
					continue
				}

				for _, snapshot := range snapshots {
					r.logger.Debug("advertising snapshot", "height", snapshot.Height, "format", snapshot.Format, "peer", envelope.From.String())
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

				if r.syncer == nil {
					r.mtx.RUnlock()
					r.logger.Debug("received unexpected snapshot; no state sync in progress")
					continue
				}

				r.mtx.RUnlock()

				r.logger.Debug("received snapshot", "height", msg.Height, "format", msg.Format, "peer", envelope.From.String())
				_, err := r.syncer.AddSnapshot(envelope.From, &snapshot{
					Height:   msg.Height,
					Format:   msg.Format,
					Chunks:   msg.Chunks,
					Hash:     msg.Hash,
					Metadata: msg.Metadata,
				})
				if err != nil {
					r.logger.Error("failed to add snapshot", "height", msg.Height, "format", msg.Format, "err", err, "channel", r.snapshotCh.ID)
					continue
				}

			default:
				r.logger.Error("received unknown message: %T", msg)
				r.chunkCh.Error <- p2p.PeerError{
					PeerID:   envelope.From,
					Err:      fmt.Errorf("unexpected message: %T", msg),
					Severity: p2p.PeerErrorSeverityLow,
				}
			}

		case <-ctx.Done():
			r.logger.Debug("stopped listening on snapshot channel")
			return
		}
	}
}

func (r *Reactor) processChunkCh(ctx context.Context) {
	for {
		select {
		case envelope := <-r.chunkCh.In:
			switch msg := envelope.Message.(type) {
			case *ssproto.ChunkRequest:
				r.logger.Debug("received chunk request", "height", msg.Height, "format", msg.Format, "chunk", msg.Index, "peer", envelope.From.String())
				resp, err := r.conn.LoadSnapshotChunkSync(abci.RequestLoadSnapshotChunk{
					Height: msg.Height,
					Format: msg.Format,
					Chunk:  msg.Index,
				})
				if err != nil {
					r.logger.Error("failed to load chunk", "height", msg.Height, "format", msg.Format, "chunk", msg.Index, "err", err, "peer", envelope.From.String())
					continue
				}

				r.logger.Debug("sending chunk", "height", msg.Height, "format", msg.Format, "chunk", msg.Index, "peer", envelope.From.String())
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

				if r.syncer == nil {
					r.mtx.RUnlock()
					r.logger.Debug("received unexpected chunk; no state sync in progress", "peer", envelope.From.String())
					continue
				}

				r.mtx.RUnlock()

				r.logger.Debug("received chunk; adding to sync", "height", msg.Height, "format", msg.Format, "chunk", msg.Index, "peer", envelope.From.String())
				_, err := r.syncer.AddChunk(&chunk{
					Height: msg.Height,
					Format: msg.Format,
					Index:  msg.Index,
					Chunk:  msg.Chunk,
					Sender: envelope.From,
				})
				if err != nil {
					r.logger.Error("failed to add chunk", "height", msg.Height, "format", msg.Format, "chunk", msg.Index, "err", err, "peer", envelope.From.String())
					continue
				}

			default:
				r.logger.Error("received unknown message: %T", msg)
				r.chunkCh.Error <- p2p.PeerError{
					PeerID:   envelope.From,
					Err:      fmt.Errorf("unexpected message: %T", msg),
					Severity: p2p.PeerErrorSeverityLow,
				}
			}

		case <-ctx.Done():
			r.logger.Debug("stopped listening on chunk channel")
			return
		}
	}
}

func (r *Reactor) handlePeerUpdate(peerUpdate p2p.PeerUpdate) {
	r.mtx.RLock()
	defer r.mtx.RUnlock()

	if r.syncer != nil {
		switch peerUpdate.Status {
		case p2p.PeerStatusNew, p2p.PeerStatusUp:
			r.syncer.AddPeer(peerUpdate.PeerID)

		case p2p.PeerStatusDown, p2p.PeerStatusRemoved, p2p.PeerStatusBanned:
			r.syncer.RemovePeer(peerUpdate.PeerID)
		}
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

	r.syncer = newSyncer(r.logger, r.conn, r.connQuery, stateProvider, r.snapshotCh.Out, r.chunkCh.Out, r.tempDir)
	r.mtx.Unlock()

	// request snapshots from all currently connected peers
	r.logger.Debug("requesting snapshots from known peers")
	r.p2pSwitch.Broadcast(SnapshotChannel, mustEncodeMsg(&ssproto.SnapshotsRequest{}))

	state, commit, err := r.syncer.SyncAny(discoveryTime)

	r.mtx.Lock()
	r.syncer = nil
	r.mtx.Unlock()

	return state, commit, err
}

// ============================================================================
// TODO: Remove once legacy p2p stack is removed.
//
// ref: https://github.com/tendermint/tendermint/issues/5670
// ============================================================================

func (r *Reactor) OnUnmarshalFailure(_ byte, src p2p.Peer, _ []byte, err error) {
	r.p2pSwitch.StopPeerForError(src, err)
}

func (r *Reactor) Validate(_ byte, src p2p.Peer, _ []byte, msg proto.Message) error {
	if err := validateMsg(msg); err != nil {
		r.p2pSwitch.StopPeerForError(src, err)
		return err
	}

	return nil
}

// GetChannelShims returns a slice of ChannelDescriptorShim objects, where each
// object wraps a reference to a legacy p2p ChannelDescriptor and the corresponding
// p2p proto.Message the new p2p Channel is responsible for handling.
func GetChannelShims() []*p2p.ChannelDescriptorShim {
	return []*p2p.ChannelDescriptorShim{
		{
			MsgType: new(ssproto.Message),
			Descriptor: &p2p.ChannelDescriptor{
				ID:                  SnapshotChannel,
				Priority:            3,
				SendQueueCapacity:   10,
				RecvMessageCapacity: snapshotMsgSize,
			},
		},
		{
			MsgType: new(ssproto.Message),
			Descriptor: &p2p.ChannelDescriptor{
				ID:                  ChunkChannel,
				Priority:            1,
				SendQueueCapacity:   4,
				RecvMessageCapacity: chunkMsgSize,
			},
		},
	}
}
