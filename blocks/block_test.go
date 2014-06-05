package blocks

import (
    . "github.com/tendermint/tendermint/binary"
    "testing"
    "bytes"
    "fmt"
    "math/rand"
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
    var block = NewBlock()

    sendTx := &SendTx{
        Signature:  Signature{AccountNumber(randVar()), randBytes(32)},
        Fee:        randVar(),
        To:         AccountNumber(randVar()),
        Amount:     randVar(),
    }

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

    nameTx := &NameTx{
        Signature:  Signature{AccountNumber(randVar()), randBytes(32)},
        Fee:        randVar(),
        Name:       String(randBytes(12)),
        PubKey:     randBytes(32),
    }

    block.AddTx(sendTx)
    block.AddTx(bondTx)
    block.AddTx(unbondTx)
    block.AddTx(nameTx)

    blockBytes := block.Bytes()

    fmt.Println(buf.Bytes(), len(buf.Bytes()))

}
