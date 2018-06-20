package client_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/rpc/client"
	"github.com/tendermint/tendermint/rpc/client/mock"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
)

func TestWaitForHeight(t *testing.T) {
	assert, require := assert.New(t), require.New(t)

	// test with error result - immediate failure
	m := &mock.StatusMock{
		Call: mock.Call{
			Error: errors.New("bye"),
		},
	}
	r := mock.NewStatusRecorder(m)

	// connection failure always leads to error
	err := client.WaitForHeight(r, 8, nil)
	require.NotNil(err)
	require.Equal("bye", err.Error())
	// we called status once to check
	require.Equal(1, len(r.Calls))

	// now set current block height to 10
	m.Call = mock.Call{
		Response: &ctypes.ResultStatus{SyncInfo: ctypes.SyncInfo{LatestBlockHeight: 10}},
	}

	// we will not wait for more than 10 blocks
	err = client.WaitForHeight(r, 40, nil)
	require.NotNil(err)
	require.True(strings.Contains(err.Error(), "aborting"))
	// we called status once more to check
	require.Equal(2, len(r.Calls))

	// waiting for the past returns immediately
	err = client.WaitForHeight(r, 5, nil)
	require.Nil(err)
	// we called status once more to check
	require.Equal(3, len(r.Calls))

	// since we can't update in a background goroutine (test --race)
	// we use the callback to update the status height
	myWaiter := func(delta int64) error {
		// update the height for the next call
		m.Call.Response = &ctypes.ResultStatus{SyncInfo: ctypes.SyncInfo{LatestBlockHeight: 15}}
		return client.DefaultWaitStrategy(delta)
	}

	// we wait for a few blocks
	err = client.WaitForHeight(r, 12, myWaiter)
	require.Nil(err)
	// we called status once to check
	require.Equal(5, len(r.Calls))

	pre := r.Calls[3]
	require.Nil(pre.Error)
	prer, ok := pre.Response.(*ctypes.ResultStatus)
	require.True(ok)
	assert.Equal(int64(10), prer.SyncInfo.LatestBlockHeight)

	post := r.Calls[4]
	require.Nil(post.Error)
	postr, ok := post.Response.(*ctypes.ResultStatus)
	require.True(ok)
	assert.Equal(int64(15), postr.SyncInfo.LatestBlockHeight)
}
