package types

import (
	abci "github.com/tendermint/abci/types"
)

// NOTE: all types in this file are considered UNSTABLE

//------------------------------------------------------
// blockchain services types
// NOTE: Interfaces used by RPC must be thread safe!
//------------------------------------------------------

//------------------------------------------------------
// mempool

// Mempool defines the mempool interface.
// Updates to the mempool need to be synchronized with committing a block
// so apps can reset their transient state on Commit
// UNSTABLE
type Mempool interface {
	Lock()
	Unlock()

	Size() int
	CheckTx(Tx, func(*abci.Response)) error
	Reap(int) Txs
	Update(height uint64, txs Txs) error
	Flush()

	TxsAvailable() <-chan uint64
	EnableTxsAvailable()
}

// MockMempool is an empty implementation of a Mempool, useful for testing.
// UNSTABLE
type MockMempool struct {
}

func (m MockMempool) Lock()                                        {}
func (m MockMempool) Unlock()                                      {}
func (m MockMempool) Size() int                                    { return 0 }
func (m MockMempool) CheckTx(tx Tx, cb func(*abci.Response)) error { return nil }
func (m MockMempool) Reap(n int) Txs                               { return Txs{} }
func (m MockMempool) Update(height uint64, txs Txs) error          { return nil }
func (m MockMempool) Flush()                                       {}
func (m MockMempool) TxsAvailable() <-chan uint64                  { return make(chan uint64) }
func (m MockMempool) EnableTxsAvailable()                          {}

//------------------------------------------------------
// blockstore

// BlockStoreRPC is the block store interface used by the RPC.
// UNSTABLE
type BlockStoreRPC interface {
	Height() uint64

	LoadBlockMeta(height uint64) *BlockMeta
	LoadBlock(height uint64) *Block
	LoadBlockPart(height uint64, index int) *Part

	LoadBlockCommit(height uint64) *Commit
	LoadSeenCommit(height uint64) *Commit
}

// BlockStore defines the BlockStore interface.
// UNSTABLE
type BlockStore interface {
	BlockStoreRPC
	SaveBlock(block *Block, blockParts *PartSet, seenCommit *Commit)
}
