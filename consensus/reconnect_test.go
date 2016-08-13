package consensus

import (
	"sync"
	"testing"
	"time"

	. "github.com/tendermint/go-common"
	"github.com/tendermint/tendermint/types"
	"github.com/tendermint/tmsp/example/dummy"
	"github.com/tendermint/tmsp/server"
)

func TestAppRestart1(t *testing.T) {

	state, privVal := fixedState()

	proxyAddr := "tcp://localhost:36658"
	transport := config.GetString("tmsp")

	var tmspServer Service
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		var err error
		tmspServer, err = server.NewServer(proxyAddr, transport, dummy.NewDummyApplication())
		if err != nil {
			t.Fatal(err)
		}
		wg.Done()
	}()

	wg.Wait()
	cs, err := newConsensusStateProxyApp(state, privVal, proxyAddr, transport)
	if err != nil {
		t.Fatal(err)
	}

	EnsureDir(config.GetString("db_dir"), 0700) // incase we use memdb, cswal still gets written here

	newBlockCh := subscribeToEvent(cs.evsw, "tester", types.EventStringNewBlock(), 0)
	voteCh := subscribeToEvent(cs.evsw, "tester", types.EventStringVote(), 0)

	// start timeout and receive routines
	cs.Start()

	_, _, _ = <-voteCh, <-voteCh, <-newBlockCh

	log.Warn("Stopping tmsp server before hitting proxy app. Consensus state should not stop")
	tmspServer.Stop()
	tmspServer.Reset()

	_, _ = <-voteCh, <-voteCh

	timer := time.NewTimer(time.Second * 3)
	select {
	case <-newBlockCh:
		t.Fatal("expected no new block as app crashed")
	case <-timer.C:
	}

	time.Sleep(time.Second * 3)
	log.Warn("Restart tmsp server")
	if _, err := tmspServer.Start(); err != nil {
		t.Fatal(err)
	}

	timer = time.NewTimer(time.Second * 10)
	select {
	case <-newBlockCh:
	case <-timer.C:
		t.Fatal("expected new block after app restart")
	}
}

func TestAppRestart2(t *testing.T) {

	state, privVal := fixedState()

	proxyAddr := "tcp://localhost:36658"
	transport := config.GetString("tmsp")

	var tmspServer Service
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		var err error
		tmspServer, err = server.NewServer(proxyAddr, transport, dummy.NewDummyApplication())
		if err != nil {
			t.Fatal(err)
		}
		wg.Done()
	}()

	wg.Wait()
	cs, err := newConsensusStateProxyApp(state, privVal, proxyAddr, transport)
	if err != nil {
		t.Fatal(err)
	}

	EnsureDir(config.GetString("db_dir"), 0700) // incase we use memdb, cswal still gets written here

	newBlockCh := subscribeToEvent(cs.evsw, "tester", types.EventStringNewBlock(), 0)
	voteCh := subscribeToEvent(cs.evsw, "tester", types.EventStringVote(), 0)

	// start timeout and receive routines
	cs.Start()

	_, _, _ = <-voteCh, <-voteCh, <-newBlockCh

	<-voteCh

	log.Warn("Stopping tmsp server to hit proxy app. Consensus state should stop")
	tmspServer.Stop()
	tmspServer.Reset()

	<-voteCh

	timer := time.NewTimer(time.Second * 3)
	select {
	case <-newBlockCh:
		t.Fatal("expected no new block as app crashed")
	case <-timer.C:
	}

	time.Sleep(time.Second * 3)
	log.Warn("Restart tmsp server")
	if _, err := tmspServer.Start(); err != nil {
		t.Fatal(err)
	}

	// the two votes should be replayed
	_, _ = <-voteCh, <-voteCh

	timer = time.NewTimer(time.Second * 10)
	select {
	case <-newBlockCh:
	case <-timer.C:
		t.Fatal("expected new block after app restart")
	}

}
