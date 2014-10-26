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

func randBlock() *Block {
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
			Type:      VoteTypePrevote,
			BlockHash: RandBytes(32),
			Signature: randSig(),
		},
		VoteB: Vote{
			Height:    RandUInt32Exp(),
			Round:     RandUInt16Exp(),
			Type:      VoteTypePrevote,
			BlockHash: RandBytes(32),
			Signature: randSig(),
		},
	}

	// Block
	block := &Block{
		Header: Header{
			Network:       "Tendermint",
			Height:        RandUInt32Exp(),
			Fees:          RandUInt64Exp(),
			Time:          RandTime(),
			LastBlockHash: RandBytes(32),
			StateHash:     RandBytes(32),
		},
		Validation: Validation{
			Signatures: []Signature{randSig(), randSig()},
		},
		Data: Data{
			Txs: []Tx{sendTx, nameTx, bondTx, unbondTx, dupeoutTx},
		},
	}
	return block
}

func TestBlock(t *testing.T) {

	block := randBlock()
	// Mutate the block and ensure that the hash changed.
	lastHash := block.Hash()
	expectChange := func(mutateFn func(b *Block), message string) {
		// mutate block
		mutateFn(block)
		// nuke hashes
		block.hash = nil
		block.Header.hash = nil
		block.Validation.hash = nil
		block.Data.hash = nil
		// compare
		if bytes.Equal(lastHash, block.Hash()) {
			t.Error(message)
		} else {
			lastHash = block.Hash()
		}
	}
	expectChange(func(b *Block) { b.Header.Network = "blah" }, "Expected hash to depend on Network")
	expectChange(func(b *Block) { b.Header.Height += 1 }, "Expected hash to depend on Height")
	expectChange(func(b *Block) { b.Header.Fees += 1 }, "Expected hash to depend on Fees")
	expectChange(func(b *Block) { b.Header.Time = RandTime() }, "Expected hash to depend on Time")
	expectChange(func(b *Block) { b.Header.LastBlockHash = RandBytes(32) }, "Expected hash to depend on LastBlockHash")
	expectChange(func(b *Block) { b.Header.StateHash = RandBytes(32) }, "Expected hash to depend on StateHash")
	expectChange(func(b *Block) { b.Validation.Signatures[0].SignerId += 1 }, "Expected hash to depend on Validation Signature")
	expectChange(func(b *Block) { b.Validation.Signatures[0].Bytes = RandBytes(32) }, "Expected hash to depend on Validation Signature")
	expectChange(func(b *Block) { b.Data.Txs[0].(*SendTx).Signature.SignerId += 1 }, "Expected hash to depend on tx Signature")
	expectChange(func(b *Block) { b.Data.Txs[0].(*SendTx).Amount += 1 }, "Expected hash to depend on send tx Amount")

	// Write the block, read it in again, check hash.
	block1 := randBlock()
	block1Bytes := BinaryBytes(block1)
	var n int64
	var err error
	block2 := ReadBlock(bytes.NewReader(block1Bytes), &n, &err)
	if err != nil {
		t.Errorf("Reading block failed: %v", err)
	}
	if !bytes.Equal(block1.Hash(), block2.Hash()) {
		t.Errorf("Expected write/read to preserve original hash")
		t.Logf("\nBlock1:\n%v", block1)
		t.Logf("\nBlock2:\n%v", block2)
	}
}
