package mock_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tendermint/tendermint/libs/bytes"
	"github.com/tendermint/tendermint/rpc/client/mock"
	"github.com/tendermint/tendermint/rpc/coretypes"
)

func TestStatus(t *testing.T) {
	assert, require := assert.New(t), require.New(t)

	m := &mock.StatusMock{
		Call: mock.Call{
			Response: &coretypes.ResultStatus{
				SyncInfo: coretypes.SyncInfo{
					LatestBlockHash:     bytes.HexBytes("block"),
					LatestAppHash:       bytes.HexBytes("app"),
					LatestBlockHeight:   10,
					MaxPeerBlockHeight:  20,
					TotalSyncedTime:     time.Second,
					RemainingTime:       time.Minute,
					TotalSnapshots:      10,
					ChunkProcessAvgTime: time.Duration(10),
					SnapshotHeight:      10,
					SnapshotChunksCount: 9,
					SnapshotChunksTotal: 10,
					BackFilledBlocks:    9,
					BackFillBlocksTotal: 10,
				},
			}},
	}

	r := mock.NewStatusRecorder(m)
	require.Equal(0, len(r.Calls))

	// make sure response works proper
	status, err := r.Status(context.Background())
	require.Nil(err, "%+v", err)
	assert.EqualValues("block", status.SyncInfo.LatestBlockHash)
	assert.EqualValues(10, status.SyncInfo.LatestBlockHeight)
	assert.EqualValues(20, status.SyncInfo.MaxPeerBlockHeight)
	assert.EqualValues(time.Second, status.SyncInfo.TotalSyncedTime)
	assert.EqualValues(time.Minute, status.SyncInfo.RemainingTime)

	// make sure recorder works properly
	require.Equal(1, len(r.Calls))
	rs := r.Calls[0]
	assert.Equal("status", rs.Name)
	assert.Nil(rs.Args)
	assert.Nil(rs.Error)
	require.NotNil(rs.Response)
	st, ok := rs.Response.(*coretypes.ResultStatus)
	require.True(ok)
	assert.EqualValues("block", st.SyncInfo.LatestBlockHash)
	assert.EqualValues(10, st.SyncInfo.LatestBlockHeight)
	assert.EqualValues(20, st.SyncInfo.MaxPeerBlockHeight)
	assert.EqualValues(time.Second, status.SyncInfo.TotalSyncedTime)
	assert.EqualValues(time.Minute, status.SyncInfo.RemainingTime)

	assert.EqualValues(10, st.SyncInfo.TotalSnapshots)
	assert.EqualValues(time.Duration(10), st.SyncInfo.ChunkProcessAvgTime)
	assert.EqualValues(10, st.SyncInfo.SnapshotHeight)
	assert.EqualValues(9, status.SyncInfo.SnapshotChunksCount)
	assert.EqualValues(10, status.SyncInfo.SnapshotChunksTotal)
	assert.EqualValues(9, status.SyncInfo.BackFilledBlocks)
	assert.EqualValues(10, status.SyncInfo.BackFillBlocksTotal)
}
