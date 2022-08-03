package consensus

import (
	"context"
	"fmt"
	"os"
	"path"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"

	abciclient "github.com/tendermint/tendermint/abci/client"
	"github.com/tendermint/tendermint/abci/example/kvstore"
	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/internal/eventbus"
	"github.com/tendermint/tendermint/internal/evidence"
	"github.com/tendermint/tendermint/internal/mempool"
	"github.com/tendermint/tendermint/internal/p2p"
	sm "github.com/tendermint/tendermint/internal/state"
	"github.com/tendermint/tendermint/internal/store"
	"github.com/tendermint/tendermint/internal/test/factory"
	"github.com/tendermint/tendermint/libs/log"
	tmtime "github.com/tendermint/tendermint/libs/time"
	tmcons "github.com/tendermint/tendermint/proto/tendermint/consensus"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	"github.com/tendermint/tendermint/types"
)

// Byzantine node sends two different prevotes (nil and blockID) to the same
// validator.
func TestByzantinePrevoteEquivocation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode")
	}

	// empirically, this test either passes in <1s or hits some
	// kind of deadlock and hit the larger timeout. This timeout
	// can be extended a bunch if needed, but it's good to avoid
	// falling back to a much coarser timeout
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	config := configSetup(t)

	nValidators := 4
	prevoteHeight := int64(2)
	testName := "consensus_byzantine_test"
	tickerFunc := newMockTickerFunc(true)

	valSet, privVals := factory.ValidatorSet(ctx, t, nValidators, 30)
	genDoc := factory.GenesisDoc(config, time.Now(), valSet.Validators, factory.ConsensusParams())
	states := make([]*State, nValidators)

	for i := 0; i < nValidators; i++ {
		func() {
			logger := consensusLogger().With("test", "byzantine", "validator", i)
			stateDB := dbm.NewMemDB() // each state needs its own db
			stateStore := sm.NewStore(stateDB)
			state, err := sm.MakeGenesisState(genDoc)
			require.NoError(t, err)
			require.NoError(t, stateStore.Save(state))

			thisConfig, err := ResetConfig(t.TempDir(), fmt.Sprintf("%s_%d", testName, i))
			require.NoError(t, err)

			defer os.RemoveAll(thisConfig.RootDir)

			ensureDir(t, path.Dir(thisConfig.Consensus.WalFile()), 0700) // dir for wal
			app := kvstore.NewApplication()
			vals := types.TM2PB.ValidatorUpdates(state.Validators)
			_, err = app.InitChain(ctx, &abci.RequestInitChain{Validators: vals})
			require.NoError(t, err)

			blockDB := dbm.NewMemDB()
			blockStore := store.NewBlockStore(blockDB)

			// one for mempool, one for consensus
			proxyAppConnMem := abciclient.NewLocalClient(logger, app)
			proxyAppConnCon := abciclient.NewLocalClient(logger, app)

			// Make Mempool
			mempool := mempool.NewTxMempool(
				log.NewNopLogger().With("module", "mempool"),
				thisConfig.Mempool,
				proxyAppConnMem,
			)
			if thisConfig.Consensus.WaitForTxs() {
				mempool.EnableTxsAvailable()
			}

			eventBus := eventbus.NewDefault(log.NewNopLogger().With("module", "events"))
			require.NoError(t, eventBus.Start(ctx))

			// Make a full instance of the evidence pool
			evidenceDB := dbm.NewMemDB()
			evpool := evidence.NewPool(logger.With("module", "evidence"), evidenceDB, stateStore, blockStore, evidence.NopMetrics(), eventBus)

			// Make State
			blockExec := sm.NewBlockExecutor(stateStore, log.NewNopLogger(), proxyAppConnCon, mempool, evpool, blockStore, eventBus, sm.NopMetrics())
			cs, err := NewState(logger, thisConfig.Consensus, stateStore, blockExec, blockStore, mempool, evpool, eventBus)
			require.NoError(t, err)
			// set private validator
			pv := privVals[i]
			cs.SetPrivValidator(ctx, pv)

			cs.SetTimeoutTicker(tickerFunc())

			states[i] = cs
		}()
	}

	rts := setup(ctx, t, nValidators, states, 512) // buffer must be large enough to not deadlock

	var bzNodeID types.NodeID

	// Set the first state's reactor as the dedicated byzantine reactor and grab
	// the NodeID that corresponds to the state so we can reference the reactor.
	bzNodeState := states[0]
	for nID, s := range rts.states {
		if s == bzNodeState {
			bzNodeID = nID
			break
		}
	}

	bzReactor := rts.reactors[bzNodeID]

	// alter prevote so that the byzantine node double votes when height is 2
	bzNodeState.doPrevote = func(ctx context.Context, height int64, round int32) {
		// allow first height to happen normally so that byzantine validator is no longer proposer
		if height == prevoteHeight {
			prevote1, err := bzNodeState.signVote(ctx,
				tmproto.PrevoteType,
				bzNodeState.ProposalBlock.Hash(),
				bzNodeState.ProposalBlockParts.Header(),
			)
			require.NoError(t, err)

			prevote2, err := bzNodeState.signVote(ctx, tmproto.PrevoteType, nil, types.PartSetHeader{})
			require.NoError(t, err)

			// send two votes to all peers (1st to one half, 2nd to another half)
			i := 0
			for _, ps := range bzReactor.peers {
				voteCh := rts.voteChannels[bzNodeID]
				if i < len(bzReactor.peers)/2 {

					require.NoError(t, voteCh.Send(ctx,
						p2p.Envelope{
							To: ps.peerID,
							Message: &tmcons.Vote{
								Vote: prevote1.ToProto(),
							},
						}))
				} else {
					require.NoError(t, voteCh.Send(ctx,
						p2p.Envelope{
							To: ps.peerID,
							Message: &tmcons.Vote{
								Vote: prevote2.ToProto(),
							},
						}))
				}

				i++
			}
		} else {
			bzNodeState.defaultDoPrevote(ctx, height, round)
		}
	}

	// Introducing a lazy proposer means that the time of the block committed is
	// different to the timestamp that the other nodes have. This tests to ensure
	// that the evidence that finally gets proposed will have a valid timestamp.
	// lazyProposer := states[1]
	lazyNodeState := states[1]

	lazyNodeState.decideProposal = func(ctx context.Context, height int64, round int32) {
		require.NotNil(t, lazyNodeState.privValidator)

		var extCommit *types.ExtendedCommit
		switch {
		case lazyNodeState.Height == lazyNodeState.state.InitialHeight:
			// We're creating a proposal for the first block.
			// The commit is empty, but not nil.
			extCommit = &types.ExtendedCommit{}
		case lazyNodeState.LastCommit.HasTwoThirdsMajority():
			// Make the commit from LastCommit
			extCommit = lazyNodeState.LastCommit.MakeExtendedCommit()
		default: // This shouldn't happen.
			lazyNodeState.logger.Error("enterPropose: Cannot propose anything: No commit for the previous block")
			return
		}

		// omit the last signature in the commit
		extCommit.ExtendedSignatures[len(extCommit.ExtendedSignatures)-1] = types.NewExtendedCommitSigAbsent()

		if lazyNodeState.privValidatorPubKey == nil {
			// If this node is a validator & proposer in the current round, it will
			// miss the opportunity to create a block.
			lazyNodeState.logger.Error("enterPropose", "err", errPubKeyIsNotSet)
			return
		}
		proposerAddr := lazyNodeState.privValidatorPubKey.Address()

		block, err := lazyNodeState.blockExec.CreateProposalBlock(
			ctx, lazyNodeState.Height, lazyNodeState.state, extCommit, proposerAddr)
		require.NoError(t, err)
		blockParts, err := block.MakePartSet(types.BlockPartSizeBytes)
		require.NoError(t, err)

		// Flush the WAL. Otherwise, we may not recompute the same proposal to sign,
		// and the privValidator will refuse to sign anything.
		if err := lazyNodeState.wal.FlushAndSync(); err != nil {
			lazyNodeState.logger.Error("error flushing to disk")
		}

		// Make proposal
		propBlockID := types.BlockID{Hash: block.Hash(), PartSetHeader: blockParts.Header()}
		proposal := types.NewProposal(height, round, lazyNodeState.ValidRound, propBlockID, block.Header.Time)
		p := proposal.ToProto()
		if err := lazyNodeState.privValidator.SignProposal(ctx, lazyNodeState.state.ChainID, p); err == nil {
			proposal.Signature = p.Signature

			// send proposal and block parts on internal msg queue
			lazyNodeState.sendInternalMessage(ctx, msgInfo{&ProposalMessage{proposal}, "", tmtime.Now()})
			for i := 0; i < int(blockParts.Total()); i++ {
				part := blockParts.GetPart(i)
				lazyNodeState.sendInternalMessage(ctx, msgInfo{&BlockPartMessage{
					lazyNodeState.Height, lazyNodeState.Round, part,
				}, "", tmtime.Now()})
			}
		} else if !lazyNodeState.replayMode {
			lazyNodeState.logger.Error("enterPropose: Error signing proposal", "height", height, "round", round, "err", err)
		}
	}

	for _, reactor := range rts.reactors {
		reactor.SwitchToConsensus(ctx, reactor.state.GetState(), false)
	}

	// Evidence should be submitted and committed at the third height but
	// we will check the first six just in case
	evidenceFromEachValidator := make([]types.Evidence, nValidators)

	var wg sync.WaitGroup
	i := 0
	subctx, subcancel := context.WithCancel(ctx)
	defer subcancel()
	for _, sub := range rts.subs {
		wg.Add(1)

		go func(j int, s eventbus.Subscription) {
			defer wg.Done()
			for {
				if subctx.Err() != nil {
					return
				}

				msg, err := s.Next(subctx)
				if subctx.Err() != nil {
					return
				}

				if err != nil {
					t.Errorf("waiting for subscription: %v", err)
					subcancel()
					return
				}

				require.NotNil(t, msg)
				block := msg.Data().(types.EventDataNewBlock).Block
				if len(block.Evidence) != 0 {
					evidenceFromEachValidator[j] = block.Evidence[0]
					return
				}
			}
		}(i, sub)
		i++
	}

	wg.Wait()

	// don't run more assertions if we've encountered a timeout
	select {
	case <-subctx.Done():
		t.Fatal("encountered timeout")
	default:
	}

	pubkey, err := bzNodeState.privValidator.GetPubKey(ctx)
	require.NoError(t, err)

	for idx, ev := range evidenceFromEachValidator {
		require.NotNil(t, ev, idx)
		ev, ok := ev.(*types.DuplicateVoteEvidence)
		require.True(t, ok)
		assert.Equal(t, pubkey.Address(), ev.VoteA.ValidatorAddress)
		assert.Equal(t, prevoteHeight, ev.Height())
	}
}
