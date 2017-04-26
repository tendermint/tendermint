package mock_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	data "github.com/tendermint/go-wire/data"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"

	"github.com/tendermint/tendermint/rpc/client/mock"
)

func TestStatus(t *testing.T) {
	assert, require := assert.New(t), require.New(t)

	m := &mock.StatusMock{
		Call: mock.Call{
			Response: &ctypes.ResultStatus{
				LatestBlockHash:   data.Bytes("block"),
				LatestAppHash:     data.Bytes("app"),
				LatestBlockHeight: 10,
			}},
	}

	r := mock.NewStatusRecorder(m)
	require.Equal(0, len(r.Calls))

	// make sure response works proper
	status, err := r.Status()
	require.Nil(err, "%+v", err)
	assert.EqualValues("block", status.LatestBlockHash)
	assert.EqualValues(10, status.LatestBlockHeight)

	// make sure recorder works properly
	require.Equal(1, len(r.Calls))
	rs := r.Calls[0]
	assert.Equal("status", rs.Name)
	assert.Nil(rs.Args)
	assert.Nil(rs.Error)
	require.NotNil(rs.Response)
	st, ok := rs.Response.(*ctypes.ResultStatus)
	require.True(ok)
	assert.EqualValues("block", st.LatestBlockHash)
	assert.EqualValues(10, st.LatestBlockHeight)
}
