package types

type BlockMeta struct {
	Hash   []byte        `json:"hash"`   // The block hash
	Header *Header       `json:"header"` // The block's Header
	Parts  PartSetHeader `json:"parts"`  // The PartSetHeader, for transfer
}

func NewBlockMeta(block *Block, blockParts *PartSet) *BlockMeta {
	return &BlockMeta{
		Hash:   block.Hash(),
		Header: block.Header,
		Parts:  blockParts.Header(),
	}
}
