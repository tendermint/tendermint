package reactor

import (
	"fmt"
	"sync"

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
	LiteChannel = byte(0x42)
	maxMsgSize  = int(1e6) // FIXME Set something accurate
)

// Reactor handles data exchange for light clients across P2P.
type Reactor struct {
	p2p.BaseReactor

	blockStore *store.BlockStore
	stateDB    db.DB
	logger     log.Logger

	mtx      sync.Mutex
	peers    map[p2p.ID]p2p.Peer
	chVals   map[int64][]chan<- *types.ValidatorSet
	chHeader map[int64][]chan<- *types.SignedHeader
}

// NewReactor creates a new light client reactor
func NewReactor(bs *store.BlockStore, stateDB db.DB) *Reactor {
	return &Reactor{
		blockStore: bs,
		stateDB:    stateDB,
		peers:      make(map[p2p.ID]p2p.Peer, 16),
		chVals:     make(map[int64][]chan<- *types.ValidatorSet),
		chHeader:   make(map[int64][]chan<- *types.SignedHeader),
	}
}

// SignedHeader synchronously attempts to fetch a header, timing out after some time.
func (r *Reactor) SignedHeader(height int64) (*types.SignedHeader, error) {
	r.mtx.Lock()
	if len(r.peers) == 0 {
		r.mtx.Unlock()
		return nil, errors.New("no available peers")
	}
	var peer p2p.Peer
	for _, p := range r.peers {
		peer = p
		break
	}
	ch := make(chan *types.SignedHeader)
	if _, ok := r.chHeader[height]; ok {
		r.chHeader[height] = append(r.chHeader[height], ch)
	} else {
		r.chHeader[height] = []chan<- *types.SignedHeader{ch}
	}
	r.mtx.Unlock()

	peer.Send(LiteChannel, cdc.MustMarshalBinaryBare(signedHeaderRequestMessage{
		height: height,
	}))

	return <-ch, nil // FIXME Need a timeout here
}

// AddPeer implements Reactor.
func (r *Reactor) AddPeer(peer p2p.Peer) {
	r.mtx.Lock()
	defer r.mtx.Unlock()
	r.logger.Info("Hi!", "peer", peer.ID())
	r.peers[peer.ID()] = peer
}

// RemovePeer implements Reactor.
func (r *Reactor) RemovePeer(peer p2p.Peer, reason interface{}) {
	r.mtx.Lock()
	defer r.mtx.Unlock()
	r.logger.Info("Bye!", "peer", peer.ID())
	delete(r.peers, peer.ID())
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
	}

	err = msg.ValidateBasic()
	if err != nil {
		r.logger.Error("peer sent us invalid msg", "peer", src, "msg", msg, "err", err)
		return
	}

	// FIXME Debug
	r.logger.Info("Receive", "src", src.ID(), "chID", chID, "msg", msg)

	switch msg := msg.(type) {
	case *signedHeaderRequestMessage:
		var signedHeader *types.SignedHeader
		blockMeta := r.blockStore.LoadBlockMeta(msg.height)
		commit := r.blockStore.LoadSeenCommit(msg.height)
		if blockMeta != nil && commit != nil {
			signedHeader = &types.SignedHeader{
				Header: &blockMeta.Header,
				Commit: commit,
			}
		}
		src.Send(LiteChannel, cdc.MustMarshalBinaryBare(&signedHeaderResponseMessage{
			signedHeader: signedHeader,
		}))
	case *signedHeaderResponseMessage:
		r.mtx.Lock()
		defer r.mtx.Unlock()
		for _, ch := range r.chHeader[msg.height] {
			ch <- msg.signedHeader
		}
		delete(r.chHeader, msg.height)
	case *validatorSetRequestMessage:
		vals, err := state.LoadValidators(r.stateDB, msg.height)
		if _, ok := err.(state.ErrNoValSetForHeight); ok {
			vals = nil
		} else if err != nil {
			r.logger.Error("Failed to fetch validator set", "height", msg.height, "err", err)
			return
		}
		src.Send(LiteChannel, cdc.MustMarshalBinaryBare(&validatorSetResponseMessage{
			validatorSet: vals,
		}))
	}
}

// SetLogger sets the logger of the reactor.
func (r *Reactor) SetLogger(logger log.Logger) {
	r.logger = logger
	r.BaseReactor.SetLogger(logger)
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

// Message is a generic message for this reactor.
type Message interface {
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
	height int64
}

func (m *signedHeaderRequestMessage) ValidateBasic() error {
	if m.height < 0 {
		return errors.New("height cannot be negative")
	}
	return nil
}

type signedHeaderResponseMessage struct {
	height       int64
	signedHeader *types.SignedHeader
}

func (m *signedHeaderResponseMessage) ValidateBasic() error {
	return nil
}

type validatorSetRequestMessage struct {
	height int64
}

func (m *validatorSetRequestMessage) ValidateBasic() error {
	if m.height < 0 {
		return errors.New("height cannot be negative")
	}
	return nil
}

type validatorSetResponseMessage struct {
	height       int64
	validatorSet *types.ValidatorSet
}

func (m *validatorSetResponseMessage) ValidateBasic() error {
	return nil
}
