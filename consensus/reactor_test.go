package consensus

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"runtime/pprof"
	"sync"
	"testing"
	"time"

	"github.com/tendermint/abci/example/kvstore"
	cmn "github.com/tendermint/tmlibs/common"
	"github.com/tendermint/tmlibs/log"

	cfg "github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/p2p"
	p2pdummy "github.com/tendermint/tendermint/p2p/dummy"
	"github.com/tendermint/tendermint/types"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	config = ResetConfig("consensus_reactor_test")
}

//----------------------------------------------
// in-process testnets

func startConsensusNet(t *testing.T, css []*ConsensusState, N int) ([]*ConsensusReactor, []chan interface{}, []*types.EventBus) {
	reactors := make([]*ConsensusReactor, N)
	eventChans := make([]chan interface{}, N)
	eventBuses := make([]*types.EventBus, N)
	for i := 0; i < N; i++ {
		/*logger, err := tmflags.ParseLogLevel("consensus:info,*:error", logger, "info")
		if err != nil {	t.Fatal(err)}*/
		reactors[i] = NewConsensusReactor(css[i], true) // so we dont start the consensus states
		reactors[i].SetLogger(css[i].Logger)

		// eventBus is already started with the cs
		eventBuses[i] = css[i].eventBus
		reactors[i].SetEventBus(eventBuses[i])

		eventChans[i] = make(chan interface{}, 1)
		err := eventBuses[i].Subscribe(context.Background(), testSubscriber, types.EventQueryNewBlock, eventChans[i])
		require.NoError(t, err)
	}
	// make connected switches and start all reactors
	p2p.MakeConnectedSwitches(t, config.P2P, N, func(i int, s *p2p.Switch) *p2p.Switch {
		s.AddReactor("CONSENSUS", reactors[i])
		s.SetLogger(reactors[i].conS.Logger.With("module", "p2p"))
		return s
	}, p2p.Connect2Switches)

	// now that everyone is connected,  start the state machines
	// If we started the state machines before everyone was connected,
	// we'd block when the cs fires NewBlockEvent and the peers are trying to start their reactors
	// TODO: is this still true with new pubsub?
	for i := 0; i < N; i++ {
		s := reactors[i].conS.GetState()
		reactors[i].SwitchToConsensus(s, 0)
	}
	return reactors, eventChans, eventBuses
}

func stopConsensusNet(logger log.Logger, reactors []*ConsensusReactor, eventBuses []*types.EventBus) {
	logger.Info("stopConsensusNet", "n", len(reactors))
	for i, r := range reactors {
		logger.Info("stopConsensusNet: Stopping ConsensusReactor", "i", i)
		r.Switch.Stop()
	}
	for i, b := range eventBuses {
		logger.Info("stopConsensusNet: Stopping eventBus", "i", i)
		b.Stop()
	}
	logger.Info("stopConsensusNet: DONE", "n", len(reactors))
}

// Ensure a testnet makes blocks
func TestReactorBasic(t *testing.T) {
	N := 4
	css := randConsensusNet(N, "consensus_reactor_test", newMockTickerFunc(true), newCounter)
	reactors, eventChans, eventBuses := startConsensusNet(t, css, N)
	defer stopConsensusNet(log.TestingLogger(), reactors, eventBuses)
	// wait till everyone makes the first new block
	timeoutWaitGroup(t, N, func(j int) {
		<-eventChans[j]
	}, css)
}

// Ensure a testnet sends proposal heartbeats and makes blocks when there are txs
func TestReactorProposalHeartbeats(t *testing.T) {
	N := 4
	css := randConsensusNet(N, "consensus_reactor_test", newMockTickerFunc(true), newCounter,
		func(c *cfg.Config) {
			c.Consensus.CreateEmptyBlocks = false
		})
	reactors, eventChans, eventBuses := startConsensusNet(t, css, N)
	defer stopConsensusNet(log.TestingLogger(), reactors, eventBuses)
	heartbeatChans := make([]chan interface{}, N)
	var err error
	for i := 0; i < N; i++ {
		heartbeatChans[i] = make(chan interface{}, 1)
		err = eventBuses[i].Subscribe(context.Background(), testSubscriber, types.EventQueryProposalHeartbeat, heartbeatChans[i])
		require.NoError(t, err)
	}
	// wait till everyone sends a proposal heartbeat
	timeoutWaitGroup(t, N, func(j int) {
		<-heartbeatChans[j]
	}, css)

	// send a tx
	if err := css[3].mempool.CheckTx([]byte{1, 2, 3}, nil); err != nil {
		//t.Fatal(err)
	}

	// wait till everyone makes the first new block
	timeoutWaitGroup(t, N, func(j int) {
		<-eventChans[j]
	}, css)
}

// Test we record block parts from other peers
func TestReactorRecordsBlockParts(t *testing.T) {
	// create dummy peer
	peer := p2pdummy.NewPeer()
	ps := NewPeerState(peer).SetLogger(log.TestingLogger())
	peer.Set(types.PeerStateKey, ps)

	// create reactor
	css := randConsensusNet(1, "consensus_reactor_records_block_parts_test", newMockTickerFunc(true), newPersistentKVStore)
	reactor := NewConsensusReactor(css[0], false) // so we dont start the consensus states
	reactor.SetEventBus(css[0].eventBus)
	reactor.SetLogger(log.TestingLogger())
	sw := p2p.MakeSwitch(cfg.DefaultP2PConfig(), 1, "testing", "123.123.123", func(i int, sw *p2p.Switch) *p2p.Switch { return sw })
	reactor.SetSwitch(sw)
	err := reactor.Start()
	require.NoError(t, err)
	defer reactor.Stop()

	// 1) new block part
	parts := types.NewPartSetFromData(cmn.RandBytes(100), 10)
	msg := &BlockPartMessage{
		Height: 2,
		Round:  0,
		Part:   parts.GetPart(0),
	}
	bz, err := cdc.MarshalBinaryBare(msg)
	require.NoError(t, err)

	reactor.Receive(DataChannel, peer, bz)
	require.Equal(t, 1, ps.BlockPartsSent(), "number of block parts sent should have increased by 1")

	// 2) block part with the same height, but different round
	msg.Round = 1

	bz, err = cdc.MarshalBinaryBare(msg)
	require.NoError(t, err)

	reactor.Receive(DataChannel, peer, bz)
	require.Equal(t, 1, ps.BlockPartsSent(), "number of block parts sent should stay the same")

	// 3) block part from earlier height
	msg.Height = 1
	msg.Round = 0

	bz, err = cdc.MarshalBinaryBare(msg)
	require.NoError(t, err)

	reactor.Receive(DataChannel, peer, bz)
	require.Equal(t, 1, ps.BlockPartsSent(), "number of block parts sent should stay the same")
}

// Test we record votes from other peers
func TestReactorRecordsVotes(t *testing.T) {
	// create dummy peer
	peer := p2pdummy.NewPeer()
	ps := NewPeerState(peer).SetLogger(log.TestingLogger())
	peer.Set(types.PeerStateKey, ps)

	// create reactor
	css := randConsensusNet(1, "consensus_reactor_records_votes_test", newMockTickerFunc(true), newPersistentKVStore)
	reactor := NewConsensusReactor(css[0], false) // so we dont start the consensus states
	reactor.SetEventBus(css[0].eventBus)
	reactor.SetLogger(log.TestingLogger())
	sw := p2p.MakeSwitch(cfg.DefaultP2PConfig(), 1, "testing", "123.123.123", func(i int, sw *p2p.Switch) *p2p.Switch { return sw })
	reactor.SetSwitch(sw)
	err := reactor.Start()
	require.NoError(t, err)
	defer reactor.Stop()
	_, val := css[0].state.Validators.GetByIndex(0)

	// 1) new vote
	vote := &types.Vote{
		ValidatorIndex:   0,
		ValidatorAddress: val.Address,
		Height:           2,
		Round:            0,
		Timestamp:        time.Now().UTC(),
		Type:             types.VoteTypePrevote,
		BlockID:          types.BlockID{},
	}
	bz, err := cdc.MarshalBinaryBare(&VoteMessage{vote})
	require.NoError(t, err)

	reactor.Receive(VoteChannel, peer, bz)
	assert.Equal(t, 1, ps.VotesSent(), "number of votes sent should have increased by 1")

	// 2) vote with the same height, but different round
	vote.Round = 1

	bz, err = cdc.MarshalBinaryBare(&VoteMessage{vote})
	require.NoError(t, err)

	reactor.Receive(VoteChannel, peer, bz)
	assert.Equal(t, 1, ps.VotesSent(), "number of votes sent should stay the same")

	// 3) vote from earlier height
	vote.Height = 1
	vote.Round = 0

	bz, err = cdc.MarshalBinaryBare(&VoteMessage{vote})
	require.NoError(t, err)

	reactor.Receive(VoteChannel, peer, bz)
	assert.Equal(t, 1, ps.VotesSent(), "number of votes sent should stay the same")
}

//-------------------------------------------------------------
// ensure we can make blocks despite cycling a validator set

func TestReactorVotingPowerChange(t *testing.T) {
	nVals := 4
	logger := log.TestingLogger()
	css := randConsensusNet(nVals, "consensus_voting_power_changes_test", newMockTickerFunc(true), newPersistentKVStore)
	reactors, eventChans, eventBuses := startConsensusNet(t, css, nVals)
	defer stopConsensusNet(logger, reactors, eventBuses)

	// map of active validators
	activeVals := make(map[string]struct{})
	for i := 0; i < nVals; i++ {
		activeVals[string(css[i].privValidator.GetAddress())] = struct{}{}
	}

	// wait till everyone makes block 1
	timeoutWaitGroup(t, nVals, func(j int) {
		<-eventChans[j]
	}, css)

	//---------------------------------------------------------------------------
	logger.Debug("---------------------------- Testing changing the voting power of one validator a few times")

	val1PubKey := css[0].privValidator.GetPubKey()
	val1PubKeyABCI := types.TM2PB.PubKey(val1PubKey)
	updateValidatorTx := kvstore.MakeValSetChangeTx(val1PubKeyABCI, 25)
	previousTotalVotingPower := css[0].GetRoundState().LastValidators.TotalVotingPower()

	waitForAndValidateBlock(t, nVals, activeVals, eventChans, css, updateValidatorTx)
	waitForAndValidateBlockWithTx(t, nVals, activeVals, eventChans, css, updateValidatorTx)
	waitForAndValidateBlock(t, nVals, activeVals, eventChans, css)
	waitForAndValidateBlock(t, nVals, activeVals, eventChans, css)

	if css[0].GetRoundState().LastValidators.TotalVotingPower() == previousTotalVotingPower {
		t.Fatalf("expected voting power to change (before: %d, after: %d)", previousTotalVotingPower, css[0].GetRoundState().LastValidators.TotalVotingPower())
	}

	updateValidatorTx = kvstore.MakeValSetChangeTx(val1PubKeyABCI, 2)
	previousTotalVotingPower = css[0].GetRoundState().LastValidators.TotalVotingPower()

	waitForAndValidateBlock(t, nVals, activeVals, eventChans, css, updateValidatorTx)
	waitForAndValidateBlockWithTx(t, nVals, activeVals, eventChans, css, updateValidatorTx)
	waitForAndValidateBlock(t, nVals, activeVals, eventChans, css)
	waitForAndValidateBlock(t, nVals, activeVals, eventChans, css)

	if css[0].GetRoundState().LastValidators.TotalVotingPower() == previousTotalVotingPower {
		t.Fatalf("expected voting power to change (before: %d, after: %d)", previousTotalVotingPower, css[0].GetRoundState().LastValidators.TotalVotingPower())
	}

	updateValidatorTx = kvstore.MakeValSetChangeTx(val1PubKeyABCI, 26)
	previousTotalVotingPower = css[0].GetRoundState().LastValidators.TotalVotingPower()

	waitForAndValidateBlock(t, nVals, activeVals, eventChans, css, updateValidatorTx)
	waitForAndValidateBlockWithTx(t, nVals, activeVals, eventChans, css, updateValidatorTx)
	waitForAndValidateBlock(t, nVals, activeVals, eventChans, css)
	waitForAndValidateBlock(t, nVals, activeVals, eventChans, css)

	if css[0].GetRoundState().LastValidators.TotalVotingPower() == previousTotalVotingPower {
		t.Fatalf("expected voting power to change (before: %d, after: %d)", previousTotalVotingPower, css[0].GetRoundState().LastValidators.TotalVotingPower())
	}
}

func TestReactorValidatorSetChanges(t *testing.T) {
	nPeers := 7
	nVals := 4
	css := randConsensusNetWithPeers(nVals, nPeers, "consensus_val_set_changes_test", newMockTickerFunc(true), newPersistentKVStore)

	logger := log.TestingLogger()

	reactors, eventChans, eventBuses := startConsensusNet(t, css, nPeers)
	defer stopConsensusNet(logger, reactors, eventBuses)

	// map of active validators
	activeVals := make(map[string]struct{})
	for i := 0; i < nVals; i++ {
		activeVals[string(css[i].privValidator.GetAddress())] = struct{}{}
	}

	// wait till everyone makes block 1
	timeoutWaitGroup(t, nPeers, func(j int) {
		<-eventChans[j]
	}, css)

	//---------------------------------------------------------------------------
	logger.Info("---------------------------- Testing adding one validator")

	newValidatorPubKey1 := css[nVals].privValidator.GetPubKey()
	valPubKey1ABCI := types.TM2PB.PubKey(newValidatorPubKey1)
	newValidatorTx1 := kvstore.MakeValSetChangeTx(valPubKey1ABCI, testMinPower)

	// wait till everyone makes block 2
	// ensure the commit includes all validators
	// send newValTx to change vals in block 3
	waitForAndValidateBlock(t, nPeers, activeVals, eventChans, css, newValidatorTx1)

	// wait till everyone makes block 3.
	// it includes the commit for block 2, which is by the original validator set
	waitForAndValidateBlockWithTx(t, nPeers, activeVals, eventChans, css, newValidatorTx1)

	// wait till everyone makes block 4.
	// it includes the commit for block 3, which is by the original validator set
	waitForAndValidateBlock(t, nPeers, activeVals, eventChans, css)

	// the commits for block 4 should be with the updated validator set
	activeVals[string(newValidatorPubKey1.Address())] = struct{}{}

	// wait till everyone makes block 5
	// it includes the commit for block 4, which should have the updated validator set
	waitForBlockWithUpdatedValsAndValidateIt(t, nPeers, activeVals, eventChans, css)

	//---------------------------------------------------------------------------
	logger.Info("---------------------------- Testing changing the voting power of one validator")

	updateValidatorPubKey1 := css[nVals].privValidator.GetPubKey()
	updatePubKey1ABCI := types.TM2PB.PubKey(updateValidatorPubKey1)
	updateValidatorTx1 := kvstore.MakeValSetChangeTx(updatePubKey1ABCI, 25)
	previousTotalVotingPower := css[nVals].GetRoundState().LastValidators.TotalVotingPower()

	waitForAndValidateBlock(t, nPeers, activeVals, eventChans, css, updateValidatorTx1)
	waitForAndValidateBlockWithTx(t, nPeers, activeVals, eventChans, css, updateValidatorTx1)
	waitForAndValidateBlock(t, nPeers, activeVals, eventChans, css)
	waitForBlockWithUpdatedValsAndValidateIt(t, nPeers, activeVals, eventChans, css)

	if css[nVals].GetRoundState().LastValidators.TotalVotingPower() == previousTotalVotingPower {
		t.Errorf("expected voting power to change (before: %d, after: %d)", previousTotalVotingPower, css[nVals].GetRoundState().LastValidators.TotalVotingPower())
	}

	//---------------------------------------------------------------------------
	logger.Info("---------------------------- Testing adding two validators at once")

	newValidatorPubKey2 := css[nVals+1].privValidator.GetPubKey()
	newVal2ABCI := types.TM2PB.PubKey(newValidatorPubKey2)
	newValidatorTx2 := kvstore.MakeValSetChangeTx(newVal2ABCI, testMinPower)

	newValidatorPubKey3 := css[nVals+2].privValidator.GetPubKey()
	newVal3ABCI := types.TM2PB.PubKey(newValidatorPubKey3)
	newValidatorTx3 := kvstore.MakeValSetChangeTx(newVal3ABCI, testMinPower)

	waitForAndValidateBlock(t, nPeers, activeVals, eventChans, css, newValidatorTx2, newValidatorTx3)
	waitForAndValidateBlockWithTx(t, nPeers, activeVals, eventChans, css, newValidatorTx2, newValidatorTx3)
	waitForAndValidateBlock(t, nPeers, activeVals, eventChans, css)
	activeVals[string(newValidatorPubKey2.Address())] = struct{}{}
	activeVals[string(newValidatorPubKey3.Address())] = struct{}{}
	waitForBlockWithUpdatedValsAndValidateIt(t, nPeers, activeVals, eventChans, css)

	//---------------------------------------------------------------------------
	logger.Info("---------------------------- Testing removing two validators at once")

	removeValidatorTx2 := kvstore.MakeValSetChangeTx(newVal2ABCI, 0)
	removeValidatorTx3 := kvstore.MakeValSetChangeTx(newVal3ABCI, 0)

	waitForAndValidateBlock(t, nPeers, activeVals, eventChans, css, removeValidatorTx2, removeValidatorTx3)
	waitForAndValidateBlockWithTx(t, nPeers, activeVals, eventChans, css, removeValidatorTx2, removeValidatorTx3)
	waitForAndValidateBlock(t, nPeers, activeVals, eventChans, css)
	delete(activeVals, string(newValidatorPubKey2.Address()))
	delete(activeVals, string(newValidatorPubKey3.Address()))
	waitForBlockWithUpdatedValsAndValidateIt(t, nPeers, activeVals, eventChans, css)
}

// Check we can make blocks with skip_timeout_commit=false
func TestReactorWithTimeoutCommit(t *testing.T) {
	N := 4
	css := randConsensusNet(N, "consensus_reactor_with_timeout_commit_test", newMockTickerFunc(false), newCounter)
	// override default SkipTimeoutCommit == true for tests
	for i := 0; i < N; i++ {
		css[i].config.SkipTimeoutCommit = false
	}

	reactors, eventChans, eventBuses := startConsensusNet(t, css, N-1)
	defer stopConsensusNet(log.TestingLogger(), reactors, eventBuses)

	// wait till everyone makes the first new block
	timeoutWaitGroup(t, N-1, func(j int) {
		<-eventChans[j]
	}, css)
}

func waitForAndValidateBlock(t *testing.T, n int, activeVals map[string]struct{}, eventChans []chan interface{}, css []*ConsensusState, txs ...[]byte) {
	timeoutWaitGroup(t, n, func(j int) {
		css[j].Logger.Debug("waitForAndValidateBlock")
		newBlockI, ok := <-eventChans[j]
		if !ok {
			return
		}
		newBlock := newBlockI.(types.EventDataNewBlock).Block
		css[j].Logger.Debug("waitForAndValidateBlock: Got block", "height", newBlock.Height)
		err := validateBlock(newBlock, activeVals)
		assert.Nil(t, err)
		for _, tx := range txs {
			css[j].mempool.CheckTx(tx, nil)
			assert.Nil(t, err)
		}
	}, css)
}

func waitForAndValidateBlockWithTx(t *testing.T, n int, activeVals map[string]struct{}, eventChans []chan interface{}, css []*ConsensusState, txs ...[]byte) {
	timeoutWaitGroup(t, n, func(j int) {
		ntxs := 0
	BLOCK_TX_LOOP:
		for {
			css[j].Logger.Debug("waitForAndValidateBlockWithTx", "ntxs", ntxs)
			newBlockI, ok := <-eventChans[j]
			if !ok {
				return
			}
			newBlock := newBlockI.(types.EventDataNewBlock).Block
			css[j].Logger.Debug("waitForAndValidateBlockWithTx: Got block", "height", newBlock.Height)
			err := validateBlock(newBlock, activeVals)
			assert.Nil(t, err)

			// check that txs match the txs we're waiting for.
			// note they could be spread over multiple blocks,
			// but they should be in order.
			for _, tx := range newBlock.Data.Txs {
				assert.EqualValues(t, txs[ntxs], tx)
				ntxs++
			}

			if ntxs == len(txs) {
				break BLOCK_TX_LOOP
			}
		}

	}, css)
}

func waitForBlockWithUpdatedValsAndValidateIt(t *testing.T, n int, updatedVals map[string]struct{}, eventChans []chan interface{}, css []*ConsensusState) {
	timeoutWaitGroup(t, n, func(j int) {

		var newBlock *types.Block
	LOOP:
		for {
			css[j].Logger.Debug("waitForBlockWithUpdatedValsAndValidateIt")
			newBlockI, ok := <-eventChans[j]
			if !ok {
				return
			}
			newBlock = newBlockI.(types.EventDataNewBlock).Block
			if newBlock.LastCommit.Size() == len(updatedVals) {
				css[j].Logger.Debug("waitForBlockWithUpdatedValsAndValidateIt: Got block", "height", newBlock.Height)
				break LOOP
			} else {
				css[j].Logger.Debug("waitForBlockWithUpdatedValsAndValidateIt: Got block with no new validators. Skipping", "height", newBlock.Height)
			}
		}

		err := validateBlock(newBlock, updatedVals)
		assert.Nil(t, err)
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

func timeoutWaitGroup(t *testing.T, n int, f func(int), css []*ConsensusState) {
	wg := new(sync.WaitGroup)
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func(j int) {
			f(j)
			wg.Done()
		}(i)
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	// we're running many nodes in-process, possibly in in a virtual machine,
	// and spewing debug messages - making a block could take a while,
	timeout := time.Second * 300

	select {
	case <-done:
	case <-time.After(timeout):
		for i, cs := range css {
			t.Log("#################")
			t.Log("Validator", i)
			t.Log(cs.GetRoundState())
			t.Log("")
		}
		os.Stdout.Write([]byte("pprof.Lookup('goroutine'):\n"))
		pprof.Lookup("goroutine").WriteTo(os.Stdout, 1)
		capture()
		panic("Timed out waiting for all validators to commit a block")
	}
}

func capture() {
	trace := make([]byte, 10240000)
	count := runtime.Stack(trace, true)
	fmt.Printf("Stack of %d bytes: %s\n", count, trace)
}
