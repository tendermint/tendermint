package blocks

import (
    . "github.com/tendermint/tendermint/binary"
    "testing"
    "math/rand"
    "bytes"
)

// Distributed pseudo-exponentially to test for various cases
func randVar() UInt64 {
    bits := rand.Uint32() % 64
    if bits == 0 { return 0 }
    n := uint64(1 << (bits-1))
    n += uint64(rand.Int63()) & ((1 << (bits-1)) - 1)
    return UInt64(n)
}

func randBytes(n int) ByteSlice {
    bs := make([]byte, n)
    for i:=0; i<n; i++ {
        bs[i] = byte(rand.Intn(256))
    }
    return bs
}

func TestBlock(t *testing.T) {

    sendTx := &SendTx{
        Signature:  Signature{AccountNumber(randVar()), randBytes(32)},
        Fee:        randVar(),
        To:         AccountNumber(randVar()),
        Amount:     randVar(),
    }

    nameTx := &NameTx{
        Signature:  Signature{AccountNumber(randVar()), randBytes(32)},
        Fee:        randVar(),
        Name:       String(randBytes(12)),
        PubKey:     randBytes(32),
    }

    txs := []Tx{}
    txs = append(txs, sendTx)
    txs = append(txs, nameTx)

    block := &Block{
        Header{
            Name:       "Tendermint",
            Height:     randVar(),
            Fees:       randVar(),
            Time:       randVar(),
            PrevHash:   randBytes(32),
            ValidationHash: randBytes(32),
            DataHash:   randBytes(32),
        },
        Validation{
            Signatures:nil,
            Adjustments:nil,
        },
        Data{txs},
    }

    blockBytes := BinaryBytes(block)
    block2 := ReadBlock(bytes.NewReader(blockBytes))
    blockBytes2 := BinaryBytes(block2)

    if !BinaryEqual(blockBytes, blockBytes2) {
        t.Fatal("Write->Read of block failed.")
    }
}

    /*
    bondTx := &BondTx{
        Signature:  Signature{AccountNumber(randVar()), randBytes(32)},
        Fee:        randVar(),
        UnbondTo:   AccountNumber(randVar()),
        Amount:     randVar(),
    }

    unbondTx := &UnbondTx{
        Signature:  Signature{AccountNumber(randVar()), randBytes(32)},
        Fee:        randVar(),
        Amount:     randVar(),
    }
    */

