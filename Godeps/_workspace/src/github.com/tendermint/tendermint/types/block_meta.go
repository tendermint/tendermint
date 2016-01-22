package types

type BlockMeta struct {
	Hash        []byte        `json:"hash"`         // The block hash
	Header      *Header       `json:"header"`       // The block's Header
	PartsHeader PartSetHeader `json:"parts_header"` // The PartSetHeader, for transfer
}

func NewBlockMeta(block *Block, blockParts *PartSet) *BlockMeta {
	return &BlockMeta{
		Hash:        block.Hash(),
		Header:      block.Header,
		PartsHeader: blockParts.Header(),
	}
}
