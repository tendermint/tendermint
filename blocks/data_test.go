package blocks

import (
    . "github.com/tendermint/tendermint/binary"
    "testing"
    "bytes"
    "fmt"
)

func TestBlockData(t *testing.T) {
    var bd = NewBlockData()
    var tx Tx

    tx = &SendTx{
        Signature:  Signature{AccountNumber(0), ByteSlice([]byte{7})},
        Fee:        1,
        To:         AccountNumber(2),
        Amount:     3,
    }

    bd.AddTx(tx)


    buf := bytes.NewBuffer(nil)
    bd.WriteTo(buf)

    fmt.Println(len(buf.Bytes()))

}
