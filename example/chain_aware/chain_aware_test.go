package main

import (
	"strconv"
	"strings"
	"testing"

	. "github.com/tendermint/go-common"
	"github.com/tendermint/abci/client"
	"github.com/tendermint/abci/server"
	"github.com/tendermint/abci/types"
)

func TestChainAware(t *testing.T) {

	app := NewChainAwareApplication()

	// Start the listener
	srv, err := server.NewServer("unix://test.sock", "socket", app)
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Stop()

	// Connect to the socket
	client, err := abcicli.NewSocketClient("unix://test.sock", false)
	if err != nil {
		Exit(Fmt("Error starting socket client: %v", err.Error()))
	}
	client.Start()
	defer client.Stop()

	n := uint64(5)
	hash := []byte("fake block hash")
	header := &types.Header{}
	for i := uint64(0); i < n; i++ {
		client.BeginBlockSync(hash, header)
		client.EndBlockSync(i)
		client.CommitSync()
	}

	r := app.Query(nil)
	spl := strings.Split(string(r.Data), ",")
	if len(spl) != 2 {
		t.Fatal("expected %d,%d ; got %s", n, n, string(r.Data))
	}
	beginCount, _ := strconv.Atoi(spl[0])
	endCount, _ := strconv.Atoi(spl[1])
	if uint64(beginCount) != n {
		t.Fatalf("expected beginCount of %d, got %d", n, beginCount)
	} else if uint64(endCount) != n {
		t.Fatalf("expected endCount of %d, got %d", n, endCount)
	}
}
