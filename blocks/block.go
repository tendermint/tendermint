package blocks

import (
    "io"
)

// BlockHeader

type BlockHeader struct {
    Name            string
    Height          uint64
    Version         uint8
    Fees            uint64
    Time            uint64
    PrevBlockHash   []byte
    ValidationHash  []byte
    DataHash        []byte
}

func (self *BlockHeader) WriteTo(w io.Writer) (n int64, err error) {
    return 0, nil
}

// Block

type Block struct {
    Header          *BlockHeader
    Validation      *BlockValidation
    Data            *BlockData
    //Checkpoint      *BlockCheckpoint
}

func (self *Block) Validate() bool {
    return false
}
