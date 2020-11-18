package statesync

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	abci "github.com/tendermint/tendermint/abci/types"
	tmsync "github.com/tendermint/tendermint/libs/sync"
	"github.com/tendermint/tendermint/p2p"
	ssproto "github.com/tendermint/tendermint/proto/tendermint/statesync"
	"github.com/tendermint/tendermint/proxy"
	sm "github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/types"
)

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
	p2p.BaseReactor

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

	shim *reactorShim
}

// reactorShim is a temporary shim that resides in the Reactor that allows us
// to continue to fulfill the current p2p Reactor interface while using the new
// underlying p2p semantics.
type reactorShim struct {
	ctx    context.Context
	cancel context.CancelFunc

	snapshotInCh  chan p2p.Envelope
	snapshotOutCh chan p2p.Envelope
	chunkInCh     chan p2p.Envelope
	chunkOutCh    chan p2p.Envelope
	peerErrCh     chan p2p.PeerError
	peerUpdateCh  chan p2p.PeerUpdate
}

// NewReactor returns a reference to a new state-sync reactor. It accepts a Context
// which will be used for cancellations and deadlines when executing the reactor,
// a p2p Channel used to receive and send messages, and a router that will be
// used to get updates on peers.
func NewReactor(
	conn proxy.AppConnSnapshot,
	connQuery proxy.AppConnQuery,
	snapshotCh, chunkCh *p2p.Channel,
	peerUpdates p2p.PeerUpdates,
	tempDir string,
) *Reactor {
	return &Reactor{
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
			r.Logger.Debug("peer update", "peer", peerUpdate.PeerID.String())
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
					r.Logger.Error("failed to fetch snapshots", "err", err)
					continue
				}

				for _, snapshot := range snapshots {
					r.Logger.Debug("advertising snapshot", "height", snapshot.Height, "format", snapshot.Format, "peer", envelope.From.String())
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
					r.Logger.Debug("received unexpected snapshot; no state sync in progress")
					continue
				}

				r.mtx.RUnlock()

				r.Logger.Debug("received snapshot", "height", msg.Height, "format", msg.Format, "peer", envelope.From.String())
				_, err := r.syncer.AddSnapshot(envelope.From, &snapshot{
					Height:   msg.Height,
					Format:   msg.Format,
					Chunks:   msg.Chunks,
					Hash:     msg.Hash,
					Metadata: msg.Metadata,
				})
				if err != nil {
					r.Logger.Error("failed to add snapshot", "height", msg.Height, "format", msg.Format, "err", err, "channel", r.snapshotCh.ID)
					continue
				}

			default:
				r.Logger.Error("received unknown message: %T", msg)
				r.chunkCh.Error <- p2p.PeerError{
					PeerID:   envelope.From,
					Err:      fmt.Errorf("unexpected message: %T", msg),
					Severity: p2p.PeerErrorSeverityLow,
				}
			}

		case <-ctx.Done():
			r.Logger.Debug("stopped listening on snapshot channel")
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
				r.Logger.Debug("received chunk request", "height", msg.Height, "format", msg.Format, "chunk", msg.Index, "peer", envelope.From.String())
				resp, err := r.conn.LoadSnapshotChunkSync(abci.RequestLoadSnapshotChunk{
					Height: msg.Height,
					Format: msg.Format,
					Chunk:  msg.Index,
				})
				if err != nil {
					r.Logger.Error("failed to load chunk", "height", msg.Height, "format", msg.Format, "chunk", msg.Index, "err", err, "peer", envelope.From.String())
					continue
				}

				r.Logger.Debug("sending chunk", "height", msg.Height, "format", msg.Format, "chunk", msg.Index, "peer", envelope.From.String())
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
					r.Logger.Debug("received unexpected chunk; no state sync in progress", "peer", envelope.From.String())
					continue
				}

				r.mtx.RUnlock()

				r.Logger.Debug("received chunk; adding to sync", "height", msg.Height, "format", msg.Format, "chunk", msg.Index, "peer", envelope.From.String())
				_, err := r.syncer.AddChunk(&chunk{
					Height: msg.Height,
					Format: msg.Format,
					Index:  msg.Index,
					Chunk:  msg.Chunk,
					Sender: envelope.From,
				})
				if err != nil {
					r.Logger.Error("failed to add chunk", "height", msg.Height, "format", msg.Format, "chunk", msg.Index, "err", err, "peer", envelope.From.String())
					continue
				}

			default:
				r.Logger.Error("received unknown message: %T", msg)
				r.chunkCh.Error <- p2p.PeerError{
					PeerID:   envelope.From,
					Err:      fmt.Errorf("unexpected message: %T", msg),
					Severity: p2p.PeerErrorSeverityLow,
				}
			}

		case <-ctx.Done():
			r.Logger.Debug("stopped listening on chunk channel")
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

// ============================================================================
// Types and business logic below may be deprecated.
//
// TODO: Rename once legacy p2p types are removed.
// ref: https://github.com/tendermint/tendermint/issues/5670
// ============================================================================

func newReactorShim() *reactorShim {
	ctx, cancel := context.WithCancel(context.Background())
	return &reactorShim{
		ctx:           ctx,
		cancel:        cancel,
		snapshotInCh:  make(chan p2p.Envelope),
		snapshotOutCh: make(chan p2p.Envelope),
		chunkInCh:     make(chan p2p.Envelope),
		chunkOutCh:    make(chan p2p.Envelope),
		peerErrCh:     make(chan p2p.PeerError),
		peerUpdateCh:  make(chan p2p.PeerUpdate),
	}
}

// NewReactorDeprecated creates a new state sync reactor using the deprecated
// p2p stack.
func NewReactorDeprecated(conn proxy.AppConnSnapshot, connQuery proxy.AppConnQuery, tempDir string) *Reactor {
	shim := newReactorShim()

	snapshotCh := &p2p.Channel{
		ID:    p2p.ChannelID(SnapshotChannel),
		In:    shim.snapshotInCh,
		Out:   shim.snapshotOutCh,
		Error: shim.peerErrCh,
	}

	chunkCh := &p2p.Channel{
		ID:    p2p.ChannelID(ChunkChannel),
		In:    shim.chunkInCh,
		Out:   shim.chunkOutCh,
		Error: shim.peerErrCh,
	}

	r := NewReactor(conn, connQuery, snapshotCh, chunkCh, shim.peerUpdateCh, tempDir)
	r.BaseReactor = *p2p.NewBaseReactor("StateSync", r)
	r.shim = shim

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
	if r.IsRunning() {
		go r.Run(r.shim.ctx)
	}

	return nil
}

// AddPeer implements p2p.Reactor.
func (r *Reactor) AddPeer(peer p2p.Peer) {
	r.mtx.RLock()
	defer r.mtx.RUnlock()

	if r.syncer != nil {
		peerID, err := p2p.PeerIDFromString(string(peer.ID()))
		if err != nil {
			// It is OK to panic here as we'll be removing the Reactor interface and
			// Peer type in favor of using a PeerID directly.
			panic(err)
		}

		r.syncer.AddPeer(peerID)
	}
}

// RemovePeer implements p2p.Reactor.
func (r *Reactor) RemovePeer(peer p2p.Peer, reason interface{}) {
	r.mtx.RLock()
	defer r.mtx.RUnlock()

	if r.syncer != nil {
		peerID, err := p2p.PeerIDFromString(string(peer.ID()))
		if err != nil {
			// It is OK to panic here as we'll be removing the Reactor interface and
			// Peer type in favor of using a PeerID directly.
			panic(err)
		}

		r.syncer.RemovePeer(peerID)
	}
}

// Receive implements p2p.Reactor.
func (r *Reactor) Receive(chID byte, src p2p.Peer, msgBytes []byte) {
	if !r.IsRunning() {
		return
	}

	msg, err := decodeMsg(msgBytes)
	if err != nil {
		r.Logger.Error("error decoding message", "peer", src, "ch_id", chID, "msg", msg, "err", err, "bytes", msgBytes)
		r.Switch.StopPeerForError(src, err)
		return
	}

	if err = validateMsg(msg); err != nil {
		r.Logger.Error("invalid message", "peer", src, "ch_id", chID, "msg", msg, "err", err)
		r.Switch.StopPeerForError(src, err)
		return
	}

	peerID, err := p2p.PeerIDFromString(string(src.ID()))
	if err != nil {
		// It is OK to panic here as we'll be removing the Reactor interface and
		// Peer type in favor of using a PeerID directly.
		panic(err)
	}

	// Mimic the current p2p behavior where we proxy receiving a message by sending
	// a p2p envelope on the appropriate (new) p2p Channel.
	switch chID {
	case SnapshotChannel:
		go func() {
			r.shim.snapshotInCh <- p2p.Envelope{
				From:    peerID,
				Message: msg,
			}
		}()

	case ChunkChannel:
		go func() {
			r.shim.chunkInCh <- p2p.Envelope{
				From:    peerID,
				Message: msg,
			}
		}()

	default:
		r.Logger.Error("received message on an invalid channel", "ch_id", chID)
	}

	// Mimic the current p2p behavior where we send a response back to the source
	// peer on the appropriate channel.
	//
	// NOTE:
	// 1. We do not listen on peerErrCh because we're guaranteed the message is
	// valid by the above validateMsg call. Otherwise, we'd need to read off of it
	// since we do not want Receive to block.
	// 2. We only listen for envelopes on outbound channel(s) to send to the source
	// peer based on the specific message type. For messages that do not result
	// in a outbound envelopes (e.g. SnapshotsResponse), we do not listen otherwise
	// we'd need to either wait with a timeout or use complicated ACKs.
	_, chunkReq := msg.(*ssproto.ChunkRequest)
	_, snapReq := msg.(*ssproto.SnapshotsRequest)
	if chunkReq || snapReq {
		select {
		case e := <-r.shim.snapshotOutCh:
			src.Send(chID, mustEncodeMsg(e.Message.(*ssproto.SnapshotsResponse)))

		case e := <-r.shim.chunkOutCh:
			src.Send(chID, mustEncodeMsg(e.Message.(*ssproto.ChunkResponse)))
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

	r.syncer = newSyncer(r.Logger, r.conn, r.connQuery, stateProvider, r.snapshotCh.Out, r.chunkCh.Out, r.tempDir)
	r.mtx.Unlock()

	// request snapshots from all currently connected peers
	r.Logger.Debug("requesting snapshots from known peers")
	r.Switch.Broadcast(SnapshotChannel, mustEncodeMsg(&ssproto.SnapshotsRequest{}))

	state, commit, err := r.syncer.SyncAny(discoveryTime)

	r.mtx.Lock()
	r.syncer = nil
	r.mtx.Unlock()

	return state, commit, err
}
