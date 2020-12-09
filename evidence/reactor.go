package evidence

import (
	"fmt"
	"time"

	clist "github.com/tendermint/tendermint/libs/clist"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/libs/service"
	tmsync "github.com/tendermint/tendermint/libs/sync"
	"github.com/tendermint/tendermint/p2p"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	"github.com/tendermint/tendermint/types"
)

var (
	_ service.Service = (*Reactor)(nil)

	// ChannelShims contains a map of ChannelDescriptorShim objects, where each
	// object wraps a reference to a legacy p2p ChannelDescriptor and the corresponding
	// p2p proto.Message the new p2p Channel is responsible for handling.
	//
	//
	// TODO: Remove once p2p refactor is complete.
	// ref: https://github.com/tendermint/tendermint/issues/5670
	ChannelShims = map[p2p.ChannelID]*p2p.ChannelDescriptorShim{
		EvidenceChannel: {
			MsgType: new(tmproto.EvidenceList),
			Descriptor: &p2p.ChannelDescriptor{
				ID:                  byte(EvidenceChannel),
				Priority:            5,
				RecvMessageCapacity: maxMsgSize,
			},
		},
	}
)

const (
	EvidenceChannel = p2p.ChannelID(0x38)

	maxMsgSize = 1048576 // 1MB TODO make it configurable

	// broadcast all uncommitted evidence this often. This sets when the reactor
	// goes back to the start of the list and begins sending the evidence again.
	// Most evidence should be committed in the very next block that is why we wait
	// just over the block production rate before sending evidence again.
	broadcastEvidenceIntervalS = 10
)

// Reactor handles evpool evidence broadcasting amongst peers.
type Reactor struct {
	service.BaseService

	evpool          *Pool
	eventBus        *types.EventBus
	evidenceCh      *p2p.Channel
	evidenceChDone  chan bool
	peerUpdates     p2p.PeerUpdates
	peerUpdatesDone chan bool

	mtx          tmsync.RWMutex
	peerRoutines map[string]chan struct{}
}

// NewReactor returns a new Reactor with the given config and evpool.
func NewReactor(
	logger log.Logger,
	evidenceCh *p2p.Channel,
	peerUpdates p2p.PeerUpdates,
	evpool *Pool,
) *Reactor {
	r := &Reactor{
		evpool:          evpool,
		evidenceCh:      evidenceCh,
		evidenceChDone:  make(chan bool),
		peerUpdates:     peerUpdates,
		peerUpdatesDone: make(chan bool),
		peerRoutines:    make(map[string]chan struct{}),
	}

	r.BaseService = *service.NewBaseService(logger, "Evidence", r)
	return r
}

// SetEventBus implements events.Eventable.
func (r *Reactor) SetEventBus(b *types.EventBus) {
	r.eventBus = b
}

// OnStart starts separate go routines for each p2p Channel and listens for
// envelopes on each. In addition, it also listens for peer updates and handles
// messages on that p2p channel accordingly. The caller must be sure to execute
// OnStop to ensure the outbound p2p Channels are closed. No error is returned.
func (r *Reactor) OnStart() error {
	go r.processEvidenceCh()
	go r.processPeerUpdates()

	return nil
}

// OnStop stops the reactor by signaling to all spawned goroutines to exit and
// blocking until they all exit. Finally, it will close the evidence p2p Channel.
func (r *Reactor) OnStop() {
	// Wait for all goroutines to safely exit before proceeding to close all p2p
	// Channels. After closing, the router can be signaled that it is safe to stop
	// sending on the inbound In channel and close it.
	r.evidenceChDone <- true
	r.peerUpdatesDone <- true

	if err := r.evidenceCh.Close(); err != nil {
		panic(fmt.Sprintf("failed to close evidence channel: %s", err))
	}
}

// processEvidenceCh implements a blocking event loop where we listen for p2p
// Envelope messages from the evidenceCh.
func (r *Reactor) processEvidenceCh() {
	for {
		select {
		case envelope := <-r.evidenceCh.In:
			switch msg := envelope.Message.(type) {
			case *tmproto.EvidenceList:
				r.Logger.Debug(
					"received evidence list",
					"num_evidence", len(msg.Evidence),
					"peer", envelope.From.String(),
				)

				for _, protoEv := range msg.Evidence {
					ev, err := types.EvidenceFromProto(&protoEv)
					if err != nil {
						r.Logger.Error("failed to convert evidence", "peer", envelope.From.String(), "err", err)
						continue
					}

					if err := r.evpool.AddEvidence(ev); err != nil {
						r.Logger.Error("failed to add evidence", "peer", envelope.From.String(), "err", err)

						// If we're given invalid evidence by the peer, notify the router
						// that we should remove this peer.
						if _, ok := err.(*types.ErrInvalidEvidence); ok {
							r.evidenceCh.Error <- p2p.PeerError{
								PeerID:   envelope.From,
								Err:      err,
								Severity: p2p.PeerErrorSeverityLow,
							}
						}
					}
				}

			default:
				r.Logger.Error("received unknown message", "msg", msg, "peer", envelope.From.String())
				r.evidenceCh.Error <- p2p.PeerError{
					PeerID:   envelope.From,
					Err:      fmt.Errorf("received unknown message: %T", msg),
					Severity: p2p.PeerErrorSeverityLow,
				}
			}

		case <-r.evidenceChDone:
			r.Logger.Debug("stopped listening on evidence channel")
			return
		}
	}
}

// processPeerUpdates starts a blocking process where we listen for peer updates.
// For each peer update of status PeerStatusNew or PeerStatusUp, we spawn a
// evidence broadcasting goroutine if one hasn't already been spawned. For each
// peer update of status PeerStatusDown, PeerStatusRemoved, or PeerStatusBanned
// we mark the broadcasting goroutine as done if it exists.
func (r *Reactor) processPeerUpdates() {
	for {
		select {
		case peerUpdate := <-r.peerUpdates:
			r.Logger.Debug("peer update", "peer", peerUpdate.PeerID.String())

			switch peerUpdate.Status {
			case p2p.PeerStatusNew, p2p.PeerStatusUp:
				r.mtx.Lock()

				// Check if we've already started a goroutine for this PeerID, if not
				// create a new done channel, update the peerRoutines, and start the
				// goroutine to broadcast evidence to that peer.
				_, ok := r.peerRoutines[peerUpdate.PeerID.String()]
				if !ok {
					doneCh := make(chan struct{})
					r.peerRoutines[peerUpdate.PeerID.String()] = doneCh
					go r.broadcastEvidenceLoop(peerUpdate.PeerID, doneCh)
				}

				r.mtx.Unlock()

			case p2p.PeerStatusDown, p2p.PeerStatusRemoved, p2p.PeerStatusBanned:
				r.mtx.Lock()

				doneCh, ok := r.peerRoutines[peerUpdate.PeerID.String()]
				if ok {
					close(doneCh)
					delete(r.peerRoutines, peerUpdate.PeerID.String())
				}

				r.mtx.Unlock()
			}

		case <-r.peerUpdatesDone:
			r.Logger.Debug("stopped listening on peer updates channel")
			return
		}
	}
}

// broadcastEvidenceLoop starts a blocking process that continuously reads
// evidence objects off of a linked-list and sends the evidence to the given
// peer. It is modeled after the mempool routine.
func (r *Reactor) broadcastEvidenceLoop(peerID p2p.PeerID, done <-chan struct{}) {
	var next *clist.CElement
	for {
		// This happens because the CElement we were looking at got garbage
		// collected (removed). That is, .NextWaitChan() returned nil. So we can go
		// ahead and start from the beginning.
		if next == nil {
			select {
			case <-r.evpool.EvidenceWaitChan(): // wait until evidence is available
				if next = r.evpool.EvidenceFront(); next == nil {
					continue
				}

			case <-done:
				// We can safely exit the goroutine once the peer is marked as removed
				// by processPeerUpdates closing the done channel.
				return

			case <-r.Quit():
				// we can safely exit the goroutine once the reactor stops
				return
			}
		}

		ev := next.Value.(types.Evidence)
		evProto, err := types.EvidenceToProto(ev)
		if err != nil {
			panic(err)
		}

		// Before attempting to gossip the evidence, ensure we haven't stopped the
		// reactor or received a signal marking the peer as removed.
		select {
		case <-done:
			// We can safely exit the goroutine once the peer is marked as removed
			// by processPeerUpdates closing the done channel.
			return

		case <-r.Quit():
			// we can safely exit the goroutine once the reactor stops
			return

		default:
			r.Logger.Debug("gossiping evidence to peer", "evidence", ev, "peer", peerID)
			r.evidenceCh.Out <- p2p.Envelope{
				To: peerID,
				Message: &tmproto.EvidenceList{
					Evidence: []tmproto.Evidence{*evProto},
				},
			}
		}

		select {
		case <-time.After(time.Second * broadcastEvidenceIntervalS):
			// start from the beginning every broadcastEvidenceIntervalS seconds
			next = nil

		case <-next.NextWaitChan():
			next = next.Next()

		case <-done:
			// We can safely exit the goroutine once the peer is marked as removed
			// by processPeerUpdates closing the done channel.
			return

		case <-r.Quit():
			// we can safely exit the goroutine once the reactor stops
			return
		}
	}
}
