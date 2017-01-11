package consensus

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/tendermint/tendermint/config/tendermint_test"

	"github.com/tendermint/go-events"
	"github.com/tendermint/go-p2p"
	"github.com/tendermint/tendermint/types"
	"github.com/tendermint/tmsp/example/dummy"
)

func init() {
	config = tendermint_test.ResetConfig("consensus_reactor_test")
}

//----------------------------------------------
// in-process testnets

// Ensure a testnet makes blocks
func TestReactor(t *testing.T) {
	N := 4
	css := randConsensusNet(N, "consensus_reactor_test", newMockTickerFunc(true))
	reactors := make([]*ConsensusReactor, N)
	eventChans := make([]chan interface{}, N)
	for i := 0; i < N; i++ {
		reactors[i] = NewConsensusReactor(css[i], true) // so we dont start the consensus states

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

	// start the state machines
	for i := 0; i < N; i++ {
		s := reactors[i].conS.GetState()
		reactors[i].SwitchToConsensus(s)
	}

	// wait till everyone makes the first new block
	timeoutWaitGroup(t, N, func(wg *sync.WaitGroup, j int) {
		<-eventChans[j]
		wg.Done()
	})
}

//-------------------------------------------------------------
// ensure we can make blocks despite cycling a validator set

func TestValidatorSetChanges(t *testing.T) {
	nPeers := 8
	nVals := 4
	css := randConsensusNetWithPeers(nVals, nPeers, "consensus_val_set_changes_test", newMockTickerFunc(true))
	reactors := make([]*ConsensusReactor, nPeers)
	eventChans := make([]chan interface{}, nPeers)
	for i := 0; i < nPeers; i++ {
		reactors[i] = NewConsensusReactor(css[i], true) // so we dont start the consensus states

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

	// now that everyone is connected,  start the state machines
	// If we started the state machines before everyone was connected,
	// we'd block when the cs fires NewBlockEvent and the peers are trying to start their reactors
	for i := 0; i < nPeers; i++ {
		s := reactors[i].conS.GetState()
		reactors[i].SwitchToConsensus(s)
	}

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

	//---------------------------------------------------------------------------
	log.Info("Testing adding one validator")

	newValidatorPubKey1 := css[nVals].privValidator.(*types.PrivValidator).PubKey
	newValidatorTx1 := dummy.MakeValSetChangeTx(newValidatorPubKey1.Bytes(), uint64(testMinPower))

	// wait till everyone makes block 2
	// ensure the commit includes all validators
	// send newValTx to change vals in block 3
	waitForAndValidateBlock(t, nPeers, activeVals, eventChans, css, newValidatorTx1)

	// wait till everyone makes block 3.
	// it includes the commit for block 2, which is by the original validator set
	waitForAndValidateBlock(t, nPeers, activeVals, eventChans, css)

	// wait till everyone makes block 4.
	// it includes the commit for block 3, which is by the original validator set
	waitForAndValidateBlock(t, nPeers, activeVals, eventChans, css)

	// the commits for block 4 should be with the updated validator set
	activeVals[string(newValidatorPubKey1.Address())] = struct{}{}

	// wait till everyone makes block 5
	// it includes the commit for block 4, which should have the updated validator set
	waitForAndValidateBlock(t, nPeers, activeVals, eventChans, css)

	//---------------------------------------------------------------------------
	log.Info("Testing changing the voting power of one validator")

	updateValidatorTx1 := dummy.MakeValSetChangeTx(newValidatorPubKey1.Bytes(), 25)
	previousTotalVotingPower := css[nVals].LastValidators.TotalVotingPower()

	waitForAndValidateBlock(t, nPeers, activeVals, eventChans, css, updateValidatorTx1)
	waitForAndValidateBlock(t, nPeers, activeVals, eventChans, css)
	waitForAndValidateBlock(t, nPeers, activeVals, eventChans, css)
	waitForAndValidateBlock(t, nPeers, activeVals, eventChans, css)
	if css[nVals].LastValidators.TotalVotingPower() == previousTotalVotingPower {
		t.Errorf("expected voting power to change (before: %d, after: %d)", previousTotalVotingPower, css[nVals].LastValidators.TotalVotingPower())
	}

	//---------------------------------------------------------------------------
	log.Info("Testing adding two validators at once")

	newValidatorPubKey2 := css[nVals+1].privValidator.(*types.PrivValidator).PubKey
	newValidatorTx2 := dummy.MakeValSetChangeTx(newValidatorPubKey2.Bytes(), uint64(testMinPower))

	newValidatorPubKey3 := css[nVals+2].privValidator.(*types.PrivValidator).PubKey
	newValidatorTx3 := dummy.MakeValSetChangeTx(newValidatorPubKey3.Bytes(), uint64(testMinPower))

	waitForAndValidateBlock(t, nPeers, activeVals, eventChans, css, newValidatorTx2, newValidatorTx3)
	waitForAndValidateBlock(t, nPeers, activeVals, eventChans, css)
	waitForAndValidateBlock(t, nPeers, activeVals, eventChans, css)
	activeVals[string(newValidatorPubKey2.Address())] = struct{}{}
	activeVals[string(newValidatorPubKey3.Address())] = struct{}{}
	waitForAndValidateBlock(t, nPeers, activeVals, eventChans, css)

	//---------------------------------------------------------------------------
	log.Info("Testing removing two validators at once")

	removeValidatorTx2 := dummy.MakeValSetChangeTx(newValidatorPubKey2.Bytes(), 0)
	removeValidatorTx3 := dummy.MakeValSetChangeTx(newValidatorPubKey3.Bytes(), 0)

	waitForAndValidateBlock(t, nPeers, activeVals, eventChans, css, removeValidatorTx2, removeValidatorTx3)
	waitForAndValidateBlock(t, nPeers, activeVals, eventChans, css)
	waitForAndValidateBlock(t, nPeers, activeVals, eventChans, css)
	delete(activeVals, string(newValidatorPubKey2.Address()))
	delete(activeVals, string(newValidatorPubKey3.Address()))
	waitForAndValidateBlock(t, nPeers, activeVals, eventChans, css)

}

func waitForAndValidateBlock(t *testing.T, n int, activeVals map[string]struct{}, eventChans []chan interface{}, css []*ConsensusState, txs ...[]byte) {
	timeoutWaitGroup(t, n, func(wg *sync.WaitGroup, j int) {
		newBlockI := <-eventChans[j]
		newBlock := newBlockI.(types.EventDataNewBlock).Block
		log.Info("Got block", "height", newBlock.Height, "validator", j)
		err := validateBlock(newBlock, activeVals)
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

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second * 3):
		t.Fatalf("Timed out waiting for all validators to commit a block")
	}
}
