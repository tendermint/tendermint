package evidence

import (
	"fmt"
	"reflect"
	"time"

	amino "github.com/tendermint/go-amino"

	clist "github.com/tendermint/tendermint/libs/clist"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/p2p"
	"github.com/tendermint/tendermint/types"
)

const (
	EvidenceChannel = byte(0x38)

	maxMsgSize = 24576 // 24KB  - 50 pieces of evidence per msg TODO make it configurable

	// interval with which the reactor will attempt to broadcast evidence. This also means
	// the max time between
	broadcastEvidenceIntervalS = 1
)

// Reactor handles evpool evidence broadcasting amongst peers.
type Reactor struct {
	p2p.BaseReactor
	evpool   *Pool
	eventBus *types.EventBus
}

// NewReactor returns a new Reactor with the given config and evpool.
func NewReactor(evpool *Pool) *Reactor {
	evR := &Reactor{
		evpool: evpool,
	}
	evR.BaseReactor = *p2p.NewBaseReactor("Evidence", evR)
	return evR
}

// SetLogger sets the Logger on the reactor and the underlying Evidence.
func (evR *Reactor) SetLogger(l log.Logger) {
	evR.Logger = l
	evR.evpool.SetLogger(l)
}

// GetChannels implements Reactor.
// It returns the list of channels for this reactor.
func (evR *Reactor) GetChannels() []*p2p.ChannelDescriptor {
	return []*p2p.ChannelDescriptor{
		{
			ID:       EvidenceChannel,
			Priority: 5,
		},
	}
}

// AddPeer implements Reactor.
func (evR *Reactor) AddPeer(peer p2p.Peer) {
	go evR.broadcastEvidenceRoutine(peer)
}

// Receive implements Reactor.
// It adds any received evidence to the evpool.
func (evR *Reactor) Receive(chID byte, src p2p.Peer, msgBytes []byte) {
	msg, err := decodeMsg(msgBytes)
	if err != nil {
		evR.Logger.Error("Error decoding message", "src", src, "chId", chID, "msg", msg, "err", err, "bytes", msgBytes)
		evR.Switch.StopPeerForError(src, err)
		return
	}

	if err = msg.ValidateBasic(); err != nil {
		evR.Logger.Error("Peer sent us invalid msg", "peer", src, "msg", msg, "err", err)
		evR.Switch.StopPeerForError(src, err)
		return
	}

	evR.Logger.Debug("Receive", "src", src, "chId", chID, "msg", msg)

	switch msg := msg.(type) {
	case *ListMessage:
		for _, ev := range msg.Evidence {
			err := evR.evpool.AddEvidence(ev)
			switch err.(type) {
			case ErrInvalidEvidence:
				evR.Logger.Error("Evidence is not valid", "evidence", msg.Evidence, "err", err)
				// punish peer
				evR.Switch.StopPeerForError(src, err)
				return
			case nil:
			default:
				evR.Logger.Error("Evidence has not been added", "evidence", msg.Evidence, "err", err)
				return
			}
		}
	default:
		evR.Logger.Error(fmt.Sprintf("Unknown message type %v", reflect.TypeOf(msg)))
	}
}

// SetEventBus implements events.Eventable.
func (evR *Reactor) SetEventBus(b *types.EventBus) {
	evR.eventBus = b
}

// Modeled after the mempool routine.
// - Evidence accumulates in a clist.
// - Each peer has a routine that iterates through the clist,
// sending available evidence to the peer.
// - If we're waiting for new evidence and the list is not empty,
// start iterating from the beginning again.
func (evR *Reactor) broadcastEvidenceRoutine(peer p2p.Peer) {
	var next *clist.CElement
	messageSize := int(maxMsgSize / types.MaxEvidenceBytes)
	for {
		// This happens because the CElement we were looking at got garbage
		// collected (removed). That is, .NextWait() returned nil. Go ahead and
		// start from the beginning.
		if next == nil {
			select {
			case <-evR.evpool.EvidenceWaitChan(): // Wait until evidence is available
				if next = evR.evpool.EvidenceFront(); next == nil {
					continue
				}
			case <-peer.Quit():
				return
			case <-evR.Quit():
				return
			}
		}

		peerState, ok := peer.Get(types.PeerStateKey).(PeerState)
		if !ok {
			// Peer does not have a state yet. We set it in the consensus reactor, but
			// when we add peer in Switch, the order we call reactors#AddPeer is
			// different every time due to us using a map. Sometimes other reactors
			// will be initialized before the consensus reactor. We should wait a few
			// milliseconds and retry.
			time.Sleep(broadcastEvidenceIntervalS * time.Second)
			continue
		}

		var evMsg ListMessage
		for next != nil && len(evMsg.Evidence) < messageSize {
			ev := next.Value.(types.Evidence)
			// check to see if peer is behind, if so jump to next piece of evidence
			if ev.Height() <= peerState.GetHeight() {
				// check that the evidence has not expired else remove it
				if evR.evpool.IsExpired(ev) {
					evR.evpool.removePendingEvidence(ev)
					evR.evpool.evidenceList.Remove(next)
					next.DetachPrev()
				} else {
					evMsg.Evidence = append(evMsg.Evidence, ev)
				}
			}
			// As evidence arrives in batches, wait for a period to see if there is
			// more evidence to come, if not then finish the msg and send it to the peer.
			// (10ms is the allocated time for each loop of AddEvidence)
			afterCh := time.After(time.Millisecond * 10)
			select {
			case <-afterCh:
				next = nil
			case <-next.NextWaitChan():
				next = next.Next()
			case <-peer.Quit():
				return
			case <-evR.Quit():
				return
			}
		}

		success := peer.Send(EvidenceChannel, cdc.MustMarshalBinaryBare(evMsg))

		// if failed to send then wait before trying again
		if next == nil || !success {
			time.Sleep(broadcastEvidenceIntervalS * time.Second)
		}

	}
}

// PeerState describes the state of a peer.
type PeerState interface {
	GetHeight() int64
}

//-----------------------------------------------------------------------------
// Messages

// Message is a message sent or received by the Reactor.
type Message interface {
	ValidateBasic() error
}

func RegisterMessages(cdc *amino.Codec) {
	cdc.RegisterInterface((*Message)(nil), nil)
	cdc.RegisterConcrete(&ListMessage{},
		"tendermint/evidence/ListMessage", nil)
}

func decodeMsg(bz []byte) (msg Message, err error) {
	if len(bz) > maxMsgSize {
		return msg, fmt.Errorf("msg exceeds max size (%d > %d)", len(bz), maxMsgSize)
	}
	err = cdc.UnmarshalBinaryBare(bz, &msg)
	return
}

//-------------------------------------

// ListMessage contains a list of evidence.
type ListMessage struct {
	Evidence []types.Evidence
}

// ValidateBasic performs basic validation.
func (m *ListMessage) ValidateBasic() error {
	for i, ev := range m.Evidence {
		if err := ev.ValidateBasic(); err != nil {
			return fmt.Errorf("invalid evidence (#%d): %v", i, err)
		}
	}
	return nil
}

// String returns a string representation of the ListMessage.
func (m *ListMessage) String() string {
	return fmt.Sprintf("[ListMessage %v]", m.Evidence)
}
