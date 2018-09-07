package types

// BlockMeta contains meta information about a block - namely, it's ID and Header.
type BlockMeta struct {
	BlockID BlockID `json:"block_id"` // the block hash and partsethash
	Header  Header  `json:"header"`   // The block's Header
}

// NewBlockMeta returns a new BlockMeta from the block and its blockParts.
func NewBlockMeta(block *Block, blockParts *PartSet, proposeRound int) *BlockMeta {
	return &BlockMeta{
		BlockID: BlockID{block.Hash(), proposeRound, blockParts.Header()},
		Header:  block.Header,
	}
}
