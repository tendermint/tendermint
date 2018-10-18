package types

import amino "github.com/tendermint/go-amino"


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

//marshal,unmarshal and size methods
var bamino = amino.NewCodec()

func (bm *BlockMeta) Encode() ([]byte, error) {
	return bm.MarshalBinary(&bm)
}

func (bm *BlockMeta) Decode(bs []byte) error {
	err := bm.UnmarshalBinary(bs, &bm)
	if err != nil {
		return err
	}
	return nil

}

func (bm *BlockMeta) Unmarshal(bs []byte) error {
	return b.Decode(bs)
}

func (bm *BlockMeta) Marshal() ([]byte, error) {
	return b.Encode()
}

func (bm *BlockMeta) MarshalTo(data []byte) (int, error) {
	bs, err := b.Encode()
	if err != nil {
		return -1, err
	}
	return copy(data, bs), nil
}

func (bm *BlockMeta) Size() int {
	bs, _ := b.Encode()
	return len(bs)
}


func (bm BlockMeta) MarshalJSON() ([]byte, error) {
	return bm.MarshalJSON(b)
}

func (bm *BlockMeta) UnmarshalJSON(data []byte) (err error) {
	return bm.UnmarshalJSON(data, &b)
}