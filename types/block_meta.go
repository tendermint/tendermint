package types

// BlockMeta contains meta information about a block - namely, it's ID and Header.
type BlockMeta struct {
	BlockID BlockID `json:"block_id"` // the block hash and partsethash
	Header  Header  `json:"header"`   // The block's Header
}

// NewBlockMeta returns a new BlockMeta from the block and its blockParts.
func NewBlockMeta(block *Block, blockParts *PartSet) *BlockMeta {
	return &BlockMeta{
		BlockID: BlockID{block.Hash(), blockParts.Header()},
		Header:  block.Header,
	}
}

// Protobuf Compatiablity
func (bm *BlockMeta) Unmarshal(bs []byte) error {
	return cdc.UnmarshalBinaryBare(bs, bm)
}

func (bm *BlockMeta) Marshal() ([]byte, error) {
	return cdc.MarshalBinaryBare(bm)
}

func (bm *BlockMeta) MarshalTo(data []byte) (int, error) {
	bs, err := bm.Marshal()
	if err != nil {
		return -1, err
	}
	return copy(data, bs), nil
}

func (bm *BlockMeta) Size() int {
	bs, _ := bm.Marshal()
	return len(bs)
}
