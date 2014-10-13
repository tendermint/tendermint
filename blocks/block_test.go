package blocks

import (
	"bytes"
	. "github.com/tendermint/tendermint/binary"
	. "github.com/tendermint/tendermint/common"
	"testing"
)

func randSig() Signature {
	return Signature{RandUInt64Exp(), RandBytes(32)}
}

func randBaseTx() BaseTx {
	return BaseTx{0, RandUInt64Exp(), randSig()}
}

func TestBlock(t *testing.T) {

	// Account Txs

	sendTx := &SendTx{
		BaseTx: randBaseTx(),
		To:     RandUInt64Exp(),
		Amount: RandUInt64Exp(),
	}

	nameTx := &NameTx{
		BaseTx: randBaseTx(),
		Name:   string(RandBytes(12)),
		PubKey: RandBytes(32),
	}

	// Validation Txs

	bondTx := &BondTx{
		BaseTx: randBaseTx(),
		//UnbondTo: RandUInt64Exp(),
	}

	unbondTx := &UnbondTx{
		BaseTx: randBaseTx(),
	}

	dupeoutTx := &DupeoutTx{
		VoteA: Vote{
			Height:    RandUInt32Exp(),
			Round:     RandUInt16Exp(),
			Type:      VoteTypeBare,
			BlockHash: RandBytes(32),
			Signature: randSig(),
		},
		VoteB: Vote{
			Height:    RandUInt32Exp(),
			Round:     RandUInt16Exp(),
			Type:      VoteTypeBare,
			BlockHash: RandBytes(32),
			Signature: randSig(),
		},
	}

	// Block

	block := &Block{
		Header: Header{
			Network:             "Tendermint",
			Height:              RandUInt32Exp(),
			Fees:                RandUInt64Exp(),
			Time:                RandTime(),
			LastBlockHash:       RandBytes(32),
			ValidationStateHash: RandBytes(32),
			AccountStateHash:    RandBytes(32),
		},
		Validation: Validation{
			Signatures: []Signature{randSig(), randSig()},
		},
		Data: Data{
			Txs: []Tx{sendTx, nameTx, bondTx, unbondTx, dupeoutTx},
		},
	}

	// Write the block, read it in again, write it again.
	// Then, compare.
	// TODO We should compute the hash instead, so Block -> Bytes -> Block and compare hashes.

	blockBytes := BinaryBytes(block)
	var n int64
	var err error
	block2 := ReadBlock(bytes.NewReader(blockBytes), &n, &err)
	if err != nil {
		t.Errorf("Reading block failed: %v", err)
	}

	blockBytes2 := BinaryBytes(block2)

	if !bytes.Equal(blockBytes, blockBytes2) {
		t.Fatal("Write->Read of block failed.")
	}
}
