package evidence_test

import (
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/go-kit/kit/log/term"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	dbm "github.com/tendermint/tm-db"

	"github.com/tendermint/tendermint/evidence"
	"github.com/tendermint/tendermint/evidence/mocks"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/p2p"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	sm "github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/types"
)

var (
	numEvidence = 10
	timeout     = 120 * time.Second // ridiculously high because CircleCI is slow

	rng = rand.New(rand.NewSource(time.Now().UnixNano()))
)

type reactorTestSuite struct {
	reactor *evidence.Reactor
	pool    *evidence.Pool

	peerID p2p.PeerID

	evidenceChannel   *p2p.Channel
	evidenceInCh      chan p2p.Envelope
	evidenceOutCh     chan p2p.Envelope
	evidencePeerErrCh chan p2p.PeerError

	peerUpdates *p2p.PeerUpdatesCh
}

func setup(t *testing.T, logger log.Logger, pool *evidence.Pool, chBuf uint) *reactorTestSuite {
	t.Helper()

	pID := make([]byte, 16)
	_, err := rng.Read(pID)
	require.NoError(t, err)

	rts := &reactorTestSuite{
		pool:              pool,
		evidenceInCh:      make(chan p2p.Envelope, chBuf),
		evidenceOutCh:     make(chan p2p.Envelope, chBuf),
		evidencePeerErrCh: make(chan p2p.PeerError, chBuf),
		peerUpdates:       p2p.NewPeerUpdates(),
		peerID:            pID,
	}

	rts.evidenceChannel = p2p.NewChannel(
		evidence.EvidenceChannel,
		new(tmproto.EvidenceList),
		rts.evidenceInCh,
		rts.evidenceOutCh,
		rts.evidencePeerErrCh,
	)

	rts.reactor = evidence.NewReactor(
		logger,
		rts.evidenceChannel,
		rts.peerUpdates,
		pool,
	)

	require.NoError(t, rts.reactor.Start())
	require.True(t, rts.reactor.IsRunning())

	t.Cleanup(func() {
		require.NoError(t, rts.reactor.Stop())
		require.False(t, rts.reactor.IsRunning())
	})

	return rts
}

// evidenceLogger creates a testing logger which uses a different colors for
// each validator. The "validator" key must exist.
func evidenceLogger() log.Logger {
	return log.TestingLoggerWithColorFn(func(keyVals ...interface{}) term.FgBgColor {
		for i := 0; i < len(keyVals)-1; i += 2 {
			if keyVals[i] == "validator" {
				return term.FgBgColor{Fg: term.Color(uint8(keyVals[i+1].(int) + 1))}
			}
		}

		return term.FgBgColor{}
	})
}

func createTestSuites(t *testing.T, stateStores []sm.Store, chBuf uint) []*reactorTestSuite {
	t.Helper()

	numSStores := len(stateStores)

	testSuites := make([]*reactorTestSuite, numSStores)
	logger := evidenceLogger()
	evidenceTime := time.Date(2019, 1, 1, 0, 0, 0, 0, time.UTC)

	for i := 0; i < numSStores; i++ {
		evidenceDB := dbm.NewMemDB()
		blockStore := &mocks.BlockStore{}
		blockStore.On("LoadBlockMeta", mock.AnythingOfType("int64")).Return(
			&types.BlockMeta{Header: types.Header{Time: evidenceTime}},
		)

		pool, err := evidence.NewPool(evidenceDB, stateStores[i], blockStore)
		require.NoError(t, err)

		testSuites[i] = setup(t, logger.With("validator", i), pool, chBuf)
	}

	return testSuites
}

func waitForEvidence(t *testing.T, evList types.EvidenceList, suites []*reactorTestSuite) {
	wg := new(sync.WaitGroup)

	for _, suite := range suites {
		wg.Add(1)

		go func(s *reactorTestSuite) {
			var localEvList []types.Evidence

			currentPoolSize := 0
			for currentPoolSize != len(evList) {
				// each evidence should not be more than 500 bytes
				localEvList, _ = s.pool.PendingEvidence(int64(len(evList) * 500))
				currentPoolSize = len(localEvList)
			}

			// put the reaped evidence in a map so we can quickly check we got everything
			evMap := make(map[string]types.Evidence)
			for _, e := range localEvList {
				evMap[string(e.Hash())] = e
			}

			for i, expectedEv := range evList {
				gotEv := evMap[string(expectedEv.Hash())]
				require.Equalf(
					t,
					expectedEv,
					gotEv,
					"evidence at index %d in pool does not match; got: %v, expected: %v", i, gotEv, expectedEv,
				)
			}

			wg.Done()
		}(suite)
	}

	// wait for the evidence in all evidence pools
	wg.Wait()
}

func createEvidenceList(
	t *testing.T,
	pool *evidence.Pool,
	val types.PrivValidator,
	numEvidence int,
) types.EvidenceList {
	evList := make([]types.Evidence, numEvidence)
	for i := 0; i < numEvidence; i++ {
		ev := types.NewMockDuplicateVoteEvidenceWithValidator(
			int64(i+1),
			time.Date(2019, 1, 1, 0, 0, 0, 0, time.UTC),
			val,
			evidenceChainID,
		)

		require.NoError(t, pool.AddEvidence(ev))
		evList[i] = ev
	}

	return evList
}

func TestReactorBroadcastEvidence(t *testing.T) {
	numSStores := 7

	// create a stateDB for all test suites (nodes)
	stateDBs := make([]sm.Store, numSStores)
	val := types.NewMockPV()

	// We need all validators saved for heights at least as high as we have
	// evidence for.
	height := int64(numEvidence) + 10
	for i := 0; i < numSStores; i++ {
		stateDBs[i] = initializeValidatorState(t, val, height)
	}

	// Create a series of test suites where each suite contains a reactor and
	// evidence pool. In addition, we mark a primary suite and the rest are
	// secondaries where each secondary is added as a peer via a PeerUpdate to the
	// primary. As a result, the primary will gossip all evidence to each secondary.
	testSuites := createTestSuites(t, stateDBs, 1)
	primary := testSuites[0]
	secondaries := testSuites[1:]

	// create a mapping for efficient suite lookup by peer ID
	suitesByPeerID := make(map[string]*reactorTestSuite)
	for _, suite := range testSuites {
		suitesByPeerID[suite.peerID.String()] = suite
	}

	// Simulate a router by listening for all outbound envelopes and proxying the
	// envelope to the respective peer (suite).
	for _, suite := range testSuites {
		go func(s *reactorTestSuite) {
			for envelope := range s.evidenceOutCh {
				other := suitesByPeerID[envelope.To.String()]
				other.evidenceInCh <- p2p.Envelope{
					From:    s.peerID,
					To:      envelope.To,
					Message: envelope.Message,
				}
			}
		}(suite)
	}

	evList := createEvidenceList(t, primary.pool, val, numEvidence)

	// Add each secondary suite (node) as a peer to the primary suite (node). This
	// will cause the primary to gossip all evidence to the secondaries.
	for _, suite := range secondaries {
		primary.peerUpdates.TestSend(p2p.PeerUpdate{
			Status: p2p.PeerStatusNew,
			PeerID: suite.peerID,
		})
	}

	// Wait till all secondary suites (nodes) received all evidence from the
	// primary suite (node).
	waitForEvidence(t, evList, secondaries)

	for _, suite := range testSuites {
		require.Equal(t, numEvidence, int(suite.pool.Size()))
	}
}

// ============================================================================
// ============================================================================
// ============================================================================

// // We have two evidence reactors connected to one another but are at different heights.
// // Reactor 1 which is ahead receives a number of evidence. It should only send the evidence
// // that is below the height of the peer to that peer.
// func TestReactorSelectiveBroadcast(t *testing.T) {
// 	config := cfg.TestConfig()

// 	val := types.NewMockPV()
// 	height1 := int64(numEvidence) + 10
// 	height2 := int64(numEvidence) / 2

// 	// DB1 is ahead of DB2
// 	stateDB1 := initializeValidatorState(val, height1)
// 	stateDB2 := initializeValidatorState(val, height2)

// 	// make reactors from statedb
// 	reactors, pools := makeAndConnectReactorsAndPools(config, []sm.Store{stateDB1, stateDB2})

// 	// set the peer height on each reactor
// 	for _, r := range reactors {
// 		for _, peer := range r.Switch.Peers().List() {
// 			ps := peerState{height1}
// 			peer.Set(types.PeerStateKey, ps)
// 		}
// 	}

// 	// update the first reactor peer's height to be very small
// 	peer := reactors[0].Switch.Peers().List()[0]
// 	ps := peerState{height2}
// 	peer.Set(types.PeerStateKey, ps)

// 	// send a bunch of valid evidence to the first reactor's evpool
// 	evList := sendEvidence(t, pools[0], val, numEvidence)

// 	// only ones less than the peers height should make it through
// 	waitForEvidence(t, evList[:numEvidence/2-1], []*evidence.Pool{pools[1]})

// 	// peers should still be connected
// 	peers := reactors[1].Switch.Peers().List()
// 	assert.Equal(t, 1, len(peers))
// }

// // This tests aims to ensure that reactors don't send evidence that they have committed or that ar
// // not ready for the peer through three scenarios.
// // First, committed evidence to a newly connected peer
// // Second, evidence to a peer that is behind
// // Third, evidence that was pending and became committed just before the peer caught up
// func TestReactorsGossipNoCommittedEvidence(t *testing.T) {
// 	config := cfg.TestConfig()

// 	val := types.NewMockPV()
// 	var height int64 = 10

// 	// DB1 is ahead of DB2
// 	stateDB1 := initializeValidatorState(val, height-1)
// 	stateDB2 := initializeValidatorState(val, height-2)
// 	state, err := stateDB1.Load()
// 	require.NoError(t, err)
// 	state.LastBlockHeight++

// 	// make reactors from statedb
// 	reactors, pools := makeAndConnectReactorsAndPools(config, []sm.Store{stateDB1, stateDB2})

// 	evList := sendEvidence(t, pools[0], val, 2)
// 	pools[0].Update(state, evList)
// 	require.EqualValues(t, uint32(0), pools[0].Size())

// 	time.Sleep(100 * time.Millisecond)

// 	peer := reactors[0].Switch.Peers().List()[0]
// 	ps := peerState{height - 2}
// 	peer.Set(types.PeerStateKey, ps)

// 	peer = reactors[1].Switch.Peers().List()[0]
// 	ps = peerState{height}
// 	peer.Set(types.PeerStateKey, ps)

// 	// wait to see that no evidence comes through
// 	time.Sleep(300 * time.Millisecond)

// 	// the second pool should not have received any evidence because it has already been committed
// 	assert.Equal(t, uint32(0), pools[1].Size(), "second reactor should not have received evidence")

// 	// the first reactor receives three more evidence
// 	evList = make([]types.Evidence, 3)
// 	for i := 0; i < 3; i++ {
// 		ev := types.NewMockDuplicateVoteEvidenceWithValidator(height-3+int64(i),
// 			time.Date(2019, 1, 1, 0, 0, 0, 0, time.UTC), val, state.ChainID)
// 		err := pools[0].AddEvidence(ev)
// 		require.NoError(t, err)
// 		evList[i] = ev
// 	}

// 	// wait to see that only one evidence is sent
// 	time.Sleep(300 * time.Millisecond)

// 	// the second pool should only have received the first evidence because it is behind
// 	peerEv, _ := pools[1].PendingEvidence(10000)
// 	assert.EqualValues(t, []types.Evidence{evList[0]}, peerEv)

// 	// the last evidence is committed and the second reactor catches up in state to the first
// 	// reactor. We therefore expect that the second reactor only receives one more evidence, the
// 	// one that is still pending and not the evidence that has already been committed.
// 	state.LastBlockHeight++
// 	pools[0].Update(state, []types.Evidence{evList[2]})
// 	// the first reactor should have the two remaining pending evidence
// 	require.EqualValues(t, uint32(2), pools[0].Size())

// 	// now update the state of the second reactor
// 	pools[1].Update(state, types.EvidenceList{})
// 	peer = reactors[0].Switch.Peers().List()[0]
// 	ps = peerState{height}
// 	peer.Set(types.PeerStateKey, ps)

// 	// wait to see that only two evidence is sent
// 	time.Sleep(300 * time.Millisecond)

// 	peerEv, _ = pools[1].PendingEvidence(1000)
// 	assert.EqualValues(t, []types.Evidence{evList[0], evList[1]}, peerEv)
// }

// type peerState struct {
// 	height int64
// }

// func (ps peerState) GetHeight() int64 {
// 	return ps.height
// }

// func exampleVote(t byte) *types.Vote {
// 	var stamp, err = time.Parse(types.TimeFormat, "2017-12-25T03:00:01.234Z")
// 	if err != nil {
// 		panic(err)
// 	}

// 	return &types.Vote{
// 		Type:      tmproto.SignedMsgType(t),
// 		Height:    3,
// 		Round:     2,
// 		Timestamp: stamp,
// 		BlockID: types.BlockID{
// 			Hash: tmhash.Sum([]byte("blockID_hash")),
// 			PartSetHeader: types.PartSetHeader{
// 				Total: 1000000,
// 				Hash:  tmhash.Sum([]byte("blockID_part_set_header_hash")),
// 			},
// 		},
// 		ValidatorAddress: crypto.AddressHash([]byte("validator_address")),
// 		ValidatorIndex:   56789,
// 	}
// }

// // nolint:lll //ignore line length for tests
// func TestEvidenceVectors(t *testing.T) {

// 	val := &types.Validator{
// 		Address:     crypto.AddressHash([]byte("validator_address")),
// 		VotingPower: 10,
// 	}

// 	valSet := types.NewValidatorSet([]*types.Validator{val})

// 	dupl := types.NewDuplicateVoteEvidence(
// 		exampleVote(1),
// 		exampleVote(2),
// 		defaultEvidenceTime,
// 		valSet,
// 	)

// 	testCases := []struct {
// 		testName     string
// 		evidenceList []types.Evidence
// 		expBytes     string
// 	}{
// 		{"DuplicateVoteEvidence", []types.Evidence{dupl}, "0a85020a82020a79080210031802224a0a208b01023386c371778ecb6368573e539afc3cc860ec3a2f614e54fe5652f4fc80122608c0843d122072db3d959635dff1bb567bedaa70573392c5159666a3f8caf11e413aac52207a2a0b08b1d381d20510809dca6f32146af1f4111082efb388211bc72c55bcd61e9ac3d538d5bb031279080110031802224a0a208b01023386c371778ecb6368573e539afc3cc860ec3a2f614e54fe5652f4fc80122608c0843d122072db3d959635dff1bb567bedaa70573392c5159666a3f8caf11e413aac52207a2a0b08b1d381d20510809dca6f32146af1f4111082efb388211bc72c55bcd61e9ac3d538d5bb03180a200a2a060880dbaae105"},
// 	}

// 	for _, tc := range testCases {
// 		tc := tc

// 		evi := make([]tmproto.Evidence, len(tc.evidenceList))
// 		for i := 0; i < len(tc.evidenceList); i++ {
// 			ev, err := types.EvidenceToProto(tc.evidenceList[i])
// 			require.NoError(t, err, tc.testName)
// 			evi[i] = *ev
// 		}

// 		epl := tmproto.EvidenceList{
// 			Evidence: evi,
// 		}

// 		bz, err := epl.Marshal()
// 		require.NoError(t, err, tc.testName)

// 		require.Equal(t, tc.expBytes, hex.EncodeToString(bz), tc.testName)

// 	}

// }
