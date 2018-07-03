package abcicli_test

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/abci/client"
	"github.com/tendermint/tendermint/abci/server"
	"github.com/tendermint/tendermint/abci/types"
)

func TestSocketClientStopForErrorDeadlock(t *testing.T) {
	c := abcicli.NewSocketClient(":80", false)
	err := errors.New("foo-tendermint")

	// See Issue https://github.com/tendermint/abci/issues/114
	doneChan := make(chan bool)
	go func() {
		defer close(doneChan)
		c.StopForError(err)
		c.StopForError(err)
	}()

	select {
	case <-doneChan:
	case <-time.After(time.Second * 4):
		t.Fatalf("Test took too long, potential deadlock still exists")
	}
}

func TestProperSyncCalls(t *testing.T) {
	port := ":29385"
	app := new(types.BaseApplication)

	s, err := server.NewServer(port, "socket", app)
	require.NoError(t, err)
	err = s.Start()
	require.NoError(t, err)
	defer s.Stop()

	c := abcicli.NewSocketClient(port, true)
	err = c.Start()
	require.NoError(t, err)
	defer c.Stop()

	resp := make(chan error, 1)
	go func() {
		_, err := c.BeginBlockSync(types.RequestBeginBlock{})
		resp <- err
	}()

	select {
	case <-time.After(time.Second):
		require.Fail(t, "No response arrived")
	case err, ok := <-resp:
		require.True(t, ok, "Must not close channel")
		assert.NoError(t, err, "This should return success")
	}

}
