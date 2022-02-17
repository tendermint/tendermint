package abciclient_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"math/rand"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	abciclient "github.com/tendermint/tendermint/abci/client"
	"github.com/tendermint/tendermint/abci/server"
	"github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/libs/service"
)

func TestProperSyncCalls(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	app := slowApp{}
	logger := log.NewNopLogger()

	_, c := setupClientServer(ctx, t, logger, app)

	resp := make(chan error, 1)
	go func() {
		rsp, err := c.FinalizeBlock(ctx, types.RequestFinalizeBlock{})
		assert.NoError(t, err)
		assert.NoError(t, c.Flush(ctx))
		assert.NotNil(t, rsp)
		select {
		case <-ctx.Done():
		case resp <- c.Error():
		}
	}()

	select {
	case <-time.After(time.Second):
		require.Fail(t, "No response arrived")
	case err, ok := <-resp:
		require.True(t, ok, "Must not close channel")
		assert.NoError(t, err, "This should return success")
	}
}

func setupClientServer(
	ctx context.Context,
	t *testing.T,
	logger log.Logger,
	app types.Application,
) (service.Service, abciclient.Client) {
	t.Helper()

	// some port between 20k and 30k
	port := 20000 + rand.Int31()%10000
	addr := fmt.Sprintf("localhost:%d", port)

	s, err := server.NewServer(logger, addr, "socket", app)
	require.NoError(t, err)
	require.NoError(t, s.Start(ctx))
	t.Cleanup(s.Wait)

	c := abciclient.NewSocketClient(logger, addr, true)
	require.NoError(t, c.Start(ctx))
	t.Cleanup(c.Wait)

	require.True(t, s.IsRunning())
	require.True(t, c.IsRunning())

	return s, c
}

type slowApp struct {
	types.BaseApplication
}

func (slowApp) FinalizeBlock(req types.RequestFinalizeBlock) types.ResponseFinalizeBlock {
	time.Sleep(200 * time.Millisecond)
	return types.ResponseFinalizeBlock{}
}
