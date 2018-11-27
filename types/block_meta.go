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
//Unmarshal interface takes the serialized block-meta object and transforms into an executable form.
func (bm *BlockMeta) Unmarshal(bs []byte) error {
	return cdc.UnmarshalBinaryBare(bs, bm)
}

//Marshal interface  serialize the block-meta object and allocates the appropriate buffer.
func (bm *BlockMeta) Marshal() ([]byte, error) {
	return cdc.MarshalBinaryBare(bm)
}

//MarshalTo method allows BlockMeta object to use reusable buffer.
func (bm *BlockMeta) MarshalTo(data []byte) (int, error) {
	bs, err := bm.Marshal()
	if err != nil {
		return -1, err
	}
	return copy(data, bs), nil
}

// Size returns size of the block in bytes.
func (bm *BlockMeta) Size() int {
	bs, _ := bm.Marshal()
	return len(bs)
}
