package v2

import (
	"fmt"

	bc "github.com/tendermint/tendermint/blockchain"
	"github.com/tendermint/tendermint/p2p"
	bcproto "github.com/tendermint/tendermint/proto/tendermint/blockchain"
	"github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/types"
)

type iIO interface {
	sendBlockRequest(peerID p2p.ID, height int64) error
	sendBlockToPeer(block *types.Block, peerID p2p.ID) error
	sendBlockNotFound(height int64, peerID p2p.ID) error
	sendStatusResponse(base, height int64, peerID p2p.ID) error

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
	// for when we switch from blockchain reactor and fast sync to
	// the consensus machine
	SwitchToConsensus(state state.State, skipWAL bool)
}

func (sio *switchIO) sendBlockRequest(peerID p2p.ID, height int64) error {
	peer := sio.sw.Peers().Get(peerID)
	if peer == nil {
		return fmt.Errorf("peer not found")
	}
	msgBytes, err := bc.EncodeMsg(&bcproto.BlockRequest{Height: height})
	if err != nil {
		return err
	}

	queued := peer.TrySend(BlockchainChannel, msgBytes)
	if !queued {
		return fmt.Errorf("send queue full")
	}
	return nil
}

func (sio *switchIO) sendStatusResponse(base int64, height int64, peerID p2p.ID) error {
	peer := sio.sw.Peers().Get(peerID)
	if peer == nil {
		return fmt.Errorf("peer not found")
	}

	msgBytes, err := bc.EncodeMsg(&bcproto.StatusResponse{Height: height, Base: base})
	if err != nil {
		return err
	}

	if queued := peer.TrySend(BlockchainChannel, msgBytes); !queued {
		return fmt.Errorf("peer queue full")
	}

	return nil
}

func (sio *switchIO) sendBlockToPeer(block *types.Block, peerID p2p.ID) error {
	peer := sio.sw.Peers().Get(peerID)
	if peer == nil {
		return fmt.Errorf("peer not found")
	}
	if block == nil {
		panic("trying to send nil block")
	}

	bpb, err := block.ToProto()
	if err != nil {
		return err
	}

	msgBytes, err := bc.EncodeMsg(&bcproto.BlockResponse{Block: bpb})
	if err != nil {
		return err
	}
	if queued := peer.TrySend(BlockchainChannel, msgBytes); !queued {
		return fmt.Errorf("peer queue full")
	}

	return nil
}

func (sio *switchIO) sendBlockNotFound(height int64, peerID p2p.ID) error {
	peer := sio.sw.Peers().Get(peerID)
	if peer == nil {
		return fmt.Errorf("peer not found")
	}
	msgBytes, err := bc.EncodeMsg(&bcproto.NoBlockResponse{Height: height})
	if err != nil {
		return err
	}

	if queued := peer.TrySend(BlockchainChannel, msgBytes); !queued {
		return fmt.Errorf("peer queue full")
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

func (sio *switchIO) broadcastStatusRequest() error {
	msgBytes, err := bc.EncodeMsg(&bcproto.StatusRequest{})
	if err != nil {
		return err
	}

	// XXX: maybe we should use an io specific peer list here
	sio.sw.Broadcast(BlockchainChannel, msgBytes)

	return nil
}
