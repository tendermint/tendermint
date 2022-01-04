package tests

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	abciclientent "github.com/tendermint/tendermint/abci/client"
	"github.com/tendermint/tendermint/abci/example/kvstore"
	abciserver "github.com/tendermint/tendermint/abci/server"
	"github.com/tendermint/tendermint/libs/log"
)

func TestClientServerNoAddrPrefix(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	const (
		addr      = "localhost:26658"
		transport = "socket"
	)
	app := kvstore.NewApplication()
	logger := log.NewTestingLogger(t)

	server, err := abciserver.NewServer(logger, addr, transport, app)
	assert.NoError(t, err, "expected no error on NewServer")
	err = server.Start(ctx)
	assert.NoError(t, err, "expected no error on server.Start")

	client, err := abciclientent.NewClient(logger, addr, transport, true)
	assert.NoError(t, err, "expected no error on NewClient")
	err = client.Start(ctx)
	assert.NoError(t, err, "expected no error on client.Start")
}
