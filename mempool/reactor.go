package mempool

import (
	"errors"
	"fmt"
	"math"
	"sync"
	"time"

	cfg "github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/libs/clist"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/libs/service"
	tmsync "github.com/tendermint/tendermint/libs/sync"
	"github.com/tendermint/tendermint/p2p"
	protomem "github.com/tendermint/tendermint/proto/tendermint/mempool"
	"github.com/tendermint/tendermint/types"
)

var (
	_ service.Service = (*Reactor)(nil)
	_ p2p.Wrapper     = (*protomem.Message)(nil)
)

const (
	MempoolChannel = p2p.ChannelID(0x30)

	// peerCatchupSleepIntervalMS defines how much time to sleep if a peer is behind
	peerCatchupSleepIntervalMS = 100

	// UnknownPeerID is the peer ID to use when running CheckTx when there is
	// no peer (e.g. RPC)
	UnknownPeerID uint16 = 0

	maxActiveIDs = math.MaxUint16
)

// PeerManager defines the interface contract required for getting necessary
// peer information. This should eventually be replaced with a message-oriented
// approach utilizing the p2p stack.
type PeerManager interface {
	GetHeight(p2p.NodeID) int64
}

// Reactor implements a service that contains mempool of txs that are broadcasted
// amongst peers. It maintains a map from peer ID to counter, to prevent gossiping
// txs to the peers you received it from.
type Reactor struct {
	service.BaseService

	config  *cfg.MempoolConfig
	mempool *CListMempool
	ids     *mempoolIDs

	// XXX: Currently, this is the only way to get information about a peer. Ideally,
	// we rely on message-oriented communication to get necessary peer data.
	// ref: https://github.com/tendermint/tendermint/issues/5670
	peerMgr PeerManager

	mempoolCh   *p2p.Channel
	peerUpdates *p2p.PeerUpdatesCh
	closeCh     chan struct{}

	// peerWG is used to coordinate graceful termination of all peer broadcasting
	// goroutines.
	peerWG sync.WaitGroup

	mtx          tmsync.Mutex
	peerRoutines map[p2p.NodeID]*tmsync.Closer
}

// NewReactor returns a reference to a new reactor.
func NewReactor(
	logger log.Logger,
	config *cfg.MempoolConfig,
	peerMgr PeerManager,
	mempool *CListMempool,
	mempoolCh *p2p.Channel,
	peerUpdates *p2p.PeerUpdatesCh,
) *Reactor {

	r := &Reactor{
		config:       config,
		peerMgr:      peerMgr,
		mempool:      mempool,
		ids:          newMempoolIDs(),
		mempoolCh:    mempoolCh,
		peerUpdates:  peerUpdates,
		closeCh:      make(chan struct{}),
		peerRoutines: make(map[p2p.NodeID]*tmsync.Closer),
	}

	r.BaseService = *service.NewBaseService(logger, "Mempool", r)
	return r
}

// GetChannelShims returns a map of ChannelDescriptorShim objects, where each
// object wraps a reference to a legacy p2p ChannelDescriptor and the corresponding
// p2p proto.Message the new p2p Channel is responsible for handling.
//
//
// TODO: Remove once p2p refactor is complete.
// ref: https://github.com/tendermint/tendermint/issues/5670
func GetChannelShims(config *cfg.MempoolConfig) map[p2p.ChannelID]*p2p.ChannelDescriptorShim {
	largestTx := make([]byte, config.MaxTxBytes)
	batchMsg := protomem.Message{
		Sum: &protomem.Message_Txs{
			Txs: &protomem.Txs{Txs: [][]byte{largestTx}},
		},
	}

	return map[p2p.ChannelID]*p2p.ChannelDescriptorShim{
		MempoolChannel: {
			MsgType: new(protomem.Message),
			Descriptor: &p2p.ChannelDescriptor{
				ID:                  byte(MempoolChannel),
				Priority:            5,
				RecvMessageCapacity: batchMsg.Size(),
			},
		},
	}
}

// OnStart starts separate go routines for each p2p Channel and listens for
// envelopes on each. In addition, it also listens for peer updates and handles
// messages on that p2p channel accordingly. The caller must be sure to execute
// OnStop to ensure the outbound p2p Channels are closed.
func (r *Reactor) OnStart() error {
	if !r.config.Broadcast {
		r.Logger.Info("tx broadcasting is disabled")
	}

	go r.processMempoolCh()
	go r.processPeerUpdates()

	return nil
}

// OnStop stops the reactor by signaling to all spawned goroutines to exit and
// blocking until they all exit.
func (r *Reactor) OnStop() {
	r.mtx.Lock()
	for _, c := range r.peerRoutines {
		c.Close()
	}
	r.mtx.Unlock()

	// wait for all spawned peer tx broadcasting goroutines to gracefully exit
	r.peerWG.Wait()

	// Close closeCh to signal to all spawned goroutines to gracefully exit. All
	// p2p Channels should execute Close().
	close(r.closeCh)

	// Wait for all p2p Channels to be closed before returning. This ensures we
	// can easily reason about synchronization of all p2p Channels and ensure no
	// panics will occur.
	<-r.mempoolCh.Done()
	<-r.peerUpdates.Done()
}

// handleMempoolMessage handles envelopes sent from peers on the MempoolChannel.
// For every tx in the message, we execute CheckTx. It returns an error if an
// empty set of txs are sent in an envelope or if we receive an unexpected
// message type.
func (r *Reactor) handleMempoolMessage(envelope p2p.Envelope) error {
	logger := r.Logger.With("peer", envelope.From)

	switch msg := envelope.Message.(type) {
	case *protomem.Txs:
		protoTxs := msg.GetTxs()
		if len(protoTxs) == 0 {
			return errors.New("empty txs received from peer")
		}

		txInfo := TxInfo{SenderID: r.ids.GetForPeer(envelope.From)}
		if len(envelope.From) != 0 {
			txInfo.SenderP2PID = envelope.From
		}

		for _, tx := range protoTxs {
			if err := r.mempool.CheckTx(types.Tx(tx), nil, txInfo); err != nil {
				logger.Error("checktx failed for tx", "tx", txID(tx), "err", err)
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
func (r *Reactor) handleMessage(chID p2p.ChannelID, envelope p2p.Envelope) (err error) {
	defer func() {
		if e := recover(); e != nil {
			err = fmt.Errorf("panic in processing message: %v", e)
		}
	}()

	r.Logger.Debug("received message", "peer", envelope.From)

	switch chID {
	case MempoolChannel:
		err = r.handleMempoolMessage(envelope)

	default:
		err = fmt.Errorf("unknown channel ID (%d) for envelope (%v)", chID, envelope)
	}

	return err
}

// processMempoolCh implements a blocking event loop where we listen for p2p
// Envelope messages from the mempoolCh.
func (r *Reactor) processMempoolCh() {
	defer r.mempoolCh.Close()

	for {
		select {
		case envelope := <-r.mempoolCh.In():
			if err := r.handleMessage(r.mempoolCh.ID(), envelope); err != nil {
				r.Logger.Error("failed to process message", "ch_id", r.mempoolCh.ID(), "envelope", envelope, "err", err)
				r.mempoolCh.Error() <- p2p.PeerError{
					PeerID:   envelope.From,
					Err:      err,
					Severity: p2p.PeerErrorSeverityLow,
				}
			}

		case <-r.closeCh:
			r.Logger.Debug("stopped listening on mempool channel; closing...")
			return
		}
	}
}

// processPeerUpdate processes a PeerUpdate. For added peers, PeerStatusUp, we
// check if the reactor is running and if we've already started a tx broadcasting
// goroutine or not. If not, we start one for the newly added peer. For down or
// removed peers, we remove the peer from the mempool peer ID set and signal to
// stop the tx broadcasting goroutine.
func (r *Reactor) processPeerUpdate(peerUpdate p2p.PeerUpdate) {
	r.Logger.Debug("received peer update", "peer", peerUpdate.PeerID, "status", peerUpdate.Status)

	r.mtx.Lock()
	defer r.mtx.Unlock()

	switch peerUpdate.Status {
	case p2p.PeerStatusUp:
		// Do not allow starting new tx broadcast loops after reactor shutdown
		// has been initiated. This can happen after we've manually closed all
		// peer broadcast loops and closed r.closeCh, but the router still sends
		// in-flight peer updates.
		if !r.IsRunning() {
			return
		}

		if r.config.Broadcast {
			// Check if we've already started a goroutine for this peer, if not we create
			// a new done channel so we can explicitly close the goroutine if the peer
			// is later removed, we increment the waitgroup so the reactor can stop
			// safely, and finally start the goroutine to broadcast txs to that peer.
			_, ok := r.peerRoutines[peerUpdate.PeerID]
			if !ok {
				closer := tmsync.NewCloser()

				r.peerRoutines[peerUpdate.PeerID] = closer
				r.peerWG.Add(1)

				r.ids.ReserveForPeer(peerUpdate.PeerID)

				// start a broadcast routine ensuring all txs are forwarded to the peer
				go r.broadcastTxRoutine(peerUpdate.PeerID, closer)
			}
		}

	case p2p.PeerStatusDown, p2p.PeerStatusRemoved, p2p.PeerStatusBanned:
		r.ids.Reclaim(peerUpdate.PeerID)

		// Check if we've started a tx broadcasting goroutine for this peer.
		// If we have, we signal to terminate the goroutine via the channel's closure.
		// This will internally decrement the peer waitgroup and remove the peer
		// from the map of peer tx broadcasting goroutines.
		closer, ok := r.peerRoutines[peerUpdate.PeerID]
		if ok {
			closer.Close()
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

func (r *Reactor) broadcastTxRoutine(peerID p2p.NodeID, closer *tmsync.Closer) {
	peerMempoolID := r.ids.GetForPeer(peerID)
	var next *clist.CElement

	// remove the peer ID from the map of routines and mark the waitgroup as done
	defer func() {
		r.mtx.Lock()
		delete(r.peerRoutines, peerID)
		r.mtx.Unlock()

		r.peerWG.Done()

		if e := recover(); e != nil {
			r.Logger.Error("recovering from broadcasting mempool loop", "err", e)
		}
	}()

	for {
		if !r.IsRunning() {
			return
		}

		// This happens because the CElement we were looking at got garbage
		// collected (removed). That is, .NextWait() returned nil. Go ahead and
		// start from the beginning.
		if next == nil {
			select {
			case <-r.mempool.TxsWaitChan(): // wait until a tx is available
				if next = r.mempool.TxsFront(); next == nil {
					continue
				}

			case <-closer.Done():
				// The peer is marked for removal via a PeerUpdate as the doneCh was
				// explicitly closed to signal we should exit.
				return

			case <-r.closeCh:
				// The reactor has signaled that we are stopped and thus we should
				// implicitly exit this peer's goroutine.
				return
			}
		}

		memTx := next.Value.(*mempoolTx)

		if r.peerMgr != nil {
			height := r.peerMgr.GetHeight(peerID)
			if height > 0 && height < memTx.Height()-1 {
				// allow for a lag of one block
				time.Sleep(peerCatchupSleepIntervalMS * time.Millisecond)
				continue
			}
		}

		// NOTE: Transaction batching was disabled due to:
		// https://github.com/tendermint/tendermint/issues/5796

		if _, ok := memTx.senders.Load(peerMempoolID); !ok {
			// Send the mempool tx to the corresponding peer. Note, the peer may be
			// behind and thus would not be able to process the mempool tx correctly.
			r.mempoolCh.Out() <- p2p.Envelope{
				To: peerID,
				Message: &protomem.Txs{
					Txs: [][]byte{memTx.tx},
				},
			}
			r.Logger.Debug("gossiped tx to peer", "tx", txID(memTx.tx), "peer", peerID)
		}

		select {
		case <-next.NextWaitChan():
			// see the start of the for loop for nil check
			next = next.Next()

		case <-closer.Done():
			// The peer is marked for removal via a PeerUpdate as the doneCh was
			// explicitly closed to signal we should exit.
			return

		case <-r.closeCh:
			// The reactor has signaled that we are stopped and thus we should
			// implicitly exit this peer's goroutine.
			return
		}
	}
}
