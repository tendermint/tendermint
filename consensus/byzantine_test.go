package consensus

import (
	"context"
	"fmt"
	"os"
	"path"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	abcicli "github.com/tendermint/tendermint/abci/client"
	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/evidence"
	"github.com/tendermint/tendermint/libs/log"
	tmsync "github.com/tendermint/tendermint/libs/sync"
	mempl "github.com/tendermint/tendermint/mempool"
	"github.com/tendermint/tendermint/p2p"
	tmcons "github.com/tendermint/tendermint/proto/tendermint/consensus"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	sm "github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/store"
	"github.com/tendermint/tendermint/types"
	dbm "github.com/tendermint/tm-db"
)

// Byzantine node sends two different prevotes (nil and blockID) to the same
// validator.
func TestByzantinePrevoteEquivocation(t *testing.T) {
	configSetup(t)

	nValidators := 4
	prevoteHeight := int64(2)
	testName := "consensus_byzantine_test"
	tickerFunc := newMockTickerFunc(true)
	appFunc := newCounter

	genDoc, privVals := randGenesisDoc(nValidators, false, 30)
	states := make([]*State, nValidators)

	for i := 0; i < nValidators; i++ {
		func() {
			logger := consensusLogger().With("test", "byzantine", "validator", i)
			stateDB := dbm.NewMemDB() // each state needs its own db
			stateStore := sm.NewStore(stateDB)
			state, _ := stateStore.LoadFromDBOrGenesisDoc(genDoc)

			thisConfig := ResetConfig(fmt.Sprintf("%s_%d", testName, i))
			defer os.RemoveAll(thisConfig.RootDir)

			ensureDir(path.Dir(thisConfig.Consensus.WalFile()), 0700) // dir for wal
			app := appFunc()
			vals := types.TM2PB.ValidatorUpdates(state.Validators)
			app.InitChain(abci.RequestInitChain{Validators: vals})

			blockDB := dbm.NewMemDB()
			blockStore := store.NewBlockStore(blockDB)

			// one for mempool, one for consensus
			mtx := new(tmsync.Mutex)
			proxyAppConnMem := abcicli.NewLocalClient(mtx, app)
			proxyAppConnCon := abcicli.NewLocalClient(mtx, app)

			// Make Mempool
			mempool := mempl.NewCListMempool(thisConfig.Mempool, proxyAppConnMem, 0)
			mempool.SetLogger(log.TestingLogger().With("module", "mempool"))
			if thisConfig.Consensus.WaitForTxs() {
				mempool.EnableTxsAvailable()
			}

			// Make a full instance of the evidence pool
			evidenceDB := dbm.NewMemDB()
			evpool, err := evidence.NewPool(logger.With("module", "evidence"), evidenceDB, stateStore, blockStore)
			require.NoError(t, err)

			// Make State
			blockExec := sm.NewBlockExecutor(stateStore, log.TestingLogger(), proxyAppConnCon, mempool, evpool)
			cs := NewState(thisConfig.Consensus, state, blockExec, blockStore, mempool, evpool)
			cs.SetLogger(cs.Logger)
			// set private validator
			pv := privVals[i]
			cs.SetPrivValidator(pv)

			eventBus := types.NewEventBus()
			eventBus.SetLogger(log.TestingLogger().With("module", "events"))
			err = eventBus.Start()
			require.NoError(t, err)
			cs.SetEventBus(eventBus)

			cs.SetTimeoutTicker(tickerFunc())
			cs.SetLogger(logger)

			states[i] = cs
		}()
	}

	rts := setup(t, nValidators, states, 100) // buffer must be large enough to not deadlock

	var bzNodeID p2p.NodeID

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
	bzNodeState.doPrevote = func(height int64, round int32) {
		// allow first height to happen normally so that byzantine validator is no longer proposer
		if height == prevoteHeight {
			prevote1, err := bzNodeState.signVote(
				tmproto.PrevoteType,
				bzNodeState.ProposalBlock.Hash(),
				bzNodeState.ProposalBlockParts.Header(),
			)
			require.NoError(t, err)

			prevote2, err := bzNodeState.signVote(tmproto.PrevoteType, nil, types.PartSetHeader{})
			require.NoError(t, err)

			// send two votes to all peers (1st to one half, 2nd to another half)
			i := 0
			for _, ps := range bzReactor.peers {
				if i < len(bzReactor.peers)/2 {
					bzNodeState.Logger.Info("signed and pushed vote", "vote", prevote1, "peer", ps.peerID)
					bzReactor.voteCh.Out <- p2p.Envelope{
						To: ps.peerID,
						Message: &tmcons.Vote{
							Vote: prevote1.ToProto(),
						},
					}
				} else {
					bzNodeState.Logger.Info("signed and pushed vote", "vote", prevote2, "peer", ps.peerID)
					bzReactor.voteCh.Out <- p2p.Envelope{
						To: ps.peerID,
						Message: &tmcons.Vote{
							Vote: prevote2.ToProto(),
						},
					}
				}

				i++
			}
		} else {
			bzNodeState.Logger.Info("behaving normally")
			bzNodeState.defaultDoPrevote(height, round)
		}
	}

	// Introducing a lazy proposer means that the time of the block committed is
	// different to the timestamp that the other nodes have. This tests to ensure
	// that the evidence that finally gets proposed will have a valid timestamp.
	// lazyProposer := states[1]
	lazyNodeState := states[1]

	lazyNodeState.decideProposal = func(height int64, round int32) {
		lazyNodeState.Logger.Info("Lazy Proposer proposing condensed commit")
		require.NotNil(t, lazyNodeState.privValidator)

		var commit *types.Commit
		switch {
		case lazyNodeState.Height == lazyNodeState.state.InitialHeight:
			// We're creating a proposal for the first block.
			// The commit is empty, but not nil.
			commit = types.NewCommit(0, 0, types.BlockID{}, nil)
		case lazyNodeState.LastCommit.HasTwoThirdsMajority():
			// Make the commit from LastCommit
			commit = lazyNodeState.LastCommit.MakeCommit()
		default: // This shouldn't happen.
			lazyNodeState.Logger.Error("enterPropose: Cannot propose anything: No commit for the previous block")
			return
		}

		// omit the last signature in the commit
		commit.Signatures[len(commit.Signatures)-1] = types.NewCommitSigAbsent()

		if lazyNodeState.privValidatorPubKey == nil {
			// If this node is a validator & proposer in the current round, it will
			// miss the opportunity to create a block.
			lazyNodeState.Logger.Error(fmt.Sprintf("enterPropose: %v", errPubKeyIsNotSet))
			return
		}
		proposerAddr := lazyNodeState.privValidatorPubKey.Address()

		block, blockParts := lazyNodeState.blockExec.CreateProposalBlock(
			lazyNodeState.Height, lazyNodeState.state, commit, proposerAddr,
		)

		// Flush the WAL. Otherwise, we may not recompute the same proposal to sign,
		// and the privValidator will refuse to sign anything.
		if err := lazyNodeState.wal.FlushAndSync(); err != nil {
			lazyNodeState.Logger.Error("Error flushing to disk")
		}

		// Make proposal
		propBlockID := types.BlockID{Hash: block.Hash(), PartSetHeader: blockParts.Header()}
		proposal := types.NewProposal(height, round, lazyNodeState.ValidRound, propBlockID)
		p := proposal.ToProto()
		if err := lazyNodeState.privValidator.SignProposal(context.Background(), lazyNodeState.state.ChainID, p); err == nil {
			proposal.Signature = p.Signature

			// send proposal and block parts on internal msg queue
			lazyNodeState.sendInternalMessage(msgInfo{&ProposalMessage{proposal}, ""})
			for i := 0; i < int(blockParts.Total()); i++ {
				part := blockParts.GetPart(i)
				lazyNodeState.sendInternalMessage(msgInfo{&BlockPartMessage{lazyNodeState.Height, lazyNodeState.Round, part}, ""})
			}
			lazyNodeState.Logger.Info("Signed proposal", "height", height, "round", round, "proposal", proposal)
			lazyNodeState.Logger.Debug(fmt.Sprintf("Signed proposal block: %v", block))
		} else if !lazyNodeState.replayMode {
			lazyNodeState.Logger.Error("enterPropose: Error signing proposal", "height", height, "round", round, "err", err)
		}
	}

	for _, reactor := range rts.reactors {
		state := reactor.state.GetState()
		reactor.SwitchToConsensus(state, false)
	}

	// Evidence should be submitted and committed at the third height but
	// we will check the first six just in case
	evidenceFromEachValidator := make([]types.Evidence, nValidators)

	wg := new(sync.WaitGroup)
	i := 0
	for _, sub := range rts.subs {
		wg.Add(1)

		go func(j int, s types.Subscription) {
			defer wg.Done()
			for {
				select {
				case msg := <-s.Out():
					require.NotNil(t, msg)
					block := msg.Data().(types.EventDataNewBlock).Block
					if len(block.Evidence.Evidence) != 0 {
						evidenceFromEachValidator[j] = block.Evidence.Evidence[0]
						return
					}
				case <-s.Canceled():
					require.Fail(t, "subscription failed for %d", j)
					return
				}
			}
		}(i, sub)

		i++
	}

	wg.Wait()

	pubkey, err := bzNodeState.privValidator.GetPubKey(context.Background())
	require.NoError(t, err)

	for idx, ev := range evidenceFromEachValidator {
		if assert.NotNil(t, ev, idx) {
			ev, ok := ev.(*types.DuplicateVoteEvidence)
			assert.True(t, ok)
			assert.Equal(t, pubkey.Address(), ev.VoteA.ValidatorAddress)
			assert.Equal(t, prevoteHeight, ev.Height())
		}
	}
}

// 4 validators. 1 is byzantine. The other three are partitioned into A (1 val) and B (2 vals).
// byzantine validator sends conflicting proposals into A and B,
// and prevotes/precommits on both of them.
// B sees a commit, A doesn't.
// Heal partition and ensure A sees the commit
func TestByzantineConflictingProposalsWithPartition(t *testing.T) {
	// TODO: https://github.com/tendermint/tendermint/issues/6092
	t.SkipNow()

	// n := 4
	// logger := consensusLogger().With("test", "byzantine")
	// app := newCounter

	// states, cleanup := randConsensusState(n, "consensus_byzantine_test", newMockTickerFunc(false), app)
	// t.Cleanup(cleanup)

	// // give the byzantine validator a normal ticker
	// ticker := NewTimeoutTicker()
	// ticker.SetLogger(states[0].Logger)
	// states[0].SetTimeoutTicker(ticker)

	// p2pLogger := logger.With("module", "p2p")

	// blocksSubs := make([]types.Subscription, n)
	// reactors := make([]p2p.Reactor, n)
	// for i := 0; i < n; i++ {
	// 	// enable txs so we can create different proposals
	// 	assertMempool(states[i].txNotifier).EnableTxsAvailable()

	// 	eventBus := states[i].eventBus
	// 	eventBus.SetLogger(logger.With("module", "events", "validator", i))

	// 	var err error
	// 	blocksSubs[i], err = eventBus.Subscribe(context.Background(), testSubscriber, types.EventQueryNewBlock)
	// 	require.NoError(t, err)

	// 	conR := NewReactor(states[i], true) // so we don't start the consensus states
	// 	conR.SetLogger(logger.With("validator", i))
	// 	conR.SetEventBus(eventBus)

	// 	var conRI p2p.Reactor = conR

	// 	// make first val byzantine
	// 	if i == 0 {
	// 		conRI = NewByzantineReactor(conR)
	// 	}

	// 	reactors[i] = conRI
	// 	err = states[i].blockExec.Store().Save(states[i].state) // for save height 1's validators info
	// 	require.NoError(t, err)
	// }

	// switches := p2p.MakeConnectedSwitches(config.P2P, N, func(i int, sw *p2p.Switch) *p2p.Switch {
	// 	sw.SetLogger(p2pLogger.With("validator", i))
	// 	sw.AddReactor("CONSENSUS", reactors[i])
	// 	return sw
	// }, func(sws []*p2p.Switch, i, j int) {
	// 	// the network starts partitioned with globally active adversary
	// 	if i != 0 {
	// 		return
	// 	}
	// 	p2p.Connect2Switches(sws, i, j)
	// })

	// // make first val byzantine
	// // NOTE: Now, test validators are MockPV, which by default doesn't
	// // do any safety checks.
	// states[0].privValidator.(types.MockPV).DisableChecks()
	// states[0].decideProposal = func(j int32) func(int64, int32) {
	// 	return func(height int64, round int32) {
	// 		byzantineDecideProposalFunc(t, height, round, states[j], switches[j])
	// 	}
	// }(int32(0))
	// // We are setting the prevote function to do nothing because the prevoting
	// // and precommitting are done alongside the proposal.
	// states[0].doPrevote = func(height int64, round int32) {}

	// defer func() {
	// 	for _, sw := range switches {
	// 		err := sw.Stop()
	// 		require.NoError(t, err)
	// 	}
	// }()

	// // start the non-byz state machines.
	// // note these must be started before the byz
	// for i := 1; i < n; i++ {
	// 	cr := reactors[i].(*Reactor)
	// 	cr.SwitchToConsensus(cr.conS.GetState(), false)
	// }

	// // start the byzantine state machine
	// byzR := reactors[0].(*ByzantineReactor)
	// s := byzR.reactor.conS.GetState()
	// byzR.reactor.SwitchToConsensus(s, false)

	// // byz proposer sends one block to peers[0]
	// // and the other block to peers[1] and peers[2].
	// // note peers and switches order don't match.
	// peers := switches[0].Peers().List()

	// // partition A
	// ind0 := getSwitchIndex(switches, peers[0])

	// // partition B
	// ind1 := getSwitchIndex(switches, peers[1])
	// ind2 := getSwitchIndex(switches, peers[2])
	// p2p.Connect2Switches(switches, ind1, ind2)

	// // wait for someone in the big partition (B) to make a block
	// <-blocksSubs[ind2].Out()

	// t.Log("A block has been committed. Healing partition")
	// p2p.Connect2Switches(switches, ind0, ind1)
	// p2p.Connect2Switches(switches, ind0, ind2)

	// // wait till everyone makes the first new block
	// // (one of them already has)
	// wg := new(sync.WaitGroup)
	// for i := 1; i < N-1; i++ {
	// 	wg.Add(1)
	// 	go func(j int) {
	// 		<-blocksSubs[j].Out()
	// 		wg.Done()
	// 	}(i)
	// }

	// done := make(chan struct{})
	// go func() {
	// 	wg.Wait()
	// 	close(done)
	// }()

	// tick := time.NewTicker(time.Second * 10)
	// select {
	// case <-done:
	// case <-tick.C:
	// 	for i, reactor := range reactors {
	// 		t.Log(fmt.Sprintf("Consensus Reactor %v", i))
	// 		t.Log(fmt.Sprintf("%v", reactor))
	// 	}
	// 	t.Fatalf("Timed out waiting for all validators to commit first block")
	// }
}

// func byzantineDecideProposalFunc(t *testing.T, height int64, round int32, cs *State, sw *p2p.Switch) {
// 	// byzantine user should create two proposals and try to split the vote.
// 	// Avoid sending on internalMsgQueue and running consensus state.

// 	// Create a new proposal block from state/txs from the mempool.
// 	block1, blockParts1 := cs.createProposalBlock()
// 	polRound, propBlockID := cs.ValidRound, types.BlockID{Hash: block1.Hash(), PartSetHeader: blockParts1.Header()}
// 	proposal1 := types.NewProposal(height, round, polRound, propBlockID)
// 	p1 := proposal1.ToProto()
// 	if err := cs.privValidator.SignProposal(cs.state.ChainID, p1); err != nil {
// 		t.Error(err)
// 	}

// 	proposal1.Signature = p1.Signature

// 	// some new transactions come in (this ensures that the proposals are different)
// 	deliverTxsRange(cs, 0, 1)

// 	// Create a new proposal block from state/txs from the mempool.
// 	block2, blockParts2 := cs.createProposalBlock()
// 	polRound, propBlockID = cs.ValidRound, types.BlockID{Hash: block2.Hash(), PartSetHeader: blockParts2.Header()}
// 	proposal2 := types.NewProposal(height, round, polRound, propBlockID)
// 	p2 := proposal2.ToProto()
// 	if err := cs.privValidator.SignProposal(cs.state.ChainID, p2); err != nil {
// 		t.Error(err)
// 	}

// 	proposal2.Signature = p2.Signature

// 	block1Hash := block1.Hash()
// 	block2Hash := block2.Hash()

// 	// broadcast conflicting proposals/block parts to peers
// 	peers := sw.Peers().List()
// 	t.Logf("Byzantine: broadcasting conflicting proposals to %d peers", len(peers))
// 	for i, peer := range peers {
// 		if i < len(peers)/2 {
// 			go sendProposalAndParts(height, round, cs, peer, proposal1, block1Hash, blockParts1)
// 		} else {
// 			go sendProposalAndParts(height, round, cs, peer, proposal2, block2Hash, blockParts2)
// 		}
// 	}
// }

// func sendProposalAndParts(
// 	height int64,
// 	round int32,
// 	cs *State,
// 	peer p2p.Peer,
// 	proposal *types.Proposal,
// 	blockHash []byte,
// 	parts *types.PartSet,
// ) {
// 	// proposal
// 	msg := &ProposalMessage{Proposal: proposal}
// 	peer.Send(DataChannel, MustEncode(msg))

// 	// parts
// 	for i := 0; i < int(parts.Total()); i++ {
// 		part := parts.GetPart(i)
// 		msg := &BlockPartMessage{
// 			Height: height, // This tells peer that this part applies to us.
// 			Round:  round,  // This tells peer that this part applies to us.
// 			Part:   part,
// 		}
// 		peer.Send(DataChannel, MustEncode(msg))
// 	}

// 	// votes
// 	cs.mtx.Lock()
// 	prevote, _ := cs.signVote(tmproto.PrevoteType, blockHash, parts.Header())
// 	precommit, _ := cs.signVote(tmproto.PrecommitType, blockHash, parts.Header())
// 	cs.mtx.Unlock()

// 	peer.Send(VoteChannel, MustEncode(&VoteMessage{prevote}))
// 	peer.Send(VoteChannel, MustEncode(&VoteMessage{precommit}))
// }

// type ByzantineReactor struct {
// 	service.Service
// 	reactor *Reactor
// }

// func NewByzantineReactor(conR *Reactor) *ByzantineReactor {
// 	return &ByzantineReactor{
// 		Service: conR,
// 		reactor: conR,
// 	}
// }

// func (br *ByzantineReactor) SetSwitch(s *p2p.Switch)               { br.reactor.SetSwitch(s) }
// func (br *ByzantineReactor) GetChannels() []*p2p.ChannelDescriptor { return br.reactor.GetChannels() }

// func (br *ByzantineReactor) AddPeer(peer p2p.Peer) {
// 	if !br.reactor.IsRunning() {
// 		return
// 	}

// 	// Create peerState for peer
// 	peerState := NewPeerState(peer).SetLogger(br.reactor.Logger)
// 	peer.Set(types.PeerStateKey, peerState)

// 	// Send our state to peer.
// 	// If we're syncing, broadcast a RoundStepMessage later upon SwitchToConsensus().
// 	if !br.reactor.waitSync {
// 		br.reactor.sendNewRoundStepMessage(peer)
// 	}
// }

// func (br *ByzantineReactor) RemovePeer(peer p2p.Peer, reason interface{}) {
// 	br.reactor.RemovePeer(peer, reason)
// }

// func (br *ByzantineReactor) Receive(chID byte, peer p2p.Peer, msgBytes []byte) {
// 	br.reactor.Receive(chID, peer, msgBytes)
// }

// func (br *ByzantineReactor) InitPeer(peer p2p.Peer) p2p.Peer { return peer }
