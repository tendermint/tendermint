package main

import (
	"testing"

	"github.com/stretchr/testify/assert"

	abciclient "github.com/tendermint/abci/client"
	"github.com/tendermint/abci/example/dummy"
	abciserver "github.com/tendermint/abci/server"
)

func TestClientServerNoAddrPrefix(t *testing.T) {
	addr := "localhost:46658"
	transport := "socket"
	app := dummy.NewDummyApplication()

	server, err := abciserver.NewServer(addr, transport, app)
	assert.NoError(t, err, "expected no error on NewServer")
	_, err = server.Start()
	assert.NoError(t, err, "expected no error on server.Start")

	client, err := abciclient.NewClient(addr, transport, true)
	assert.NoError(t, err, "expected no error on NewClient")
	_, err = client.Start()
	assert.NoError(t, err, "expected no error on client.Start")
}
