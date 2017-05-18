package consensus

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/tendermint/abci/example/dummy"
	"github.com/tendermint/tendermint/p2p"
	"github.com/tendermint/tendermint/types"
	"github.com/tendermint/tmlibs/events"
)

func init() {
	config = ResetConfig("consensus_reactor_test")
}

//----------------------------------------------
// in-process testnets

func startConsensusNet(t *testing.T, css []*ConsensusState, N int, subscribeEventRespond bool) ([]*ConsensusReactor, []chan interface{}) {
	reactors := make([]*ConsensusReactor, N)
	eventChans := make([]chan interface{}, N)
	logger := consensusLogger()
	for i := 0; i < N; i++ {
		reactors[i] = NewConsensusReactor(css[i], true) // so we dont start the consensus states
		reactors[i].SetLogger(logger.With("validator", i))

		eventSwitch := events.NewEventSwitch()
		eventSwitch.SetLogger(logger.With("module", "events", "validator", i))
		_, err := eventSwitch.Start()
		if err != nil {
			t.Fatalf("Failed to start switch: %v", err)
		}

		reactors[i].SetEventSwitch(eventSwitch)
		if subscribeEventRespond {
			eventChans[i] = subscribeToEventRespond(eventSwitch, "tester", types.EventStringNewBlock())
		} else {
			eventChans[i] = subscribeToEvent(eventSwitch, "tester", types.EventStringNewBlock(), 1)
		}
	}
	// make connected switches and start all reactors
	p2p.MakeConnectedSwitches(config.P2P, N, func(i int, s *p2p.Switch) *p2p.Switch {
		s.AddReactor("CONSENSUS", reactors[i])
		return s
	}, p2p.Connect2Switches)

	// now that everyone is connected,  start the state machines
	// If we started the state machines before everyone was connected,
	// we'd block when the cs fires NewBlockEvent and the peers are trying to start their reactors
	for i := 0; i < N; i++ {
		s := reactors[i].conS.GetState()
		reactors[i].SwitchToConsensus(s)
	}
	return reactors, eventChans
}

func stopConsensusNet(reactors []*ConsensusReactor) {
	for _, r := range reactors {
		r.Switch.Stop()
	}
}

// Ensure a testnet makes blocks
func TestReactor(t *testing.T) {
	N := 4
	css := randConsensusNet(N, "consensus_reactor_test", newMockTickerFunc(true), newCounter)
	reactors, eventChans := startConsensusNet(t, css, N, false)
	defer stopConsensusNet(reactors)
	// wait till everyone makes the first new block
	timeoutWaitGroup(t, N, func(wg *sync.WaitGroup, j int) {
		<-eventChans[j]
		wg.Done()
	}, css)
}

//-------------------------------------------------------------
// ensure we can make blocks despite cycling a validator set

func TestVotingPowerChange(t *testing.T) {
	nVals := 4
	css := randConsensusNet(nVals, "consensus_voting_power_changes_test", newMockTickerFunc(true), newPersistentDummy)
	reactors, eventChans := startConsensusNet(t, css, nVals, true)
	defer stopConsensusNet(reactors)

	// map of active validators
	activeVals := make(map[string]struct{})
	for i := 0; i < nVals; i++ {
		activeVals[string(css[i].privValidator.GetAddress())] = struct{}{}
	}

	// wait till everyone makes block 1
	timeoutWaitGroup(t, nVals, func(wg *sync.WaitGroup, j int) {
		<-eventChans[j]
		eventChans[j] <- struct{}{}
		wg.Done()
	}, css)

	//---------------------------------------------------------------------------
	t.Log("---------------------------- Testing changing the voting power of one validator a few times")

	val1PubKey := css[0].privValidator.(*types.PrivValidator).PubKey
	updateValidatorTx := dummy.MakeValSetChangeTx(val1PubKey.Bytes(), 25)
	previousTotalVotingPower := css[0].GetRoundState().LastValidators.TotalVotingPower()

	waitForAndValidateBlock(t, nVals, activeVals, eventChans, css, updateValidatorTx)
	waitForAndValidateBlock(t, nVals, activeVals, eventChans, css)
	waitForAndValidateBlock(t, nVals, activeVals, eventChans, css)
	waitForAndValidateBlock(t, nVals, activeVals, eventChans, css)

	if css[0].GetRoundState().LastValidators.TotalVotingPower() == previousTotalVotingPower {
		t.Fatalf("expected voting power to change (before: %d, after: %d)", previousTotalVotingPower, css[0].GetRoundState().LastValidators.TotalVotingPower())
	}

	updateValidatorTx = dummy.MakeValSetChangeTx(val1PubKey.Bytes(), 2)
	previousTotalVotingPower = css[0].GetRoundState().LastValidators.TotalVotingPower()

	waitForAndValidateBlock(t, nVals, activeVals, eventChans, css, updateValidatorTx)
	waitForAndValidateBlock(t, nVals, activeVals, eventChans, css)
	waitForAndValidateBlock(t, nVals, activeVals, eventChans, css)
	waitForAndValidateBlock(t, nVals, activeVals, eventChans, css)

	if css[0].GetRoundState().LastValidators.TotalVotingPower() == previousTotalVotingPower {
		t.Fatalf("expected voting power to change (before: %d, after: %d)", previousTotalVotingPower, css[0].GetRoundState().LastValidators.TotalVotingPower())
	}

	updateValidatorTx = dummy.MakeValSetChangeTx(val1PubKey.Bytes(), 100)
	previousTotalVotingPower = css[0].GetRoundState().LastValidators.TotalVotingPower()

	waitForAndValidateBlock(t, nVals, activeVals, eventChans, css, updateValidatorTx)
	waitForAndValidateBlock(t, nVals, activeVals, eventChans, css)
	waitForAndValidateBlock(t, nVals, activeVals, eventChans, css)
	waitForAndValidateBlock(t, nVals, activeVals, eventChans, css)

	if css[0].GetRoundState().LastValidators.TotalVotingPower() == previousTotalVotingPower {
		t.Fatalf("expected voting power to change (before: %d, after: %d)", previousTotalVotingPower, css[0].GetRoundState().LastValidators.TotalVotingPower())
	}
}

func TestValidatorSetChanges(t *testing.T) {
	nPeers := 7
	nVals := 4
	css := randConsensusNetWithPeers(nVals, nPeers, "consensus_val_set_changes_test", newMockTickerFunc(true), newPersistentDummy)
	reactors, eventChans := startConsensusNet(t, css, nPeers, true)
	defer stopConsensusNet(reactors)

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
	}, css)

	//---------------------------------------------------------------------------
	t.Log("---------------------------- Testing adding one validator")

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
	t.Log("---------------------------- Testing changing the voting power of one validator")

	updateValidatorPubKey1 := css[nVals].privValidator.(*types.PrivValidator).PubKey
	updateValidatorTx1 := dummy.MakeValSetChangeTx(updateValidatorPubKey1.Bytes(), 25)
	previousTotalVotingPower := css[nVals].GetRoundState().LastValidators.TotalVotingPower()

	waitForAndValidateBlock(t, nPeers, activeVals, eventChans, css, updateValidatorTx1)
	waitForAndValidateBlock(t, nPeers, activeVals, eventChans, css)
	waitForAndValidateBlock(t, nPeers, activeVals, eventChans, css)
	waitForAndValidateBlock(t, nPeers, activeVals, eventChans, css)

	if css[nVals].GetRoundState().LastValidators.TotalVotingPower() == previousTotalVotingPower {
		t.Errorf("expected voting power to change (before: %d, after: %d)", previousTotalVotingPower, css[nVals].GetRoundState().LastValidators.TotalVotingPower())
	}

	//---------------------------------------------------------------------------
	t.Log("---------------------------- Testing adding two validators at once")

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
	t.Log("---------------------------- Testing removing two validators at once")

	removeValidatorTx2 := dummy.MakeValSetChangeTx(newValidatorPubKey2.Bytes(), 0)
	removeValidatorTx3 := dummy.MakeValSetChangeTx(newValidatorPubKey3.Bytes(), 0)

	waitForAndValidateBlock(t, nPeers, activeVals, eventChans, css, removeValidatorTx2, removeValidatorTx3)
	waitForAndValidateBlock(t, nPeers, activeVals, eventChans, css)
	waitForAndValidateBlock(t, nPeers, activeVals, eventChans, css)
	delete(activeVals, string(newValidatorPubKey2.Address()))
	delete(activeVals, string(newValidatorPubKey3.Address()))
	waitForAndValidateBlock(t, nPeers, activeVals, eventChans, css)
}

// Check we can make blocks with skip_timeout_commit=false
func TestReactorWithTimeoutCommit(t *testing.T) {
	N := 4
	css := randConsensusNet(N, "consensus_reactor_with_timeout_commit_test", newMockTickerFunc(false), newCounter)
	// override default SkipTimeoutCommit == true for tests
	for i := 0; i < N; i++ {
		css[i].config.SkipTimeoutCommit = false
	}

	reactors, eventChans := startConsensusNet(t, css, N-1, false)
	defer stopConsensusNet(reactors)

	// wait till everyone makes the first new block
	timeoutWaitGroup(t, N-1, func(wg *sync.WaitGroup, j int) {
		<-eventChans[j]
		wg.Done()
	}, css)
}

func waitForAndValidateBlock(t *testing.T, n int, activeVals map[string]struct{}, eventChans []chan interface{}, css []*ConsensusState, txs ...[]byte) {
	timeoutWaitGroup(t, n, func(wg *sync.WaitGroup, j int) {
		newBlockI := <-eventChans[j]
		newBlock := newBlockI.(types.TMEventData).Unwrap().(types.EventDataNewBlock).Block
		t.Logf("[WARN] Got block height=%v validator=%v", newBlock.Height, j)
		err := validateBlock(newBlock, activeVals)
		if err != nil {
			t.Fatal(err)
		}
		for _, tx := range txs {
			css[j].mempool.CheckTx(tx, nil)
		}

		eventChans[j] <- struct{}{}
		wg.Done()
	}, css)
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

func timeoutWaitGroup(t *testing.T, n int, f func(*sync.WaitGroup, int), css []*ConsensusState) {
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
	case <-time.After(time.Second * 10):
		for i, cs := range css {
			fmt.Println("#################")
			fmt.Println("Validator", i)
			fmt.Println(cs.GetRoundState())
			fmt.Println("")
		}
		panic("Timed out waiting for all validators to commit a block")
	}
}
