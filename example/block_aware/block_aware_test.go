package main

import (
	"strconv"
	"strings"
	"testing"

	abcicli "github.com/tendermint/abci/client"
	"github.com/tendermint/abci/server"
	"github.com/tendermint/abci/types"
	"github.com/tendermint/tmlibs/log"
)

func TestChainAware(t *testing.T) {
	app := NewChainAwareApplication()

	// Start the listener
	srv, err := server.NewServer("unix://test.sock", "socket", app)
	if err != nil {
		t.Fatal(err)
	}
	srv.SetLogger(log.TestingLogger().With("module", "abci-server"))
	if _, err := srv.Start(); err != nil {
		t.Fatal(err.Error())
	}
	defer srv.Stop()

	// Connect to the socket
	client := abcicli.NewSocketClient("unix://test.sock", false)
	client.SetLogger(log.TestingLogger().With("module", "abci-client"))
	if _, err := client.Start(); err != nil {
		t.Fatalf("Error starting socket client: %v", err.Error())
	}
	defer client.Stop()

	n := uint64(5)
	hash := []byte("fake block hash")
	header := &types.Header{}
	for i := uint64(0); i < n; i++ {
		client.BeginBlockSync(types.RequestBeginBlock{hash, header})
		client.EndBlockSync(i)
		client.CommitSync()
	}

	r := app.Query(types.RequestQuery{})
	spl := strings.Split(string(r.Value), ",")
	if len(spl) != 2 {
		t.Fatal("expected %d,%d ; got %s", n, n, string(r.Value))
	}
	beginCount, _ := strconv.Atoi(spl[0])
	endCount, _ := strconv.Atoi(spl[1])
	if uint64(beginCount) != n {
		t.Fatalf("expected beginCount of %d, got %d", n, beginCount)
	} else if uint64(endCount) != n {
		t.Fatalf("expected endCount of %d, got %d", n, endCount)
	}
}
