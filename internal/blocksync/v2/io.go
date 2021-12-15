package v2

import (
	"errors"

	"github.com/gogo/protobuf/proto"
	"github.com/tendermint/tendermint/internal/p2p"
	"github.com/tendermint/tendermint/internal/state"
	bcproto "github.com/tendermint/tendermint/proto/tendermint/blocksync"
	"github.com/tendermint/tendermint/types"
)

var (
	errPeerQueueFull = errors.New("peer queue full")
)

type iIO interface {
	sendBlockRequest(peer p2p.Peer, height int64) error
	sendBlockToPeer(block *types.Block, peer p2p.Peer) error
	sendBlockNotFound(height int64, peer p2p.Peer) error
	sendStatusResponse(base, height int64, peer p2p.Peer) error

	sendStatusRequest(peer p2p.Peer) error
	broadcastStatusRequest() error

	trySwitchToConsensus(state state.State, skipWAL bool) bool
}

type switchIO struct {
	sw *p2p.Switch
}

func newSwitchIo(sw *p2p.Switch) *switchIO {
	return &switchIO{
		sw: sw,
	}
}

const (
	// BlockchainChannel is a channel for blocks and status updates (`BlockStore` height)
	BlockchainChannel = byte(0x40)
)

type consensusReactor interface {
	// for when we switch from blockchain reactor and block sync to
	// the consensus machine
	SwitchToConsensus(state state.State, skipWAL bool)
}

func (sio *switchIO) sendBlockRequest(peer p2p.Peer, height int64) error {
	msgProto := &bcproto.Message{
		Sum: &bcproto.Message_BlockRequest{
			BlockRequest: &bcproto.BlockRequest{
				Height: height,
			},
		},
	}

	msgBytes, err := proto.Marshal(msgProto)
	if err != nil {
		return err
	}

	queued := peer.TrySend(BlockchainChannel, msgBytes)
	if !queued {
		return errPeerQueueFull
	}
	return nil
}

func (sio *switchIO) sendStatusResponse(base int64, height int64, peer p2p.Peer) error {
	msgProto := &bcproto.Message{
		Sum: &bcproto.Message_StatusResponse{
			StatusResponse: &bcproto.StatusResponse{
				Height: height,
				Base:   base,
			},
		},
	}

	msgBytes, err := proto.Marshal(msgProto)
	if err != nil {
		return err
	}

	if queued := peer.TrySend(BlockchainChannel, msgBytes); !queued {
		return errPeerQueueFull
	}

	return nil
}

func (sio *switchIO) sendBlockToPeer(block *types.Block, peer p2p.Peer) error {
	if block == nil {
		panic("trying to send nil block")
	}

	bpb, err := block.ToProto()
	if err != nil {
		return err
	}

	msgProto := &bcproto.Message{
		Sum: &bcproto.Message_BlockResponse{
			BlockResponse: &bcproto.BlockResponse{
				Block: bpb,
			},
		},
	}

	msgBytes, err := proto.Marshal(msgProto)
	if err != nil {
		return err
	}

	if queued := peer.TrySend(BlockchainChannel, msgBytes); !queued {
		return errPeerQueueFull
	}

	return nil
}

func (sio *switchIO) sendBlockNotFound(height int64, peer p2p.Peer) error {
	msgProto := &bcproto.Message{
		Sum: &bcproto.Message_NoBlockResponse{
			NoBlockResponse: &bcproto.NoBlockResponse{
				Height: height,
			},
		},
	}

	msgBytes, err := proto.Marshal(msgProto)
	if err != nil {
		return err
	}

	if queued := peer.TrySend(BlockchainChannel, msgBytes); !queued {
		return errPeerQueueFull
	}

	return nil
}

func (sio *switchIO) trySwitchToConsensus(state state.State, skipWAL bool) bool {
	conR, ok := sio.sw.Reactor("CONSENSUS").(consensusReactor)
	if ok {
		conR.SwitchToConsensus(state, skipWAL)
	}
	return ok
}

func (sio *switchIO) sendStatusRequest(peer p2p.Peer) error {
	msgProto := &bcproto.Message{
		Sum: &bcproto.Message_StatusRequest{
			StatusRequest: &bcproto.StatusRequest{},
		},
	}

	msgBytes, err := proto.Marshal(msgProto)
	if err != nil {
		return err
	}

	if queued := peer.TrySend(BlockchainChannel, msgBytes); !queued {
		return errPeerQueueFull
	}

	return nil
}

func (sio *switchIO) broadcastStatusRequest() error {
	msgProto := &bcproto.Message{
		Sum: &bcproto.Message_StatusRequest{
			StatusRequest: &bcproto.StatusRequest{},
		},
	}

	msgBytes, err := proto.Marshal(msgProto)
	if err != nil {
		return err
	}

	// XXX: maybe we should use an io specific peer list here
	sio.sw.Broadcast(BlockchainChannel, msgBytes)

	return nil
}
