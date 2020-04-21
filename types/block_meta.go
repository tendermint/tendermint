package types

import (
	"bytes"

	"github.com/pkg/errors"

	tmproto "github.com/tendermint/tendermint/proto/types"
)

// BlockMeta contains meta information.
type BlockMeta struct {
	BlockID   BlockID `json:"block_id"`
	BlockSize int     `json:"block_size"`
	Header    Header  `json:"header"`
	NumTxs    int     `json:"num_txs"`
}

// NewBlockMeta returns a new BlockMeta.
func NewBlockMeta(block *Block, blockParts *PartSet) *BlockMeta {

	return &BlockMeta{
		BlockID:   BlockID{Hash: block.Hash(), PartsHeader: blockParts.Header()},
		BlockSize: block.Size(),
		Header:    block.Header,
		NumTxs:    len(block.Data.Txs),
	}
}

func (bm *BlockMeta) ToProto() *tmproto.BlockMeta {
	if bm == nil {
		bm = &BlockMeta{}
	}
	pb := &tmproto.BlockMeta{
		BlockID:   *bm.BlockID.ToProto(),
		BlockSize: int64(bm.BlockSize),
		Header:    *bm.Header.ToProto(),
		NumTxs:    int64(bm.NumTxs),
	}
	return pb
}

func (bm *BlockMeta) FromProto(pb *tmproto.BlockMeta) error {
	if pb == nil {
		return errors.New("blockmeta is empty")
	}

	var (
		bi BlockID
		h  Header
	)

	if err := bi.FromProto(&pb.BlockID); err != nil {
		return err
	}
	if err := h.FromProto(&pb.Header); err != nil {
		return err
	}

	bm.BlockID = bi
	bm.BlockSize = int(pb.BlockSize)
	bm.Header = h
	bm.NumTxs = int(pb.NumTxs)

	return bm.ValidateBasic()
}

// ValidateBasic performs basic validation.
func (bm *BlockMeta) ValidateBasic() error {
	if err := bm.BlockID.ValidateBasic(); err != nil {
		return err
	}
	if !bytes.Equal(bm.BlockID.Hash, bm.Header.Hash()) {
		return errors.Errorf("expected BlockID#Hash and Header#Hash to be the same, got %X != %X",
			bm.BlockID.Hash, bm.Header.Hash())
	}
	return nil
}

//-----------------------------------------------------------
// These methods are for Protobuf Compatibility

// Size returns the size of the amino encoding, in bytes.
func (bm *BlockMeta) Size() int {
	bs, _ := bm.Marshal()
	return len(bs)
}

// Marshal returns the amino encoding.
func (bm *BlockMeta) Marshal() ([]byte, error) {
	return cdc.MarshalBinaryBare(bm)
}

// MarshalTo calls Marshal and copies to the given buffer.
func (bm *BlockMeta) MarshalTo(data []byte) (int, error) {
	bs, err := bm.Marshal()
	if err != nil {
		return -1, err
	}
	return copy(data, bs), nil
}

// Unmarshal deserializes from amino encoded form.
func (bm *BlockMeta) Unmarshal(bs []byte) error {
	return cdc.UnmarshalBinaryBare(bs, bm)
}
