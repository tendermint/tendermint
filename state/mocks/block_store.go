// Code generated by mockery. DO NOT EDIT.

package mocks

import (
	mock "github.com/stretchr/testify/mock"
	state "github.com/tendermint/tendermint/state"
	types "github.com/tendermint/tendermint/types"
)

// BlockStore is an autogenerated mock type for the BlockStore type
type BlockStore struct {
	mock.Mock
}

// Base provides a mock function with given fields:
func (_m *BlockStore) Base() int64 {
	ret := _m.Called()

	var r0 int64
	if rf, ok := ret.Get(0).(func() int64); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(int64)
	}

	return r0
}

// DeleteLatestBlock provides a mock function with given fields:
func (_m *BlockStore) DeleteLatestBlock() error {
	ret := _m.Called()

	var r0 error
	if rf, ok := ret.Get(0).(func() error); ok {
		r0 = rf()
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// Height provides a mock function with given fields:
func (_m *BlockStore) Height() int64 {
	ret := _m.Called()

	var r0 int64
	if rf, ok := ret.Get(0).(func() int64); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(int64)
	}

	return r0
}

// LoadBaseMeta provides a mock function with given fields:
func (_m *BlockStore) LoadBaseMeta() *types.BlockMeta {
	ret := _m.Called()

	var r0 *types.BlockMeta
	if rf, ok := ret.Get(0).(func() *types.BlockMeta); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*types.BlockMeta)
		}
	}

	return r0
}

// LoadBlock provides a mock function with given fields: height
func (_m *BlockStore) LoadBlock(height int64) *types.Block {
	ret := _m.Called(height)

	var r0 *types.Block
	if rf, ok := ret.Get(0).(func(int64) *types.Block); ok {
		r0 = rf(height)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*types.Block)
		}
	}

	return r0
}

// LoadBlockByHash provides a mock function with given fields: hash
func (_m *BlockStore) LoadBlockByHash(hash []byte) *types.Block {
	ret := _m.Called(hash)

	var r0 *types.Block
	if rf, ok := ret.Get(0).(func([]byte) *types.Block); ok {
		r0 = rf(hash)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*types.Block)
		}
	}

	return r0
}

// LoadBlockCommit provides a mock function with given fields: height
func (_m *BlockStore) LoadBlockCommit(height int64) *types.Commit {
	ret := _m.Called(height)

	var r0 *types.Commit
	if rf, ok := ret.Get(0).(func(int64) *types.Commit); ok {
		r0 = rf(height)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*types.Commit)
		}
	}

	return r0
}

// LoadBlockMeta provides a mock function with given fields: height
func (_m *BlockStore) LoadBlockMeta(height int64) *types.BlockMeta {
	ret := _m.Called(height)

	var r0 *types.BlockMeta
	if rf, ok := ret.Get(0).(func(int64) *types.BlockMeta); ok {
		r0 = rf(height)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*types.BlockMeta)
		}
	}

	return r0
}

// LoadBlockMetaByHash provides a mock function with given fields: hash
func (_m *BlockStore) LoadBlockMetaByHash(hash []byte) *types.BlockMeta {
	ret := _m.Called(hash)

	var r0 *types.BlockMeta
	if rf, ok := ret.Get(0).(func([]byte) *types.BlockMeta); ok {
		r0 = rf(hash)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*types.BlockMeta)
		}
	}

	return r0
}

// LoadBlockPart provides a mock function with given fields: height, index
func (_m *BlockStore) LoadBlockPart(height int64, index int) *types.Part {
	ret := _m.Called(height, index)

	var r0 *types.Part
	if rf, ok := ret.Get(0).(func(int64, int) *types.Part); ok {
		r0 = rf(height, index)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*types.Part)
		}
	}

	return r0
}

// LoadSeenCommit provides a mock function with given fields: height
func (_m *BlockStore) LoadSeenCommit(height int64) *types.Commit {
	ret := _m.Called(height)

	var r0 *types.Commit
	if rf, ok := ret.Get(0).(func(int64) *types.Commit); ok {
		r0 = rf(height)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*types.Commit)
		}
	}

	return r0
}

// PruneBlocks provides a mock function with given fields: height, _a1
func (_m *BlockStore) PruneBlocks(height int64, _a1 state.State) (uint64, int64, error) {
	ret := _m.Called(height, _a1)

	var r0 uint64
	if rf, ok := ret.Get(0).(func(int64, state.State) uint64); ok {
		r0 = rf(height, _a1)
	} else {
		r0 = ret.Get(0).(uint64)
	}

	var r1 int64
	if rf, ok := ret.Get(1).(func(int64, state.State) int64); ok {
		r1 = rf(height, _a1)
	} else {
		r1 = ret.Get(1).(int64)
	}

	var r2 error
	if rf, ok := ret.Get(2).(func(int64, state.State) error); ok {
		r2 = rf(height, _a1)
	} else {
		r2 = ret.Error(2)
	}

	return r0, r1, r2
}

// SaveBlock provides a mock function with given fields: block, blockParts, seenCommit
func (_m *BlockStore) SaveBlock(block *types.Block, blockParts *types.PartSet, seenCommit *types.Commit) {
	_m.Called(block, blockParts, seenCommit)
}

// Size provides a mock function with given fields:
func (_m *BlockStore) Size() int64 {
	ret := _m.Called()

	var r0 int64
	if rf, ok := ret.Get(0).(func() int64); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(int64)
	}

	return r0
}

type mockConstructorTestingTNewBlockStore interface {
	mock.TestingT
	Cleanup(func())
}

// NewBlockStore creates a new instance of BlockStore. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
func NewBlockStore(t mockConstructorTestingTNewBlockStore) *BlockStore {
	mock := &BlockStore{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
