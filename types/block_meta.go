package types

import (
	"bytes"
	"errors"
	"fmt"

	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
)

// BlockMeta contains meta information.
type BlockMeta struct {
	BlockID          BlockID `json:"block_id"`
	StateID          StateID `json:"state_id"`
	BlockSize        int     `json:"block_size"`
	Header           Header  `json:"header"`
	HasCoreChainLock bool    `json:"has_core_chain_lock"`
	NumTxs           int     `json:"num_txs"`
}

// NewBlockMeta returns a new BlockMeta.
func NewBlockMeta(block *Block, blockParts *PartSet) *BlockMeta {
	return &BlockMeta{
		BlockID:          BlockID{block.Hash(), blockParts.Header()},
		StateID:          StateID{LastAppHash: block.Header.AppHash},
		BlockSize:        block.Size(),
		Header:           block.Header,
		HasCoreChainLock: block.CoreChainLock != nil,
		NumTxs:           len(block.Data.Txs),
	}
}

func (bm *BlockMeta) ToProto() *tmproto.BlockMeta {
	if bm == nil {
		return nil
	}

	pb := &tmproto.BlockMeta{
		BlockID:          bm.BlockID.ToProto(),
		StateID:          bm.StateID.ToProto(),
		BlockSize:        int64(bm.BlockSize),
		Header:           *bm.Header.ToProto(),
		HasCoreChainLock: bm.HasCoreChainLock,
		NumTxs:           int64(bm.NumTxs),
	}
	return pb
}

func BlockMetaFromProto(pb *tmproto.BlockMeta) (*BlockMeta, error) {
	if pb == nil {
		return nil, errors.New("blockmeta is empty")
	}

	bm := new(BlockMeta)

	bi, err := BlockIDFromProto(&pb.BlockID)
	if err != nil {
		return nil, err
	}

	si, err := StateIDFromProto(&pb.StateID)
	if err != nil {
		return nil, err
	}

	h, err := HeaderFromProto(&pb.Header)
	if err != nil {
		return nil, err
	}

	bm.BlockID = *bi
	bm.StateID = *si
	bm.BlockSize = int(pb.BlockSize)
	bm.Header = h
	bm.HasCoreChainLock = pb.HasCoreChainLock
	bm.NumTxs = int(pb.NumTxs)

	return bm, bm.ValidateBasic()
}

// ValidateBasic performs basic validation.
func (bm *BlockMeta) ValidateBasic() error {
	if err := bm.BlockID.ValidateBasic(); err != nil {
		return err
	}
	if err := bm.StateID.ValidateBasic(); err != nil {
		return err
	}
	if !bytes.Equal(bm.BlockID.Hash, bm.Header.Hash()) {
		return fmt.Errorf("expected BlockID#Hash and Header#Hash to be the same, got %X != %X",
			bm.BlockID.Hash, bm.Header.Hash())
	}
	return nil
}
