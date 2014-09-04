package blocks

import (
	"bytes"
	. "github.com/tendermint/tendermint/binary"
	"math/rand"
	"testing"
	"time"
)

// Distributed pseudo-exponentially to test for various cases
func randUInt64() uint64 {
	bits := rand.Uint32() % 64
	if bits == 0 {
		return 0
	}
	n := uint64(1 << (bits - 1))
	n += uint64(rand.Int63()) & ((1 << (bits - 1)) - 1)
	return n
}

func randUInt32() uint32 {
	bits := rand.Uint32() % 32
	if bits == 0 {
		return 0
	}
	n := uint32(1 << (bits - 1))
	n += uint32(rand.Int31()) & ((1 << (bits - 1)) - 1)
	return n
}

func randTime() time.Time {
	return time.Unix(int64(randUInt64()), 0)
}

func randAccount() Account {
	return Account{
		Id:     randUInt64(),
		PubKey: randBytes(32),
	}
}

func randBytes(n int) []byte {
	bs := make([]byte, n)
	for i := 0; i < n; i++ {
		bs[i] = byte(rand.Intn(256))
	}
	return bs
}

func randSig() Signature {
	return Signature{randUInt64(), randBytes(32)}
}

func TestBlock(t *testing.T) {

	// Txs

	sendTx := &SendTx{
		Signature: randSig(),
		Fee:       randUInt64(),
		To:        randUInt64(),
		Amount:    randUInt64(),
	}

	nameTx := &NameTx{
		Signature: randSig(),
		Fee:       randUInt64(),
		Name:      string(randBytes(12)),
		PubKey:    randBytes(32),
	}

	// Adjs

	bond := &Bond{
		Signature: randSig(),
		Fee:       randUInt64(),
		UnbondTo:  randUInt64(),
		Amount:    randUInt64(),
	}

	unbond := &Unbond{
		Signature: randSig(),
		Fee:       randUInt64(),
		Amount:    randUInt64(),
	}

	timeout := &Timeout{
		AccountId: randUInt64(),
		Penalty:   randUInt64(),
	}

	dupeout := &Dupeout{
		VoteA: BlockVote{
			Height:    randUInt64(),
			BlockHash: randBytes(32),
			Signature: randSig(),
		},
		VoteB: BlockVote{
			Height:    randUInt64(),
			BlockHash: randBytes(32),
			Signature: randSig(),
		},
	}

	// Block

	block := &Block{
		Header: Header{
			Name:           "Tendermint",
			Height:         randUInt32(),
			Fees:           randUInt64(),
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
	var n int64
	var err error
	block2 := ReadBlock(bytes.NewReader(blockBytes), &n, &err)
	blockBytes2 := BinaryBytes(block2)

	if !bytes.Equal(blockBytes, blockBytes2) {
		t.Fatal("Write->Read of block failed.")
	}
}
