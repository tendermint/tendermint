package types

import (
	abci "github.com/tendermint/abci/types"
)

//------------------------------------------------------
// blockchain services types
// NOTE: Interfaces used by RPC must be thread safe!
//------------------------------------------------------

//------------------------------------------------------
// mempool

// Updates to the mempool need to be synchronized with committing a block
// so apps can reset their transient state on Commit
type Mempool interface {
	Lock()
	Unlock()

	Size() int
	CheckTx(Tx, func(*abci.Response)) error
	Reap(int) Txs
	Update(height int, txs Txs)
	Flush()
}

type MockMempool struct {
}

func (m MockMempool) Lock()                                        {}
func (m MockMempool) Unlock()                                      {}
func (m MockMempool) Size() int                                    { return 0 }
func (m MockMempool) CheckTx(tx Tx, cb func(*abci.Response)) error { return nil }
func (m MockMempool) Reap(n int) Txs                               { return Txs{} }
func (m MockMempool) Update(height int, txs Txs)                   {}
func (m MockMempool) Flush()                                       {}

//------------------------------------------------------
// blockstore

type BlockStoreRPC interface {
	Height() int

	LoadBlockMeta(height int) *BlockMeta
	LoadBlock(height int) *Block
	LoadBlockPart(height int, index int) *Part

	LoadBlockCommit(height int) *Commit
	LoadSeenCommit(height int) *Commit
}

type BlockStore interface {
	BlockStoreRPC
	SaveBlock(block *Block, blockParts *PartSet, seenCommit *Commit)
}
