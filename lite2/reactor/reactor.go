package reactor

import (
	"fmt"

	"github.com/pkg/errors"
	amino "github.com/tendermint/go-amino"
	db "github.com/tendermint/tm-db"

	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/p2p"
	"github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/store"
	"github.com/tendermint/tendermint/types"
)

const (
	LiteChannel = byte(0x10)
	maxMsgSize  = int(1e6) // FIXME Is this sufficient?
)

// Reactor handles data exchange for light clients across P2P, via lite2/provider/p2p.
type Reactor struct {
	p2p.BaseReactor

	blockStore *store.BlockStore
	stateDB    db.DB
	dispatcher *Dispatcher
	logger     log.Logger
}

// NewReactor creates a new light client reactor.
func NewReactor(bs *store.BlockStore, stateDB db.DB) *Reactor {
	return &Reactor{
		blockStore: bs,
		stateDB:    stateDB,
		dispatcher: NewDispatcher(),
	}
}

// AddPeer implements Reactor.
func (r *Reactor) AddPeer(peer p2p.Peer) {
}

// RemovePeer implements Reactor.
func (r *Reactor) RemovePeer(peer p2p.Peer, reason interface{}) {
}

// GetChannels implements Reactor.
func (r *Reactor) GetChannels() []*p2p.ChannelDescriptor {
	return []*p2p.ChannelDescriptor{
		{
			ID:                  LiteChannel,
			Priority:            1,
			SendQueueCapacity:   1000,
			RecvBufferCapacity:  4e6,
			RecvMessageCapacity: maxMsgSize,
		},
	}
}

// Receive implements Reactor.
func (r *Reactor) Receive(chID byte, src p2p.Peer, msgBytes []byte) {
	msg, err := decodeMsg(msgBytes)
	if err != nil {
		r.logger.Error("error decoding message",
			"src", src.ID(), "chId", chID, "msg", msg, "err", err, "bytes", msgBytes)
		return
	}

	err = msg.ValidateBasic()
	if err != nil {
		r.logger.Error("peer sent invalid msg", "peer", src, "msg", msg, "err", err)
		return
	}

	r.logger.Debug("Receive", "src", src.ID(), "chID", chID, "msg", msg)

	switch msg := msg.(type) {
	case *signedHeaderRequestMessage:
		height := msg.Height
		storeHeight := r.blockStore.Height()
		if height == 0 {
			height = storeHeight
		}

		blockMeta := r.blockStore.LoadBlockMeta(height)
		if blockMeta == nil {
			r.logger.Debug("Block meta not found", "height", height)
		}

		var commit *types.Commit
		if height == storeHeight {
			commit = r.blockStore.LoadSeenCommit(height)
		} else {
			commit = r.blockStore.LoadBlockCommit(height)
		}
		if commit == nil {
			r.logger.Debug("Commit not found", "height", msg.Height)
		}

		var signedHeader *types.SignedHeader
		if blockMeta != nil && commit != nil {
			signedHeader = &types.SignedHeader{
				Header: &blockMeta.Header,
				Commit: commit,
			}
		}
		src.Send(LiteChannel, cdc.MustMarshalBinaryBare(&signedHeaderResponseMessage{
			CallID:       msg.GetCallID(),
			SignedHeader: signedHeader,
		}))

	case *validatorSetRequestMessage:
		vals, err := state.LoadValidators(r.stateDB, msg.Height)
		if err != nil {
			if _, ok := err.(state.ErrNoValSetForHeight); ok {
				r.logger.Debug("Validator set not found", "height", msg.Height)
			} else {
				r.logger.Error("Failed to fetch validator set", "height", msg.Height, "err", err)
			}
			vals = nil
		}
		src.Send(LiteChannel, cdc.MustMarshalBinaryBare(&validatorSetResponseMessage{
			CallID:       msg.GetCallID(),
			ValidatorSet: vals,
		}))

	case *signedHeaderResponseMessage:
		err := r.dispatcher.respond(src, msg)
		if err != nil {
			r.logger.Error("Invalid signed header response", "err", err.Error())
		}

	case *validatorSetResponseMessage:
		err := r.dispatcher.respond(src, msg)
		if err != nil {
			r.logger.Error("Invalid validator set response", "err", err.Error())
		}
	}
}

// SetLogger sets the logger of the reactor.
func (r *Reactor) SetLogger(logger log.Logger) {
	r.BaseReactor.SetLogger(logger)
	r.logger = logger
}

// Start implements Servive.
func (r *Reactor) Start() error {
	r.logger.Info("Starting light client reactor")
	return nil
}

// Stop implements Servive.
func (r *Reactor) Stop() error {
	r.logger.Info("Stopping light client reactor")
	return nil
}

// Dispatcher returns the dispatcher.
func (r *Reactor) Dispatcher() *Dispatcher {
	return r.dispatcher
}

// Message is a generic message for this reactor.
type Message interface {
	GetCallID() uint64
	SetCallID(uint64)
	ValidateBasic() error
}

func decodeMsg(bz []byte) (msg Message, err error) {
	if len(bz) > maxMsgSize {
		return msg, fmt.Errorf("msg exceeds max size (%d > %d)", len(bz), maxMsgSize)
	}
	err = cdc.UnmarshalBinaryBare(bz, &msg)
	return
}

// RegisterMessages registers light client P2P messages
func RegisterMessages(cdc *amino.Codec) {
	cdc.RegisterInterface((*Message)(nil), nil)
	cdc.RegisterConcrete(&signedHeaderRequestMessage{}, "tendermint/lite2/SignedHeaderRequest", nil)
	cdc.RegisterConcrete(&signedHeaderResponseMessage{}, "tendermint/lite2/SignedHeaderResponse", nil)
	cdc.RegisterConcrete(&validatorSetRequestMessage{}, "tendermint/lite2/ValidatorSetRequest", nil)
	cdc.RegisterConcrete(&validatorSetResponseMessage{}, "tendermint/lite2/ValidatorSetResponse", nil)
}

type signedHeaderRequestMessage struct {
	CallID uint64
	Height int64
}

func (m *signedHeaderRequestMessage) GetCallID() uint64   { return m.CallID }
func (m *signedHeaderRequestMessage) SetCallID(id uint64) { m.CallID = id }
func (m *signedHeaderRequestMessage) ValidateBasic() error {
	if m.Height < 0 {
		return errors.New("height cannot be negative")
	}
	return nil
}

type signedHeaderResponseMessage struct {
	CallID       uint64
	SignedHeader *types.SignedHeader
}

func (m *signedHeaderResponseMessage) GetCallID() uint64    { return m.CallID }
func (m *signedHeaderResponseMessage) SetCallID(id uint64)  { m.CallID = id }
func (m *signedHeaderResponseMessage) ValidateBasic() error { return nil }

type validatorSetRequestMessage struct {
	CallID uint64
	Height int64
}

func (m *validatorSetRequestMessage) GetCallID() uint64   { return m.CallID }
func (m *validatorSetRequestMessage) SetCallID(id uint64) { m.CallID = id }
func (m *validatorSetRequestMessage) ValidateBasic() error {
	if m.Height < 0 {
		return errors.New("height cannot be negative")
	}
	return nil
}

type validatorSetResponseMessage struct {
	CallID       uint64
	ValidatorSet *types.ValidatorSet
}

func (m *validatorSetResponseMessage) GetCallID() uint64    { return m.CallID }
func (m *validatorSetResponseMessage) SetCallID(id uint64)  { m.CallID = id }
func (m *validatorSetResponseMessage) ValidateBasic() error { return nil }
