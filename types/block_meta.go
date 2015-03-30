package types

type BlockMeta struct {
	Hash   []byte        // The block hash
	Header *Header       // The block's Header
	Parts  PartSetHeader // The PartSetHeader, for transfer
}

func NewBlockMeta(block *Block, blockParts *PartSet) *BlockMeta {
	return &BlockMeta{
		Hash:   block.Hash(),
		Header: block.Header,
		Parts:  blockParts.Header(),
	}
}
