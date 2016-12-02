package consensus

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/tendermint/tendermint/config/tendermint_test"

	"github.com/tendermint/go-events"
	"github.com/tendermint/go-logger"
	"github.com/tendermint/go-p2p"
	"github.com/tendermint/tendermint/types"
	"github.com/tendermint/tmsp/example/dummy"
)

func init() {
	config = tendermint_test.ResetConfig("consensus_reactor_test")
}

func resetConfigTimeouts() {
	logger.SetLogLevel("info")
	//config.Set("log_level", "notice")
	config.Set("timeout_propose", 2000)
	//	config.Set("timeout_propose_delta", 500)
	//	config.Set("timeout_prevote", 1000)
	//	config.Set("timeout_prevote_delta", 500)
	//	config.Set("timeout_precommit", 1000)
	//	config.Set("timeout_precommit_delta", 500)
	config.Set("timeout_commit", 1000)
}

//----------------------------------------------
// in-process testnets

// Ensure a testnet makes blocks
func TestReactor(t *testing.T) {
	resetConfigTimeouts()
	N := 4
	css := randConsensusNet(N)
	reactors := make([]*ConsensusReactor, N)
	eventChans := make([]chan interface{}, N)
	for i := 0; i < N; i++ {
		reactors[i] = NewConsensusReactor(css[i], false)

		eventSwitch := events.NewEventSwitch()
		_, err := eventSwitch.Start()
		if err != nil {
			t.Fatalf("Failed to start switch: %v", err)
		}

		reactors[i].SetEventSwitch(eventSwitch)
		eventChans[i] = subscribeToEvent(eventSwitch, "tester", types.EventStringNewBlock(), 1)
	}
	// make connected switches and start all reactors
	p2p.MakeConnectedSwitches(N, func(i int, s *p2p.Switch) *p2p.Switch {
		s.AddReactor("CONSENSUS", reactors[i])
		return s
	}, p2p.Connect2Switches)

	// wait till everyone makes the first new block
	timeoutWaitGroup(t, N, func(wg *sync.WaitGroup, j int) {
		<-eventChans[j]
		wg.Done()
	})
}

//-------------------------------------------------------------
// ensure we can make blocks despite cycling a validator set

func TestValidatorSetChanges(t *testing.T) {
	resetConfigTimeouts()
	nPeers := 8
	nVals := 4
	css := randConsensusNetWithPeers(nVals, nPeers)
	reactors := make([]*ConsensusReactor, nPeers)
	eventChans := make([]chan interface{}, nPeers)
	for i := 0; i < nPeers; i++ {
		reactors[i] = NewConsensusReactor(css[i], false)

		eventSwitch := events.NewEventSwitch()
		_, err := eventSwitch.Start()
		if err != nil {
			t.Fatalf("Failed to start switch: %v", err)
		}

		reactors[i].SetEventSwitch(eventSwitch)
		eventChans[i] = subscribeToEventRespond(eventSwitch, "tester", types.EventStringNewBlock())
	}
	p2p.MakeConnectedSwitches(nPeers, func(i int, s *p2p.Switch) *p2p.Switch {
		s.AddReactor("CONSENSUS", reactors[i])
		return s
	}, p2p.Connect2Switches)

	// map of active validators
	activeVals := make(map[string]struct{})
	for i := 0; i < nVals; i++ {
		activeVals[string(css[i].privValidator.GetAddress())] = struct{}{}
	}

	// wait till everyone makes block 1
	timeoutWaitGroup(t, nPeers, func(wg *sync.WaitGroup, j int) {
		<-eventChans[j]
		eventChans[j] <- struct{}{}
		wg.Done()
	})

	newValidatorPubKey := css[nVals].privValidator.(*types.PrivValidator).PubKey
	newValidatorTx := dummy.MakeValSetChangeTx(newValidatorPubKey.Bytes(), uint64(testMinPower))

	// wait till everyone makes block 2
	// ensure the commit includes all validators
	// send newValTx to change vals in block 3
	waitForAndValidateBlock(t, nPeers, activeVals, eventChans, css, newValidatorTx)

	// wait till everyone makes block 3.
	// it includes the commit for block 2, which is by the original validator set
	waitForAndValidateBlock(t, nPeers, activeVals, eventChans, css)

	// wait till everyone makes block 4.
	// it includes the commit for block 3, which is by the original validator set
	waitForAndValidateBlock(t, nPeers, activeVals, eventChans, css)

	// the commits for block 4 should be with the updated validator set
	activeVals[string(newValidatorPubKey.Address())] = struct{}{}

	// wait till everyone makes block 5
	// it includes the commit for block 4, which should have the updated validator set
	waitForAndValidateBlock(t, nPeers, activeVals, eventChans, css)

	// TODO: test more changes!
}

func waitForAndValidateBlock(t *testing.T, n int, activeVals map[string]struct{}, eventChans []chan interface{}, css []*ConsensusState, txs ...[]byte) {
	timeoutWaitGroup(t, n, func(wg *sync.WaitGroup, j int) {
		newBlock := <-eventChans[j]
		err := validateBlock(newBlock.(types.EventDataNewBlock).Block, activeVals)
		if err != nil {
			t.Fatal(err)
		}
		for _, tx := range txs {
			css[j].mempool.CheckTx(tx, nil)
		}

		eventChans[j] <- struct{}{}
		wg.Done()
	})
}

// expects high synchrony!
func validateBlock(block *types.Block, activeVals map[string]struct{}) error {
	if block.LastCommit.Size() != len(activeVals) {
		return fmt.Errorf("Commit size doesn't match number of active validators. Got %d, expected %d", block.LastCommit.Size(), len(activeVals))
	}

	for _, vote := range block.LastCommit.Precommits {
		if _, ok := activeVals[string(vote.ValidatorAddress)]; !ok {
			return fmt.Errorf("Found vote for unactive validator %X", vote.ValidatorAddress)
		}
	}
	return nil
}

func timeoutWaitGroup(t *testing.T, n int, f func(*sync.WaitGroup, int)) {
	wg := new(sync.WaitGroup)
	wg.Add(n)
	for i := 0; i < n; i++ {
		go f(wg, i)
	}

	// Make wait into a channel
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	tick := time.NewTicker(time.Second * 3)
	select {
	case <-done:
	case <-tick.C:
		t.Fatalf("Timed out waiting for all validators to commit a block")
	}
}
