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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/abci/example/counter"
	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/types"
)

func TestValidProposalChainLocks(t *testing.T) {
	const (
		nVals  = 4
		nPeers = nVals
	)

	conf := configSetup(t)
	states, _, _, cleanup := randConsensusNetWithPeers(
		conf,
		nVals,
		nPeers,
		"consensus_chainlocks_test",
		newMockTickerFunc(true),
		newCounterWithCoreChainLocks,
	)
	t.Cleanup(cleanup)

	for i := 0; i < nVals; i++ {
		ticker := NewTimeoutTicker()
		ticker.SetLogger(states[i].Logger)
		states[i].SetTimeoutTicker(ticker)
	}

	rts := setupReactor(t, nVals, states, 40)

	for i := 0; i < 3; i++ {
		timeoutWaitGroup(t, rts.subs, states, func(sub types.Subscription) {
			msg := <-sub.Out()
			block := msg.Data().(types.EventDataNewBlock).Block
			// this is true just because of this test where each new height has a new chain lock that is incremented by 1
			state := states[0].GetState()
			assert.EqualValues(t, i+1, block.Header.CoreChainLockedHeight)           //nolint:scopelint
			assert.EqualValues(t, state.InitialHeight+int64(i), block.Header.Height) //nolint:scopelint
		})
	}
}

// one byz val sends a proposal for a height 1 less than it should, but then sends the correct block after it
func TestReactorInvalidProposalHeightForChainLocks(t *testing.T) {
	const (
		nVals  = 4
		nPeers = nVals
	)

	conf := configSetup(t)
	states, _, _, cleanup := randConsensusNetWithPeers(
		conf,
		nVals,
		nPeers,
		"consensus_chainlocks_test",
		newMockTickerFunc(true),
		newCounterWithCoreChainLocks,
	)
	t.Cleanup(cleanup)

	for i := 0; i < nVals; i++ {
		ticker := NewTimeoutTicker()
		ticker.SetLogger(states[i].Logger)
		states[i].SetTimeoutTicker(ticker)
	}

	// this proposer sends a chain lock at each height
	byzProposerID := 0
	byzProposer := states[byzProposerID]

	// update the decide proposal to propose the incorrect height
	byzProposer.decideProposal = func(j int32) func(int64, int32) {
		return func(height int64, round int32) {
			invalidProposeCoreChainLockFunc(height, round, states[j])
		}
	}(int32(0))

	rts := setupReactor(t, nVals, states, 40)

	for i := 0; i < 3; i++ {
		timeoutWaitGroup(t, rts.subs, states, func(sub types.Subscription) {
			msg := <-sub.Out()
			block := msg.Data().(types.EventDataNewBlock).Block
			// this is true just because of this test where each new height has a new chain lock that is incremented by 1
			state := states[0].GetState()
			assert.EqualValues(t, i+1, block.Header.CoreChainLockedHeight)           //nolint:scopelint
			assert.EqualValues(t, state.InitialHeight+int64(i), block.Header.Height) //nolint:scopelint
		})
	}
}

func TestReactorInvalidBlockChainLock(t *testing.T) {
	// TODO: Leads to race, explore
	const (
		nVals  = 4
		nPeers = nVals
	)

	conf := configSetup(t)
	states, _, _, cleanup := randConsensusNetWithPeers(
		conf,
		nVals,
		nPeers,
		"consensus_chainlocks_test",
		newMockTickerFunc(true),
		newCounterWithBackwardsCoreChainLocks,
	)
	t.Cleanup(cleanup)

	for i := 0; i < nVals; i++ {
		ticker := NewTimeoutTicker()
		ticker.SetLogger(states[i].Logger)
		states[i].SetTimeoutTicker(ticker)
	}

	rts := setupReactor(t, nVals, states, 100)

	for i := 0; i < 10; i++ {
		timeoutWaitGroup(t, rts.subs, states, func(sub types.Subscription) {
			msg := <-sub.Out()
			block := msg.Data().(types.EventDataNewBlock).Block
			// this is true just because of this test where each new height has a new chain lock that is incremented by 1
			state := states[0].GetState()
			expected := 99
			if block.Header.Height == state.InitialHeight {
				expected = 1
			}
			// We started at 1 then 99, then try 98, 97, 96...
			// The chain lock should stay on 99
			assert.EqualValues(t, expected, block.Header.CoreChainLockedHeight)
		})
	}
}

func newCounterWithCoreChainLocks(_ string) abci.Application {
	counterApp := counter.NewApplication(true)
	counterApp.HasCoreChainLocks = true
	counterApp.CurrentCoreChainLockHeight = 1
	return counterApp
}

func newCounterWithBackwardsCoreChainLocks(_ string) abci.Application {
	counterApp := counter.NewApplication(true)
	counterApp.HasCoreChainLocks = true
	counterApp.CurrentCoreChainLockHeight = 100
	counterApp.CoreChainLockStep = -1
	return counterApp
}

func setupReactor(t *testing.T, n int, states []*State, size int) *reactorTestSuite {
	t.Helper()
	rts := setup(t, n, states, size)
	for _, reactor := range rts.reactors {
		state := reactor.state.GetState()
		reactor.SwitchToConsensus(state, false)
	}
	return rts
}

func invalidProposeCoreChainLockFunc(height int64, round int32, cs *State) {
	// routine to:
	// - precommit for a random block
	// - send precommit to all peers
	// - disable privValidator (so we don't do normal precommits)

	var block *types.Block
	var blockParts *types.PartSet

	// Decide on block
	if cs.ValidBlock != nil {
		// If there is valid block, choose that.
		block, blockParts = cs.ValidBlock, cs.ValidBlockParts
	} else {
		// Create a new proposal block from state/txs from the mempool.
		block, blockParts = cs.createProposalBlock()
		if block == nil {
			return
		}
	}

	// Flush the WAL. Otherwise, we may not recompute the same proposal to sign,
	// and the privValidator will refuse to sign anything.
	if err := cs.wal.FlushAndSync(); err != nil {
		cs.Logger.Error("Error flushing to disk")
	}

	// Make proposal
	propBlockID := types.BlockID{Hash: block.Hash(), PartSetHeader: blockParts.Header()}
	// It is byzantine because it is not updating the LastCoreChainLockedBlockHeight
	proposal := types.NewProposal(height, cs.state.LastCoreChainLockedBlockHeight, round, cs.ValidRound, propBlockID)
	p := proposal.ToProto()
	_, err := cs.privValidator.SignProposal(
		context.Background(),
		cs.state.ChainID,
		cs.Validators.QuorumType,
		cs.Validators.QuorumHash,
		p,
	)
	if err == nil {
		proposal.Signature = p.Signature

		// send proposal and block parts on internal msg queue
		cs.sendInternalMessage(msgInfo{&ProposalMessage{proposal}, ""})
		for i := 0; i < int(blockParts.Total()); i++ {
			part := blockParts.GetPart(i)
			cs.sendInternalMessage(msgInfo{&BlockPartMessage{cs.Height, cs.Round, part}, ""})
		}
		cs.Logger.Info("Signed proposal", "height", height, "round", round, "proposal", proposal)
		cs.Logger.Debug(fmt.Sprintf("Signed proposal block: %v", block))
	} else if !cs.replayMode {
		cs.Logger.Error("enterPropose: Error signing proposal", "height", height, "round", round, "err", err)
	}
}

func timeoutWaitGroup(
	t *testing.T,
	subs map[types.NodeID]types.Subscription,
	states []*State,
	f func(types.Subscription),
) {
	var wg sync.WaitGroup
	wg.Add(len(subs))

	for _, sub := range subs {
		go func(sub types.Subscription) {
			f(sub)
			wg.Done()
		}(sub)
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	// we're running many nodes in-process, possibly in a virtual machine,
	// and spewing debug messages - making a block could take a while,
	timeout := time.Second * 20

	select {
	case <-done:
	case <-time.After(timeout):
		for i, state := range states {
			t.Log("#################")
			t.Log("Validator", i)
			t.Log(state.GetRoundState())
			t.Log("")
		}
		os.Stdout.Write([]byte("pprof.Lookup('goroutine'):\n"))
		err := pprof.Lookup("goroutine").WriteTo(os.Stdout, 1)
		require.NoError(t, err)
		capture()
		t.Fatal("Timed out waiting for all validators to commit a block")
	}
}

func capture() {
	trace := make([]byte, 10240000)
	count := runtime.Stack(trace, true)
	fmt.Printf("Stack of %d bytes: %s\n", count, trace)
}
