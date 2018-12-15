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

	maxMsgSize = 1048576 // 1MB TODO make it configurable

	broadcastEvidenceIntervalS = 60  // broadcast uncommitted evidence this often
	peerCatchupSleepIntervalMS = 100 // If peer is behind, sleep this amount
)

// EvidenceReactor handles evpool evidence broadcasting amongst peers.
type EvidenceReactor struct {
	p2p.BaseReactor
	evpool   *EvidencePool
	eventBus *types.EventBus
}

// NewEvidenceReactor returns a new EvidenceReactor with the given config and evpool.
func NewEvidenceReactor(evpool *EvidencePool) *EvidenceReactor {
	evR := &EvidenceReactor{
		evpool: evpool,
	}
	evR.BaseReactor = *p2p.NewBaseReactor("EvidenceReactor", evR)
	return evR
}

// SetLogger sets the Logger on the reactor and the underlying Evidence.
func (evR *EvidenceReactor) SetLogger(l log.Logger) {
	evR.Logger = l
	evR.evpool.SetLogger(l)
}

// GetChannels implements Reactor.
// It returns the list of channels for this reactor.
func (evR *EvidenceReactor) GetChannels() []*p2p.ChannelDescriptor {
	return []*p2p.ChannelDescriptor{
		&p2p.ChannelDescriptor{
			ID:       EvidenceChannel,
			Priority: 5,
		},
	}
}

// AddPeer implements Reactor.
func (evR *EvidenceReactor) AddPeer(peer p2p.Peer) {
	go evR.broadcastEvidenceRoutine(peer)
}

// RemovePeer implements Reactor.
func (evR *EvidenceReactor) RemovePeer(peer p2p.Peer, reason interface{}) {
	// nothing to do
}

// Receive implements Reactor.
// It adds any received evidence to the evpool.
func (evR *EvidenceReactor) Receive(chID byte, src p2p.Peer, msgBytes []byte) {
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
	case *EvidenceListMessage:
		for _, ev := range msg.Evidence {
			err := evR.evpool.AddEvidence(ev)
			if err != nil {
				evR.Logger.Info("Evidence is not valid", "evidence", msg.Evidence, "err", err)
				// punish peer
				evR.Switch.StopPeerForError(src, err)
			}
		}
	default:
		evR.Logger.Error(fmt.Sprintf("Unknown message type %v", reflect.TypeOf(msg)))
	}
}

// SetEventSwitch implements events.Eventable.
func (evR *EvidenceReactor) SetEventBus(b *types.EventBus) {
	evR.eventBus = b
}

// Modeled after the mempool routine.
// - Evidence accumulates in a clist.
// - Each peer has a routien that iterates through the clist,
// sending available evidence to the peer.
// - If we're waiting for new evidence and the list is not empty,
// start iterating from the beginning again.
func (evR *EvidenceReactor) broadcastEvidenceRoutine(peer p2p.Peer) {
	var next *clist.CElement
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

		ev := next.Value.(types.Evidence)
		msg, retry := evR.checkSendEvidenceMessage(peer, ev)
		if msg != nil {
			success := peer.Send(EvidenceChannel, cdc.MustMarshalBinaryBare(msg))
			retry = !success
		}

		if retry {
			time.Sleep(peerCatchupSleepIntervalMS * time.Millisecond)
			continue
		}

		afterCh := time.After(time.Second * broadcastEvidenceIntervalS)
		select {
		case <-afterCh:
			// start from the beginning every tick.
			// TODO: only do this if we're at the end of the list!
			next = nil
		case <-next.NextWaitChan():
			// see the start of the for loop for nil check
			next = next.Next()
		case <-peer.Quit():
			return
		case <-evR.Quit():
			return
		}
	}
}

// Returns the message to send the peer, or nil if the evidence is invalid for the peer.
// If message is nil, return true if we should sleep and try again.
func (evR EvidenceReactor) checkSendEvidenceMessage(peer p2p.Peer, ev types.Evidence) (msg EvidenceMessage, retry bool) {
	// make sure the peer is up to date
	evHeight := ev.Height()
	peerState, ok := peer.Get(types.PeerStateKey).(PeerState)
	if !ok {
		// Peer does not have a state yet. We set it in the consensus reactor, but
		// when we add peer in Switch, the order we call reactors#AddPeer is
		// different every time due to us using a map. Sometimes other reactors
		// will be initialized before the consensus reactor. We should wait a few
		// milliseconds and retry.
		return nil, true
	}

	// NOTE: We only send evidence to peers where
	// peerHeight - maxAge < evidenceHeight < peerHeight
	maxAge := evR.evpool.State().ConsensusParams.Evidence.MaxAge
	peerHeight := peerState.GetHeight()
	if peerHeight < evHeight {
		// peer is behind. sleep while he catches up
		return nil, true
	} else if peerHeight > evHeight+maxAge {
		// evidence is too old, skip
		// NOTE: if evidence is too old for an honest peer,
		// then we're behind and either it already got committed or it never will!
		evR.Logger.Info("Not sending peer old evidence", "peerHeight", peerHeight, "evHeight", evHeight, "maxAge", maxAge, "peer", peer)
		return nil, false
	}

	// send evidence
	msg = &EvidenceListMessage{[]types.Evidence{ev}}
	return msg, false
}

// PeerState describes the state of a peer.
type PeerState interface {
	GetHeight() int64
}

//-----------------------------------------------------------------------------
// Messages

// EvidenceMessage is a message sent or received by the EvidenceReactor.
type EvidenceMessage interface {
	ValidateBasic() error
}

func RegisterEvidenceMessages(cdc *amino.Codec) {
	cdc.RegisterInterface((*EvidenceMessage)(nil), nil)
	cdc.RegisterConcrete(&EvidenceListMessage{},
		"tendermint/evidence/EvidenceListMessage", nil)
}

func decodeMsg(bz []byte) (msg EvidenceMessage, err error) {
	if len(bz) > maxMsgSize {
		return msg, fmt.Errorf("Msg exceeds max size (%d > %d)", len(bz), maxMsgSize)
	}
	err = cdc.UnmarshalBinaryBare(bz, &msg)
	return
}

//-------------------------------------

// EvidenceListMessage contains a list of evidence.
type EvidenceListMessage struct {
	Evidence []types.Evidence
}

// ValidateBasic performs basic validation.
func (m *EvidenceListMessage) ValidateBasic() error {
	for i, ev := range m.Evidence {
		if err := ev.ValidateBasic(); err != nil {
			return fmt.Errorf("Invalid evidence (#%d): %v", i, err)
		}
	}
	return nil
}

// String returns a string representation of the EvidenceListMessage.
func (m *EvidenceListMessage) String() string {
	return fmt.Sprintf("[EvidenceListMessage %v]", m.Evidence)
}
