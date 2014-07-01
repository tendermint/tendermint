package blocks

import (
	"bytes"
	. "github.com/tendermint/tendermint/binary"
	"math/rand"
	"testing"
)

// Distributed pseudo-exponentially to test for various cases
func randVar() UInt64 {
	bits := rand.Uint32() % 64
	if bits == 0 {
		return 0
	}
	n := uint64(1 << (bits - 1))
	n += uint64(rand.Int63()) & ((1 << (bits - 1)) - 1)
	return UInt64(n)
}

func randBytes(n int) ByteSlice {
	bs := make([]byte, n)
	for i := 0; i < n; i++ {
		bs[i] = byte(rand.Intn(256))
	}
	return bs
}

func randSig() Signature {
	return Signature{AccountNumber(randVar()), randBytes(32)}
}

func TestBlock(t *testing.T) {

	// Txs

	sendTx := &SendTx{
		Signature: randSig(),
		Fee:       randVar(),
		To:        AccountNumber(randVar()),
		Amount:    randVar(),
	}

	nameTx := &NameTx{
		Signature: randSig(),
		Fee:       randVar(),
		Name:      String(randBytes(12)),
		PubKey:    randBytes(32),
	}

	// Adjs

	bond := &Bond{
		Signature: randSig(),
		Fee:       randVar(),
		UnbondTo:  AccountNumber(randVar()),
		Amount:    randVar(),
	}

	unbond := &Unbond{
		Signature: randSig(),
		Fee:       randVar(),
		Amount:    randVar(),
	}

	timeout := &Timeout{
		Account: AccountNumber(randVar()),
		Penalty: randVar(),
	}

	dupeout := &Dupeout{
		VoteA: Vote{
			Height:    randVar(),
			BlockHash: randBytes(32),
			Signature: randSig(),
		},
		VoteB: Vote{
			Height:    randVar(),
			BlockHash: randBytes(32),
			Signature: randSig(),
		},
	}

	// Block

	block := &Block{
		Header{
			Name:           "Tendermint",
			Height:         randVar(),
			Fees:           randVar(),
			Time:           randVar(),
			PrevHash:       randBytes(32),
			ValidationHash: randBytes(32),
			DataHash:       randBytes(32),
		},
		Validation{
			Signatures:  []Signature{randSig(), randSig()},
			Adjustments: []Adjustment{bond, unbond, timeout, dupeout},
		},
		Data{
			Txs: []Tx{sendTx, nameTx},
		},
	}

	// Write the block, read it in again, write it again.
	// Then, compare.

	blockBytes := BinaryBytes(block)
	block2 := ReadBlock(bytes.NewReader(blockBytes))
	blockBytes2 := BinaryBytes(block2)

	if !BinaryEqual(blockBytes, blockBytes2) {
		t.Fatal("Write->Read of block failed.")
	}
}
