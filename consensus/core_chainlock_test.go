package consensus

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tendermint/tendermint/libs/log"

	"github.com/tendermint/tendermint/abci/example/counter"
	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/types"
)

func newCounterWithCoreChainLocks() abci.Application {
	counterApp := counter.NewApplication(true)
	counterApp.HasCoreChainLocks = true
	counterApp.CurrentCoreChainLockHeight = 1
	return counterApp
}

func newCounterWithBackwardsCoreChainLocks() abci.Application {
	counterApp := counter.NewApplication(true)
	counterApp.HasCoreChainLocks = true
	counterApp.CurrentCoreChainLockHeight = 100
	counterApp.CoreChainLockStep = -1
	return counterApp
}

func TestValidProposalChainLocks(t *testing.T) {
	N := 4
	css, cleanup := randConsensusNet(N, "consensus_chainlocks_test", newMockTickerFunc(true), newCounterWithCoreChainLocks)
	defer cleanup()

	for i := 0; i < 4; i++ {
		ticker := NewTimeoutTicker()
		ticker.SetLogger(css[i].Logger)
		css[i].SetTimeoutTicker(ticker)
	}

	reactors, blocksSubs, eventBuses := startConsensusNet(t, css, N)

	defer stopConsensusNet(log.TestingLogger(), reactors, eventBuses)

	for i := 0; i < 3; i++ {
		timeoutWaitGroup(t, N, func(j int) {
			msg := <-blocksSubs[j].Out()
			block := msg.Data().(types.EventDataNewBlock).Block
			// this is true just because of this test where each new height has a new chain lock that is incremented by 1
			assert.EqualValues(t, block.Header.Height, block.Header.CoreChainLockedHeight)
		}, css)
	}
}

// one byz val sends a proposal for a height 1 less than it should, but then sends the correct block after it
func TestReactorInvalidProposalHeightForChainLocks(t *testing.T) {
	N := 4
	css, cleanup := randConsensusNet(N, "consensus_chainlocks_test", newMockTickerFunc(true), newCounterWithCoreChainLocks)
	defer cleanup()

	for i := 0; i < 4; i++ {
		ticker := NewTimeoutTicker()
		ticker.SetLogger(css[i].Logger)
		css[i].SetTimeoutTicker(ticker)
	}

	reactors, blocksSubs, eventBuses := startConsensusNet(t, css, N)

	// this proposer sends a chain lock at each height
	byzProposerID := 0
	byzProposer := css[byzProposerID]

	// update the decide proposal to propose the incorrect height
	byzProposer.mtx.Lock()

	byzProposer.decideProposal = func(j int32) func(int64, int32) {
		return func(height int64, round int32) {
			invalidProposeCoreChainLockFunc(t, height, round, css[j])
		}
	}(int32(0))
	byzProposer.mtx.Unlock()

	defer stopConsensusNet(log.TestingLogger(), reactors, eventBuses)

	for i := 0; i < 3; i++ {
		timeoutWaitGroup(t, N, func(j int) {
			msg := <-blocksSubs[j].Out()
			block := msg.Data().(types.EventDataNewBlock).Block
			// this is true just because of this test where each new height has a new chain lock that is incremented by 1
			assert.EqualValues(t, block.Header.Height, block.Header.CoreChainLockedHeight)
		}, css)
	}
}

func invalidProposeCoreChainLockFunc(t *testing.T, height int64, round int32, cs *State) {
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
	if err := cs.privValidator.SignProposal(cs.state.ChainID, cs.Validators.QuorumType, cs.Validators.QuorumHash, p); err == nil {
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

func TestReactorInvalidBlockChainLock(t *testing.T) {
	N := 4
	css, cleanup := randConsensusNet(N, "consensus_chainlocks_test",
		newMockTickerFunc(true), newCounterWithBackwardsCoreChainLocks)
	defer cleanup()

	for i := 0; i < 4; i++ {
		ticker := NewTimeoutTicker()
		ticker.SetLogger(css[i].Logger)
		css[i].SetTimeoutTicker(ticker)
	}

	reactors, blocksSubs, eventBuses := startConsensusNet(t, css, N)

	defer stopConsensusNet(log.TestingLogger(), reactors, eventBuses)

	for i := 0; i < 10; i++ {
		timeoutWaitGroup(t, N, func(j int) {
			msg := <-blocksSubs[j].Out()
			block := msg.Data().(types.EventDataNewBlock).Block
			// this is true just because of this test where each new height has a new chain lock that is incremented by 1
			if block.Header.Height == 1 {
				assert.EqualValues(t, 1, block.Header.CoreChainLockedHeight)
			} else {
				// We started at 1 then 99, then try 98, 97, 96...
				// The chain lock should stay on 99
				assert.EqualValues(t, 99, block.Header.CoreChainLockedHeight)
			}

		}, css)
	}
}
