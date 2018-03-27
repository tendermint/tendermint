package evidence

import (
	"bytes"
	"fmt"
	"reflect"
	"time"

	wire "github.com/tendermint/go-wire"
	"github.com/tendermint/tmlibs/log"

	"github.com/tendermint/tendermint/p2p"
	"github.com/tendermint/tendermint/types"
)

const (
	EvidenceChannel = byte(0x38)

	maxEvidenceMessageSize     = 1048576 // 1MB TODO make it configurable
	broadcastEvidenceIntervalS = 60      // broadcast uncommitted evidence this often
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

// OnStart implements cmn.Service
func (evR *EvidenceReactor) OnStart() error {
	if err := evR.BaseReactor.OnStart(); err != nil {
		return err
	}
	go evR.broadcastRoutine()
	return nil
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
	// send the peer our high-priority evidence.
	// the rest will be sent by the broadcastRoutine
	evidences := evR.evpool.PriorityEvidence()
	msg := &EvidenceListMessage{evidences}
	success := peer.Send(EvidenceChannel, struct{ EvidenceMessage }{msg})
	if !success {
		// TODO: remove peer ?
	}
}

// RemovePeer implements Reactor.
func (evR *EvidenceReactor) RemovePeer(peer p2p.Peer, reason interface{}) {
	// nothing to do
}

// Receive implements Reactor.
// It adds any received evidence to the evpool.
func (evR *EvidenceReactor) Receive(chID byte, src p2p.Peer, msgBytes []byte) {
	_, msg, err := DecodeMessage(msgBytes)
	if err != nil {
		evR.Logger.Error("Error decoding message", "src", src, "chId", chID, "msg", msg, "err", err, "bytes", msgBytes)
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

// Broadcast new evidence to all peers.
// Broadcasts must be non-blocking so routine is always available to read off EvidenceChan.
func (evR *EvidenceReactor) broadcastRoutine() {
	ticker := time.NewTicker(time.Second * broadcastEvidenceIntervalS)
	for {
		select {
		case evidence := <-evR.evpool.EvidenceChan():
			// broadcast some new evidence
			msg := &EvidenceListMessage{[]types.Evidence{evidence}}
			evR.Switch.Broadcast(EvidenceChannel, struct{ EvidenceMessage }{msg})

			// TODO: Broadcast runs asynchronously, so this should wait on the successChan
			// in another routine before marking to be proper.
			evR.evpool.evidenceStore.MarkEvidenceAsBroadcasted(evidence)
		case <-ticker.C:
			// broadcast all pending evidence
			msg := &EvidenceListMessage{evR.evpool.PendingEvidence()}
			evR.Switch.Broadcast(EvidenceChannel, struct{ EvidenceMessage }{msg})
		case <-evR.Quit():
			return
		}
	}
}

//-----------------------------------------------------------------------------
// Messages

const (
	msgTypeEvidence = byte(0x01)
)

// EvidenceMessage is a message sent or received by the EvidenceReactor.
type EvidenceMessage interface{}

var _ = wire.RegisterInterface(
	struct{ EvidenceMessage }{},
	wire.ConcreteType{&EvidenceListMessage{}, msgTypeEvidence},
)

// DecodeMessage decodes a byte-array into a EvidenceMessage.
func DecodeMessage(bz []byte) (msgType byte, msg EvidenceMessage, err error) {
	msgType = bz[0]
	n := new(int)
	r := bytes.NewReader(bz)
	msg = wire.ReadBinary(struct{ EvidenceMessage }{}, r, maxEvidenceMessageSize, n, &err).(struct{ EvidenceMessage }).EvidenceMessage
	return
}

//-------------------------------------

// EvidenceMessage contains a list of evidence.
type EvidenceListMessage struct {
	Evidence []types.Evidence
}

// String returns a string representation of the EvidenceListMessage.
func (m *EvidenceListMessage) String() string {
	return fmt.Sprintf("[EvidenceListMessage %v]", m.Evidence)
}
