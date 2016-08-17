package consensus

import (
	"testing"
	"time"

	. "github.com/tendermint/go-common"
	"github.com/tendermint/tendermint/types"
	"github.com/tendermint/tmsp/example/dummy"
	"github.com/tendermint/tmsp/server"
)

// app conn failing is detected by conS when attempting to ExecBlock.
// It will stop and let conR create and start a new one.
func TestAppRestart(t *testing.T) {

	state, privVal := fixedState()

	proxyAddr := "tcp://localhost:36658"
	transport := config.GetString("tmsp")

	var tmspServer Service
	var err error
	tmspServer, err = server.NewServer(proxyAddr, transport, dummy.NewDummyApplication())
	if err != nil {
		t.Fatal(err)
	}

	conR, err := newConsensusReactorProxyApp(state, privVal, proxyAddr, transport)
	if err != nil {
		t.Fatal(err)
	}

	EnsureDir(config.GetString("db_dir"), 0700) // incase we use memdb, cswal still gets written here

	newBlockCh := subscribeToEvent(conR.evsw, "tester", types.EventStringNewBlock(), 0)
	voteCh := subscribeToEvent(conR.evsw, "tester", types.EventStringVote(), 0)

	// start timeout and receive routines
	conR.Start()

	_, _, _ = <-voteCh, <-voteCh, <-newBlockCh

	log.Warn("Stopping tmsp server. We shouldn't notice until attempting to commit block")
	tmspServer.Stop()

	_, _ = <-voteCh, <-voteCh

	timer := time.NewTimer(time.Second * 3)
	select {
	case <-newBlockCh:
		t.Fatal("expected no new block as app crashed")
	case <-timer.C:
	}

	time.Sleep(time.Second * 3)
	log.Warn("Restart tmsp server")
	tmspServer, err = server.NewServer(proxyAddr, transport, dummy.NewDummyApplication())
	if err != nil {
		t.Fatal(err)
	}

	_, _ = <-voteCh, <-voteCh

	timer = time.NewTimer(time.Second * 10)
	select {
	case <-newBlockCh:
	case <-timer.C:
		t.Fatal("expected new block after app restart")
	}
}
