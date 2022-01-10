package client_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tendermint/tendermint/rpc/client"
	"github.com/tendermint/tendermint/rpc/client/mock"
	"github.com/tendermint/tendermint/rpc/coretypes"
)

func TestWaitForHeight(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// test with error result - immediate failure
	m := &mock.StatusMock{
		Call: mock.Call{
			Error: errors.New("bye"),
		},
	}
	r := mock.NewStatusRecorder(m)

	// connection failure always leads to error
	err := client.WaitForHeight(ctx, r, 8, nil)
	require.Error(t, err)
	require.Equal(t, "bye", err.Error())

	// we called status once to check
	require.Equal(t, 1, len(r.Calls))

	// now set current block height to 10
	m.Call = mock.Call{
		Response: &coretypes.ResultStatus{SyncInfo: coretypes.SyncInfo{LatestBlockHeight: 10}},
	}

	// we will not wait for more than 10 blocks
	err = client.WaitForHeight(ctx, r, 40, nil)
	require.Error(t, err)
	require.True(t, strings.Contains(err.Error(), "aborting"))

	// we called status once more to check
	require.Equal(t, 2, len(r.Calls))

	// waiting for the past returns immediately
	err = client.WaitForHeight(ctx, r, 5, nil)
	require.NoError(t, err)

	// we called status once more to check
	require.Equal(t, 3, len(r.Calls))

	// since we can't update in a background goroutine (test --race)
	// we use the callback to update the status height
	myWaiter := func(delta int64) error {
		// update the height for the next call
		m.Call.Response = &coretypes.ResultStatus{SyncInfo: coretypes.SyncInfo{LatestBlockHeight: 15}}
		return client.DefaultWaitStrategy(delta)
	}

	// we wait for a few blocks
	err = client.WaitForHeight(ctx, r, 12, myWaiter)
	require.NoError(t, err)

	// we called status once to check
	require.Equal(t, 5, len(r.Calls))

	pre := r.Calls[3]
	require.Nil(t, pre.Error)
	prer, ok := pre.Response.(*coretypes.ResultStatus)
	require.True(t, ok)
	assert.Equal(t, int64(10), prer.SyncInfo.LatestBlockHeight)

	post := r.Calls[4]
	require.Nil(t, post.Error)
	postr, ok := post.Response.(*coretypes.ResultStatus)
	require.True(t, ok)
	assert.Equal(t, int64(15), postr.SyncInfo.LatestBlockHeight)
}
