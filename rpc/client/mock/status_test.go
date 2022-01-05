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
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

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
	require.Equal(t, 0, len(r.Calls))

	// make sure response works proper
	status, err := r.Status(ctx)
	require.NoError(t, err)
	assert.EqualValues(t, "block", status.SyncInfo.LatestBlockHash)
	assert.EqualValues(t, 10, status.SyncInfo.LatestBlockHeight)
	assert.EqualValues(t, 20, status.SyncInfo.MaxPeerBlockHeight)
	assert.EqualValues(t, time.Second, status.SyncInfo.TotalSyncedTime)
	assert.EqualValues(t, time.Minute, status.SyncInfo.RemainingTime)

	// make sure recorder works properly
	require.Equal(t, 1, len(r.Calls))
	rs := r.Calls[0]
	assert.Equal(t, "status", rs.Name)
	assert.Nil(t, rs.Args)
	assert.Nil(t, rs.Error)
	require.NotNil(t, rs.Response)
	st, ok := rs.Response.(*coretypes.ResultStatus)
	require.True(t, ok)
	assert.EqualValues(t, "block", st.SyncInfo.LatestBlockHash)
	assert.EqualValues(t, 10, st.SyncInfo.LatestBlockHeight)
	assert.EqualValues(t, 20, st.SyncInfo.MaxPeerBlockHeight)
	assert.EqualValues(t, time.Second, status.SyncInfo.TotalSyncedTime)
	assert.EqualValues(t, time.Minute, status.SyncInfo.RemainingTime)

	assert.EqualValues(t, 10, st.SyncInfo.TotalSnapshots)
	assert.EqualValues(t, time.Duration(10), st.SyncInfo.ChunkProcessAvgTime)
	assert.EqualValues(t, 10, st.SyncInfo.SnapshotHeight)
	assert.EqualValues(t, 9, status.SyncInfo.SnapshotChunksCount)
	assert.EqualValues(t, 10, status.SyncInfo.SnapshotChunksTotal)
	assert.EqualValues(t, 9, status.SyncInfo.BackFilledBlocks)
	assert.EqualValues(t, 10, status.SyncInfo.BackFillBlocksTotal)
}
