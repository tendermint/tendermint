package abcicli_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	abcicli "github.com/tendermint/tendermint/abci/client"
	"github.com/tendermint/tendermint/abci/server"
	"github.com/tendermint/tendermint/abci/types"
	tmrand "github.com/tendermint/tendermint/libs/rand"
	"github.com/tendermint/tendermint/libs/service"
)

func TestCalls(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	app := types.BaseApplication{}

	_, c := setupClientServer(t, app)

	resp := make(chan error, 1)
	go func() {
		res, err := c.Echo(ctx, "hello")
		require.NoError(t, err)
		require.NotNil(t, res)
		resp <- c.Error()
	}()

	select {
	case <-time.After(1 * time.Second):
		require.Fail(t, "No response arrived")
	case err, ok := <-resp:
		require.True(t, ok, "Must not close channel")
		assert.NoError(t, err, "This should return success")
	}
}

func setupClientServer(t *testing.T, app types.Application) (
	service.Service, abcicli.Client) {
	t.Helper()

	// some port between 20k and 30k
	port := 20000 + tmrand.Int32()%10000
	addr := fmt.Sprintf("localhost:%d", port)

	s := server.NewSocketServer(addr, app)
	err := s.Start()
	require.NoError(t, err)

	t.Cleanup(func() {
		if err := s.Stop(); err != nil {
			t.Error(err)
		}
	})

	c := abcicli.NewSocketClient(addr, true)
	err = c.Start()
	require.NoError(t, err)

	t.Cleanup(func() {
		if err := c.Stop(); err != nil {
			t.Error(err)
		}
	})

	return s, c
}
