package blocks

import (
	"bytes"
	. "github.com/tendermint/tendermint/binary"
	"math/rand"
	"testing"
	"time"
)

// Distributed pseudo-exponentially to test for various cases
func randuint64() uint64 {
	bits := rand.Uint32() % 64
	if bits == 0 {
		return 0
	}
	n := uint64(1 << (bits - 1))
	n += uint64(rand.Int63()) & ((1 << (bits - 1)) - 1)
	return n
}

func randuint32() uint32 {
	bits := rand.Uint32() % 32
	if bits == 0 {
		return 0
	}
	n := uint32(1 << (bits - 1))
	n += uint32(rand.Int31()) & ((1 << (bits - 1)) - 1)
	return n
}

func randTime() Time {
	return Time{time.Unix(int64(randuint64()), 0)}
}

func randAccount() Account {
	return Account{
		Id:     randuint64(),
		PubKey: randBytes(32),
	}
}

func randBytes(n int) ByteSlice {
	bs := make([]byte, n)
	for i := 0; i < n; i++ {
		bs[i] = byte(rand.Intn(256))
	}
	return bs
}

func randSig() Signature {
	return Signature{randuint64(), randBytes(32)}
}

func TestBlock(t *testing.T) {

	// Txs

	sendTx := &SendTx{
		Signature: randSig(),
		Fee:       randuint64(),
		To:        randuint64(),
		Amount:    randuint64(),
	}

	nameTx := &NameTx{
		Signature: randSig(),
		Fee:       randuint64(),
		Name:      String(randBytes(12)),
		PubKey:    randBytes(32),
	}

	// Adjs

	bond := &Bond{
		Signature: randSig(),
		Fee:       randuint64(),
		UnbondTo:  randuint64(),
		Amount:    randuint64(),
	}

	unbond := &Unbond{
		Signature: randSig(),
		Fee:       randuint64(),
		Amount:    randuint64(),
	}

	timeout := &Timeout{
		AccountId: randuint64(),
		Penalty:   randuint64(),
	}

	dupeout := &Dupeout{
		VoteA: BlockVote{
			Height:    randuint64(),
			BlockHash: randBytes(32),
			Signature: randSig(),
		},
		VoteB: BlockVote{
			Height:    randuint64(),
			BlockHash: randBytes(32),
			Signature: randSig(),
		},
	}

	// Block

	block := &Block{
		Header: Header{
			Name:           "Tendermint",
			Height:         randuint32(),
			Fees:           randuint64(),
			Time:           randTime(),
			PrevHash:       randBytes(32),
			ValidationHash: randBytes(32),
			TxsHash:        randBytes(32),
		},
		Validation: Validation{
			Signatures:  []Signature{randSig(), randSig()},
			Adjustments: []Adjustment{bond, unbond, timeout, dupeout},
		},
		Txs: Txs{
			Txs:  []Tx{sendTx, nameTx},
			hash: nil,
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
