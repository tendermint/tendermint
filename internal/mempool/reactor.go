package mempool

import (
	"context"
	"errors"
	"fmt"
	"runtime/debug"
	"sync"
	"time"

	"github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/internal/libs/clist"
	"github.com/tendermint/tendermint/internal/p2p"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/libs/service"
	protomem "github.com/tendermint/tendermint/proto/tendermint/mempool"
	"github.com/tendermint/tendermint/types"
)

var (
	_ service.Service = (*Reactor)(nil)
	_ p2p.Wrapper     = (*protomem.Message)(nil)
)

// PeerManager defines the interface contract required for getting necessary
// peer information. This should eventually be replaced with a message-oriented
// approach utilizing the p2p stack.
type PeerManager interface {
	GetHeight(types.NodeID) int64
}

// Reactor implements a service that contains mempool of txs that are broadcasted
// amongst peers. It maintains a map from peer ID to counter, to prevent gossiping
// txs to the peers you received it from.
type Reactor struct {
	service.BaseService
	logger log.Logger

	cfg     *config.MempoolConfig
	mempool *TxMempool
	ids     *IDs

	// XXX: Currently, this is the only way to get information about a peer. Ideally,
	// we rely on message-oriented communication to get necessary peer data.
	// ref: https://github.com/tendermint/tendermint/issues/5670
	peerMgr PeerManager

	mempoolCh   *p2p.Channel
	peerUpdates *p2p.PeerUpdates

	// observePanic is a function for observing panics that were recovered in methods on
	// Reactor. observePanic is called with the recovered value.
	observePanic func(interface{})

	mtx          sync.Mutex
	peerRoutines map[types.NodeID]context.CancelFunc
}

// NewReactor returns a reference to a new reactor.
func NewReactor(
	ctx context.Context,
	logger log.Logger,
	cfg *config.MempoolConfig,
	peerMgr PeerManager,
	txmp *TxMempool,
	chCreator p2p.ChannelCreator,
	peerUpdates *p2p.PeerUpdates,
) (*Reactor, error) {

	ch, err := chCreator(ctx, getChannelDescriptor(cfg))
	if err != nil {
		return nil, err
	}

	r := &Reactor{
		logger:       logger,
		cfg:          cfg,
		peerMgr:      peerMgr,
		mempool:      txmp,
		ids:          NewMempoolIDs(),
		mempoolCh:    ch,
		peerUpdates:  peerUpdates,
		peerRoutines: make(map[types.NodeID]context.CancelFunc),
		observePanic: defaultObservePanic,
	}

	r.BaseService = *service.NewBaseService(logger, "Mempool", r)
	return r, nil
}

func defaultObservePanic(r interface{}) {}

// getChannelDescriptor produces an instance of a descriptor for this
// package's required channels.
func getChannelDescriptor(cfg *config.MempoolConfig) *p2p.ChannelDescriptor {
	largestTx := make([]byte, cfg.MaxTxBytes)
	batchMsg := protomem.Message{
		Sum: &protomem.Message_Txs{
			Txs: &protomem.Txs{Txs: [][]byte{largestTx}},
		},
	}

	return &p2p.ChannelDescriptor{
		ID:                  MempoolChannel,
		MessageType:         new(protomem.Message),
		Priority:            5,
		RecvMessageCapacity: batchMsg.Size(),
		RecvBufferCapacity:  128,
	}
}

// OnStart starts separate go routines for each p2p Channel and listens for
// envelopes on each. In addition, it also listens for peer updates and handles
// messages on that p2p channel accordingly. The caller must be sure to execute
// OnStop to ensure the outbound p2p Channels are closed.
func (r *Reactor) OnStart(ctx context.Context) error {
	if !r.cfg.Broadcast {
		r.logger.Info("tx broadcasting is disabled")
	}

	go r.processMempoolCh(ctx)
	go r.processPeerUpdates(ctx)

	return nil
}

// OnStop stops the reactor by signaling to all spawned goroutines to exit and
// blocking until they all exit.
func (r *Reactor) OnStop() {}

// handleMempoolMessage handles envelopes sent from peers on the MempoolChannel.
// For every tx in the message, we execute CheckTx. It returns an error if an
// empty set of txs are sent in an envelope or if we receive an unexpected
// message type.
func (r *Reactor) handleMempoolMessage(ctx context.Context, envelope *p2p.Envelope) error {
	logger := r.logger.With("peer", envelope.From)

	switch msg := envelope.Message.(type) {
	case *protomem.Txs:
		protoTxs := msg.GetTxs()
		if len(protoTxs) == 0 {
			return errors.New("empty txs received from peer")
		}

		txInfo := TxInfo{SenderID: r.ids.GetForPeer(envelope.From)}
		if len(envelope.From) != 0 {
			txInfo.SenderNodeID = envelope.From
		}

		for _, tx := range protoTxs {
			if err := r.mempool.CheckTx(ctx, types.Tx(tx), nil, txInfo); err != nil {
				if errors.Is(err, types.ErrTxInCache) {
					// if the tx is in the cache,
					// then we've been gossiped a
					// Tx that we've already
					// got. Gossip should be
					// smarter, but it's not a
					// problem.
					continue
				}
				logger.Error("checktx failed for tx",
					"tx", fmt.Sprintf("%X", types.Tx(tx).Hash()),
					"err", err)
			}
		}

	default:
		return fmt.Errorf("received unknown message: %T", msg)
	}

	return nil
}

// handleMessage handles an Envelope sent from a peer on a specific p2p Channel.
// It will handle errors and any possible panics gracefully. A caller can handle
// any error returned by sending a PeerError on the respective channel.
func (r *Reactor) handleMessage(ctx context.Context, chID p2p.ChannelID, envelope *p2p.Envelope) (err error) {
	defer func() {
		if e := recover(); e != nil {
			r.observePanic(e)
			err = fmt.Errorf("panic in processing message: %v", e)
			r.logger.Error(
				"recovering from processing message panic",
				"err", err,
				"stack", string(debug.Stack()),
			)
		}
	}()

	r.logger.Debug("received message", "peer", envelope.From)

	switch chID {
	case MempoolChannel:
		err = r.handleMempoolMessage(ctx, envelope)

	default:
		err = fmt.Errorf("unknown channel ID (%d) for envelope (%T)", chID, envelope.Message)
	}

	return err
}

// processMempoolCh implements a blocking event loop where we listen for p2p
// Envelope messages from the mempoolCh.
func (r *Reactor) processMempoolCh(ctx context.Context) {
	iter := r.mempoolCh.Receive(ctx)
	for iter.Next(ctx) {
		envelope := iter.Envelope()
		if err := r.handleMessage(ctx, r.mempoolCh.ID, envelope); err != nil {
			r.logger.Error("failed to process message", "ch_id", r.mempoolCh.ID, "envelope", envelope, "err", err)
			if serr := r.mempoolCh.SendError(ctx, p2p.PeerError{
				NodeID: envelope.From,
				Err:    err,
			}); serr != nil {
				return
			}
		}
	}
}

// processPeerUpdate processes a PeerUpdate. For added peers, PeerStatusUp, we
// check if the reactor is running and if we've already started a tx broadcasting
// goroutine or not. If not, we start one for the newly added peer. For down or
// removed peers, we remove the peer from the mempool peer ID set and signal to
// stop the tx broadcasting goroutine.
func (r *Reactor) processPeerUpdate(ctx context.Context, peerUpdate p2p.PeerUpdate) {
	r.logger.Debug("received peer update", "peer", peerUpdate.NodeID, "status", peerUpdate.Status)

	r.mtx.Lock()
	defer r.mtx.Unlock()

	switch peerUpdate.Status {
	case p2p.PeerStatusUp:
		// Do not allow starting new tx broadcast loops after reactor shutdown
		// has been initiated. This can happen after we've manually closed all
		// peer broadcast, but the router still sends in-flight peer updates.
		if !r.IsRunning() {
			return
		}

		if r.cfg.Broadcast {
			// Check if we've already started a goroutine for this peer, if not we create
			// a new done channel so we can explicitly close the goroutine if the peer
			// is later removed, we increment the waitgroup so the reactor can stop
			// safely, and finally start the goroutine to broadcast txs to that peer.
			_, ok := r.peerRoutines[peerUpdate.NodeID]
			if !ok {
				pctx, pcancel := context.WithCancel(ctx)
				r.peerRoutines[peerUpdate.NodeID] = pcancel

				r.ids.ReserveForPeer(peerUpdate.NodeID)

				// start a broadcast routine ensuring all txs are forwarded to the peer
				go r.broadcastTxRoutine(pctx, peerUpdate.NodeID)
			}
		}

	case p2p.PeerStatusDown:
		r.ids.Reclaim(peerUpdate.NodeID)

		// Check if we've started a tx broadcasting goroutine for this peer.
		// If we have, we signal to terminate the goroutine via the channel's closure.
		// This will internally decrement the peer waitgroup and remove the peer
		// from the map of peer tx broadcasting goroutines.
		closer, ok := r.peerRoutines[peerUpdate.NodeID]
		if ok {
			closer()
		}
	}
}

// processPeerUpdates initiates a blocking process where we listen for and handle
// PeerUpdate messages. When the reactor is stopped, we will catch the signal and
// close the p2p PeerUpdatesCh gracefully.
func (r *Reactor) processPeerUpdates(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case peerUpdate := <-r.peerUpdates.Updates():
			r.processPeerUpdate(ctx, peerUpdate)
		}
	}
}

func (r *Reactor) broadcastTxRoutine(ctx context.Context, peerID types.NodeID) {
	peerMempoolID := r.ids.GetForPeer(peerID)
	var nextGossipTx *clist.CElement

	// remove the peer ID from the map of routines and mark the waitgroup as done
	defer func() {
		r.mtx.Lock()
		delete(r.peerRoutines, peerID)
		r.mtx.Unlock()

		if e := recover(); e != nil {
			r.observePanic(e)
			r.logger.Error(
				"recovering from broadcasting mempool loop",
				"err", e,
				"stack", string(debug.Stack()),
			)
		}
	}()

	for {
		if !r.IsRunning() || ctx.Err() != nil {
			return
		}

		// This happens because the CElement we were looking at got garbage
		// collected (removed). That is, .NextWait() returned nil. Go ahead and
		// start from the beginning.
		if nextGossipTx == nil {
			select {
			case <-ctx.Done():
				return
			case <-r.mempool.WaitForNextTx(): // wait until a tx is available
				if nextGossipTx = r.mempool.NextGossipTx(); nextGossipTx == nil {
					continue
				}
			}
		}

		memTx := nextGossipTx.Value.(*WrappedTx)

		if r.peerMgr != nil {
			height := r.peerMgr.GetHeight(peerID)
			if height > 0 && height < memTx.height-1 {
				// allow for a lag of one block
				time.Sleep(PeerCatchupSleepIntervalMS * time.Millisecond)
				continue
			}
		}

		// NOTE: Transaction batching was disabled due to:
		// https://github.com/tendermint/tendermint/issues/5796
		if ok := r.mempool.txStore.TxHasPeer(memTx.hash, peerMempoolID); !ok {
			// Send the mempool tx to the corresponding peer. Note, the peer may be
			// behind and thus would not be able to process the mempool tx correctly.
			if err := r.mempoolCh.Send(ctx, p2p.Envelope{
				To: peerID,
				Message: &protomem.Txs{
					Txs: [][]byte{memTx.tx},
				},
			}); err != nil {
				return
			}

			r.logger.Debug(
				"gossiped tx to peer",
				"tx", fmt.Sprintf("%X", memTx.tx.Hash()),
				"peer", peerID,
			)
		}

		select {
		case <-nextGossipTx.NextWaitChan():
			nextGossipTx = nextGossipTx.Next()
		case <-ctx.Done():
			return
		}
	}
}
