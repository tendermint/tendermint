package v2

import (
	"errors"
	"fmt"

	"github.com/tendermint/tendermint/p2p"
	"github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/types"
)

type iIo interface {
	sendBlockRequest(peerID p2p.ID, height int64) error
	sendBlockToPeer(block *types.Block, peerID p2p.ID) error
	sendBlockNotFound(height int64, peerID p2p.ID) error
	sendStatusResponse(height int64, peerID p2p.ID) error
	switchToConsensus(state state.State, blocksSynced int)
	broadcastStatusRequest(height int64)
}

type switchIo struct {
	sw p2p.Switch
}

const (
	// BlockchainChannel is a channel for blocks and status updates (`BlockStore` height)
	BlockchainChannel = byte(0x40)
)

type bcStatusRequestMessage struct {
	Height int64
}

func (m *bcStatusRequestMessage) ValidateBasic() error {
	if m.Height < 0 {
		return errors.New("negative Height")
	}
	return nil
}

type consensusReactor interface {
	// for when we switch from blockchain reactor and fast sync to
	// the consensus machine
	SwitchToConsensus(state.State, int)
}

func (sio *switchIo) sendBlockRequest(peerID p2p.ID, height int64) error {
	peer := sio.sw.Peers().Get(peerID)
	if peer == nil {
		return fmt.Errorf("peer not found")
	}

	msgBytes := cdc.MustMarshalBinaryBare(&bcBlockRequestMessage{Height: height})
	queued := peer.TrySend(BlockchainChannel, msgBytes)
	if !queued {
		return fmt.Errorf("send queue full")
	}
	return nil
}

func (sio *switchIo) sendStatusResponse(height int64, peerID p2p.ID) error {
	peer := sio.sw.Peers().Get(peerID)
	if peer == nil {
		return fmt.Errorf("peer not found")
	}
	msgBytes := cdc.MustMarshalBinaryBare(&bcStatusResponse{height: height})
	peer.Send(BlockchainChannel, msgBytes)

	return nil
}

// XXX: should p[robably return an error
func (sio *switchIo) sendBlockToPeer(block *types.Block, peerID p2p.ID) error {
	peer := sio.sw.Peers().Get(peerID)
	if peer == nil {
		return fmt.Errorf("peer not found")
	}
	if block == nil {
		return fmt.Errorf("nil block")
	}
	msgBytes := cdc.MustMarshalBinaryBare(&bcBlockResponse{block: block})
	if queued := peer.TrySend(BlockchainChannel, msgBytes); !queued {
		return fmt.Errorf("peer queue full")
	}

	return nil
}

// XXX: We'll have to register these messages with the codec
type bcNoBlockResponseMessage struct {
	Height int64
}

func (sio *switchIo) sendBlockNotFound(height int64, peerID p2p.ID) error {
	peer := sio.sw.Peers().Get(peerID)
	if peer == nil {
		return fmt.Errorf("peer not found")
	}
	msgBytes := cdc.MustMarshalBinaryBare(&bcNoBlockResponseMessage{Height: height})
	if queued := peer.TrySend(BlockchainChannel, msgBytes); !queued {
		return fmt.Errorf("peer queue full")
	}

	return nil
}

func (sio *switchIo) switchToConsensus(state state.State, blocksSynced int) {
	conR, ok := sio.sw.Reactor("CONSENSUS").(consensusReactor)
	if ok {
		conR.SwitchToConsensus(state, blocksSynced)
	}
}

func (sio *switchIo) broadcastStatusRequest(height int64) {
	msgBytes := cdc.MustMarshalBinaryBare(&bcStatusRequestMessage{height})
	// XXX: maybe we should use an io specific peer list here
	sio.sw.Broadcast(BlockchainChannel, msgBytes)
}
