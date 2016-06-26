package consensus

import (
	"net"
	"sync"
	"testing"
	"time"

	"github.com/tendermint/tendermint/config/tendermint_test"

	. "github.com/tendermint/go-common"
	dbm "github.com/tendermint/go-db"
	"github.com/tendermint/go-events"
	"github.com/tendermint/go-p2p"
	bc "github.com/tendermint/tendermint/blockchain"
	"github.com/tendermint/tendermint/types"
)

func init() {
	config = tendermint_test.ResetConfig("consensus_reactor_test")
}

func resetConfigTimeouts() {
	config.Set("log_level", "notice")
	config.Set("timeout_propose", 2000)
	//	config.Set("timeout_propose_delta", 500)
	//	config.Set("timeout_prevote", 1000)
	//	config.Set("timeout_prevote_delta", 500)
	//	config.Set("timeout_precommit", 1000)
	//	config.Set("timeout_precommit_delta", 500)
	//	config.Set("timeout_commit", 1000)
}

func TestReactor(t *testing.T) {
	resetConfigTimeouts()
	N := 4
	css := randConsensusNet(N)
	reactors := make([]*ConsensusReactor, N)
	eventChans := make([]chan interface{}, N)
	for i := 0; i < N; i++ {
		blockStoreDB := dbm.NewDB(Fmt("blockstore%d", i), config.GetString("db_backend"), config.GetString("db_dir"))
		blockStore := bc.NewBlockStore(blockStoreDB)
		reactors[i] = NewConsensusReactor(css[i], blockStore, false)
		reactors[i].SetPrivValidator(css[i].privValidator)

		eventSwitch := events.NewEventSwitch()
		_, err := eventSwitch.Start()
		if err != nil {
			t.Fatalf("Failed to start switch: %v", err)
		}

		reactors[i].SetEventSwitch(eventSwitch)
		eventChans[i] = subscribeToEvent(eventSwitch, "tester", types.EventStringNewBlock(), 1)
	}
	p2p.MakeConnectedSwitches(N, func(i int, s *p2p.Switch) *p2p.Switch {
		s.AddReactor("CONSENSUS", reactors[i])
		return s
	}, net.Pipe)

	// wait till everyone makes the first new block
	wg := new(sync.WaitGroup)
	wg.Add(N)
	for i := 0; i < N; i++ {
		go func(j int) {
			<-eventChans[j]
			wg.Done()
		}(i)
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	tick := time.NewTicker(time.Second * 3)
	select {
	case <-done:
	case <-tick.C:
		t.Fatalf("Timed out waiting for all validators to commit first block")
	}
}
