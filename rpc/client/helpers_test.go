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
		Response: &ctypes.ResultStatus{LatestBlockHeight: 10},
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
	myWaiter := func(delta int) error {
		// update the height for the next call
		m.Call.Response = &ctypes.ResultStatus{LatestBlockHeight: 15}
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
	assert.Equal(10, prer.LatestBlockHeight)

	post := r.Calls[4]
	require.Nil(post.Error)
	postr, ok := post.Response.(*ctypes.ResultStatus)
	require.True(ok)
	assert.Equal(15, postr.LatestBlockHeight)
}
