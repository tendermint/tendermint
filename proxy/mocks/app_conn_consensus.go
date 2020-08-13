// Code generated by mockery v1.0.0. DO NOT EDIT.

package mocks

import (
	mock "github.com/stretchr/testify/mock"
	abcicli "github.com/tendermint/tendermint/abcix/client"

	types "github.com/tendermint/tendermint/abcix/types"
)

// AppConnConsensus is an autogenerated mock type for the AppConnConsensus type
type AppConnConsensus struct {
	mock.Mock
}

// CommitSync provides a mock function with given fields:
func (_m *AppConnConsensus) CommitSync() (*types.ResponseCommit, error) {
	ret := _m.Called()

	var r0 *types.ResponseCommit
	if rf, ok := ret.Get(0).(func() *types.ResponseCommit); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*types.ResponseCommit)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func() error); ok {
		r1 = rf()
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// CreateBlockSync provides a mock function with given fields: _a0, _a1
func (_m *AppConnConsensus) CreateBlockSync(_a0 types.RequestCreateBlock, _a1 types.MempoolIter) (*types.ResponseCreateBlock, error) {
	ret := _m.Called(_a0, _a1)

	var r0 *types.ResponseCreateBlock
	if rf, ok := ret.Get(0).(func(types.RequestCreateBlock, types.MempoolIter) *types.ResponseCreateBlock); ok {
		r0 = rf(_a0, _a1)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*types.ResponseCreateBlock)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(types.RequestCreateBlock, types.MempoolIter) error); ok {
		r1 = rf(_a0, _a1)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// DeliverBlockSync provides a mock function with given fields: _a0
func (_m *AppConnConsensus) DeliverBlockSync(_a0 types.RequestDeliverBlock) (*types.ResponseDeliverBlock, error) {
	ret := _m.Called(_a0)

	var r0 *types.ResponseDeliverBlock
	if rf, ok := ret.Get(0).(func(types.RequestDeliverBlock) *types.ResponseDeliverBlock); ok {
		r0 = rf(_a0)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*types.ResponseDeliverBlock)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(types.RequestDeliverBlock) error); ok {
		r1 = rf(_a0)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// Error provides a mock function with given fields:
func (_m *AppConnConsensus) Error() error {
	ret := _m.Called()

	var r0 error
	if rf, ok := ret.Get(0).(func() error); ok {
		r0 = rf()
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// InitChainSync provides a mock function with given fields: _a0
func (_m *AppConnConsensus) InitChainSync(_a0 types.RequestInitChain) (*types.ResponseInitChain, error) {
	ret := _m.Called(_a0)

	var r0 *types.ResponseInitChain
	if rf, ok := ret.Get(0).(func(types.RequestInitChain) *types.ResponseInitChain); ok {
		r0 = rf(_a0)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*types.ResponseInitChain)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(types.RequestInitChain) error); ok {
		r1 = rf(_a0)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// SetResponseCallback provides a mock function with given fields: _a0
func (_m *AppConnConsensus) SetResponseCallback(_a0 abcicli.Callback) {
	_m.Called(_a0)
}
