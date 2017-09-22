// Copyright 2017 Tendermint. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
	Update(height int, txs Txs)
	Flush()

	TxsAvailable() <-chan int
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
func (m MockMempool) Update(height int, txs Txs)                   {}
func (m MockMempool) Flush()                                       {}
func (m MockMempool) TxsAvailable() <-chan int                     { return make(chan int) }
func (m MockMempool) EnableTxsAvailable()                          {}

//------------------------------------------------------
// blockstore

// BlockStoreRPC is the block store interface used by the RPC.
// UNSTABLE
type BlockStoreRPC interface {
	Height() int

	LoadBlockMeta(height int) *BlockMeta
	LoadBlock(height int) *Block
	LoadBlockPart(height int, index int) *Part

	LoadBlockCommit(height int) *Commit
	LoadSeenCommit(height int) *Commit
}

// BlockStore defines the BlockStore interface.
// UNSTABLE
type BlockStore interface {
	BlockStoreRPC
	SaveBlock(block *Block, blockParts *PartSet, seenCommit *Commit)
}
