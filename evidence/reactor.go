package evpool

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
	EvidencePoolChannel = byte(0x38)

	maxEvidencePoolMessageSize = 1048576 // 1MB TODO make it configurable
	peerCatchupSleepIntervalMS = 100     // If peer is behind, sleep this amount
	broadcastEvidenceIntervalS = 60      // broadcast uncommitted evidence this often
)

// EvidencePoolReactor handles evpool evidence broadcasting amongst peers.
type EvidencePoolReactor struct {
	p2p.BaseReactor
	config *EvidencePoolConfig
	evpool *EvidencePool
	evsw   types.EventSwitch
}

// NewEvidencePoolReactor returns a new EvidencePoolReactor with the given config and evpool.
func NewEvidencePoolReactor(config *EvidencePoolConfig, evpool *EvidencePool) *EvidencePoolReactor {
	evR := &EvidencePoolReactor{
		config: config,
		evpool: evpool,
	}
	evR.BaseReactor = *p2p.NewBaseReactor("EvidencePoolReactor", evR)
	return evR
}

// SetLogger sets the Logger on the reactor and the underlying EvidencePool.
func (evR *EvidencePoolReactor) SetLogger(l log.Logger) {
	evR.Logger = l
	evR.evpool.SetLogger(l)
}

// OnStart implements cmn.Service
func (evR *EvidencePoolReactor) OnStart() error {
	if err := evR.BaseReactor.OnStart(); err != nil {
		return err
	}
	go evR.broadcastRoutine()
	return nil
}

// GetChannels implements Reactor.
// It returns the list of channels for this reactor.
func (evR *EvidencePoolReactor) GetChannels() []*p2p.ChannelDescriptor {
	return []*p2p.ChannelDescriptor{
		&p2p.ChannelDescriptor{
			ID:       EvidencePoolChannel,
			Priority: 5,
		},
	}
}

// AddPeer implements Reactor.
func (evR *EvidencePoolReactor) AddPeer(peer p2p.Peer) {
	// first send the peer high-priority evidence
	evidence := evR.evpool.PriorityEvidence()
	msg := EvidenceMessage{evidence}
	success := peer.Send(EvidencePoolChannel, struct{ EvidencePoolMessage }{msg})
	if !success {
		// TODO: remove peer ?
	}

	// TODO: send the remaining pending evidence
	// or just let the broadcastRoutine do it ?
}

// RemovePeer implements Reactor.
func (evR *EvidencePoolReactor) RemovePeer(peer p2p.Peer, reason interface{}) {
}

// Receive implements Reactor.
// It adds any received evidence to the evpool.
func (evR *EvidencePoolReactor) Receive(chID byte, src p2p.Peer, msgBytes []byte) {
	_, msg, err := DecodeMessage(msgBytes)
	if err != nil {
		evR.Logger.Error("Error decoding message", "err", err)
		return
	}
	evR.Logger.Debug("Receive", "src", src, "chId", chID, "msg", msg)

	switch msg := msg.(type) {
	case *EvidenceMessage:
		for _, ev := range msg.Evidence {
			err := evR.evpool.AddEvidence(ev)
			if err != nil {
				evR.Logger.Info("Evidence is not valid", "evidence", msg.Evidence, "err", err)
				// TODO: punish peer
			}
		}
	default:
		evR.Logger.Error(fmt.Sprintf("Unknown message type %v", reflect.TypeOf(msg)))
	}
}

// SetEventSwitch implements events.Eventable.
func (evR *EvidencePoolReactor) SetEventSwitch(evsw types.EventSwitch) {
	evR.evsw = evsw
}

// broadcast new evidence to all peers
func (evR *EvidencePoolReactor) broadcastRoutine() {
	ticker := time.NewTicker(time.Second * broadcastEvidenceIntervalS)
	for {
		select {
		case evidence := <-evR.evpool.NewEvidenceChan():
			// broadcast some new evidence
			msg := EvidenceMessage{[]types.Evidence{evidence}}
			evR.Switch.Broadcast(EvidencePoolChannel, struct{ EvidencePoolMessage }{msg})

			// NOTE: Broadcast runs asynchronously, so this should wait on the successChan
			// in another routine before marking to be proper.
			idx := 1 // TODO
			evR.evpool.evidenceStore.MarkEvidenceAsBroadcasted(idx, evidence)
		case <-ticker.C:
			// broadcast all pending evidence
			msg := EvidenceMessage{evR.evpool.PendingEvidence()}
			evR.Switch.Broadcast(EvidencePoolChannel, struct{ EvidencePoolMessage }{msg})
		case <-evR.Quit:
			return
		}
	}
}

//-----------------------------------------------------------------------------
// Messages

const (
	msgTypeEvidence = byte(0x01)
)

// EvidencePoolMessage is a message sent or received by the EvidencePoolReactor.
type EvidencePoolMessage interface{}

var _ = wire.RegisterInterface(
	struct{ EvidencePoolMessage }{},
	wire.ConcreteType{&EvidenceMessage{}, msgTypeEvidence},
)

// DecodeMessage decodes a byte-array into a EvidencePoolMessage.
func DecodeMessage(bz []byte) (msgType byte, msg EvidencePoolMessage, err error) {
	msgType = bz[0]
	n := new(int)
	r := bytes.NewReader(bz)
	msg = wire.ReadBinary(struct{ EvidencePoolMessage }{}, r, maxEvidencePoolMessageSize, n, &err).(struct{ EvidencePoolMessage }).EvidencePoolMessage
	return
}

//-------------------------------------

// EvidenceMessage contains a list of evidence.
type EvidenceMessage struct {
	Evidence []types.Evidence
}

// String returns a string representation of the EvidenceMessage.
func (m *EvidenceMessage) String() string {
	return fmt.Sprintf("[EvidenceMessage %v]", m.Evidence)
}
