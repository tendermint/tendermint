package evidence_test

import (
	"encoding/hex"
	"fmt"
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	dbm "github.com/tendermint/tm-db"

	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/tmhash"
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

	rng = rand.New(rand.NewSource(time.Now().UnixNano()))
)

type reactorTestSuite struct {
	reactor *evidence.Reactor
	pool    *evidence.Pool

	peerID p2p.NodeID

	evidenceChannel   *p2p.Channel
	evidenceInCh      chan p2p.Envelope
	evidenceOutCh     chan p2p.Envelope
	evidencePeerErrCh chan p2p.PeerError

	peerUpdatesCh chan p2p.PeerUpdate
	peerUpdates   *p2p.PeerUpdates
}

func setup(t *testing.T, logger log.Logger, pool *evidence.Pool, chBuf uint) *reactorTestSuite {
	t.Helper()

	pID := make([]byte, 16)
	_, err := rng.Read(pID)
	require.NoError(t, err)

	peerUpdatesCh := make(chan p2p.PeerUpdate)

	rts := &reactorTestSuite{
		pool:              pool,
		evidenceInCh:      make(chan p2p.Envelope, chBuf),
		evidenceOutCh:     make(chan p2p.Envelope, chBuf),
		evidencePeerErrCh: make(chan p2p.PeerError, chBuf),
		peerUpdatesCh:     peerUpdatesCh,
		peerUpdates:       p2p.NewPeerUpdates(peerUpdatesCh),
		peerID:            p2p.NodeID(fmt.Sprintf("%x", pID)),
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

func createTestSuites(t *testing.T, stateStores []sm.Store, chBuf uint) []*reactorTestSuite {
	t.Helper()

	numSStores := len(stateStores)
	testSuites := make([]*reactorTestSuite, numSStores)
	evidenceTime := time.Date(2019, 1, 1, 0, 0, 0, 0, time.UTC)

	for i := 0; i < numSStores; i++ {
		logger := log.TestingLogger().With("validator", i)
		evidenceDB := dbm.NewMemDB()
		blockStore := &mocks.BlockStore{}
		blockStore.On("LoadBlockMeta", mock.AnythingOfType("int64")).Return(
			&types.BlockMeta{Header: types.Header{Time: evidenceTime}},
		)

		pool, err := evidence.NewPool(logger, evidenceDB, stateStores[i], blockStore)
		require.NoError(t, err)

		testSuites[i] = setup(t, logger, pool, chBuf)
	}

	return testSuites
}

func waitForEvidence(t *testing.T, evList types.EvidenceList, suites ...*reactorTestSuite) {
	t.Helper()

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

// simulateRouter will increment the provided WaitGroup and execute a simulated
// router where, for each outbound p2p Envelope from the primary reactor, we
// proxy (send) the Envelope the relevant peer reactor. Done is invoked on the
// WaitGroup when numOut Envelopes are sent (i.e. read from the outbound channel).
func simulateRouter(wg *sync.WaitGroup, primary *reactorTestSuite, suites []*reactorTestSuite, numOut int) {
	wg.Add(1)

	// create a mapping for efficient suite lookup by peer ID
	suitesByPeerID := make(map[p2p.NodeID]*reactorTestSuite)
	for _, suite := range suites {
		suitesByPeerID[suite.peerID] = suite
	}

	// Simulate a router by listening for all outbound envelopes and proxying the
	// envelope to the respective peer (suite).
	go func() {
		for i := 0; i < numOut; i++ {
			envelope := <-primary.evidenceOutCh
			other := suitesByPeerID[envelope.To]

			other.evidenceInCh <- p2p.Envelope{
				From:    primary.peerID,
				To:      envelope.To,
				Message: envelope.Message,
			}
		}

		wg.Done()
	}()
}

func TestReactorMultiDisconnect(t *testing.T) {
	val := types.NewMockPV()
	height := int64(numEvidence) + 10

	stateDB1 := initializeValidatorState(t, val, height)
	stateDB2 := initializeValidatorState(t, val, height)

	testSuites := createTestSuites(t, []sm.Store{stateDB1, stateDB2}, 20)
	primary := testSuites[0]
	secondary := testSuites[1]

	_ = createEvidenceList(t, primary.pool, val, numEvidence)

	primary.peerUpdatesCh <- p2p.PeerUpdate{
		Status: p2p.PeerStatusUp,
		NodeID: secondary.peerID,
	}

	// Ensure "disconnecting" the secondary peer from the primary more than once
	// is handled gracefully.
	primary.peerUpdatesCh <- p2p.PeerUpdate{
		Status: p2p.PeerStatusDown,
		NodeID: secondary.peerID,
	}
	primary.peerUpdatesCh <- p2p.PeerUpdate{
		Status: p2p.PeerStatusDown,
		NodeID: secondary.peerID,
	}
}

// TestReactorBroadcastEvidence creates an environment of multiple peers that
// are all at the same height. One peer, designated as a primary, gossips all
// evidence to the remaining peers.
func TestReactorBroadcastEvidence(t *testing.T) {
	numPeers := 7

	// create a stateDB for all test suites (nodes)
	stateDBs := make([]sm.Store, numPeers)
	val := types.NewMockPV()

	// We need all validators saved for heights at least as high as we have
	// evidence for.
	height := int64(numEvidence) + 10
	for i := 0; i < numPeers; i++ {
		stateDBs[i] = initializeValidatorState(t, val, height)
	}

	// Create a series of test suites where each suite contains a reactor and
	// evidence pool. In addition, we mark a primary suite and the rest are
	// secondaries where each secondary is added as a peer via a PeerUpdate to the
	// primary. As a result, the primary will gossip all evidence to each secondary.
	testSuites := createTestSuites(t, stateDBs, 0)
	primary := testSuites[0]
	secondaries := testSuites[1:]

	// Simulate a router by listening for all outbound envelopes and proxying the
	// envelopes to the respective peer (suite).
	wg := new(sync.WaitGroup)
	simulateRouter(wg, primary, testSuites, numEvidence*len(secondaries))

	evList := createEvidenceList(t, primary.pool, val, numEvidence)

	// Add each secondary suite (node) as a peer to the primary suite (node). This
	// will cause the primary to gossip all evidence to the secondaries.
	for _, suite := range secondaries {
		primary.peerUpdatesCh <- p2p.PeerUpdate{
			Status: p2p.PeerStatusUp,
			NodeID: suite.peerID,
		}
	}

	// Wait till all secondary suites (reactor) received all evidence from the
	// primary suite (node).
	waitForEvidence(t, evList, secondaries...)

	for _, suite := range testSuites {
		require.Equal(t, numEvidence, int(suite.pool.Size()))
	}

	wg.Wait()

	// ensure all channels are drained
	for _, suite := range testSuites {
		require.Empty(t, suite.evidenceOutCh)
	}
}

// TestReactorSelectiveBroadcast tests a context where we have two reactors
// connected to one another but are at different heights. Reactor 1 which is
// ahead receives a list of evidence.
func TestReactorBroadcastEvidence_Lagging(t *testing.T) {
	val := types.NewMockPV()
	height1 := int64(numEvidence) + 10
	height2 := int64(numEvidence) / 2

	// stateDB1 is ahead of stateDB2, where stateDB1 has all heights (1-10) and
	// stateDB2 only has heights 1-7.
	stateDB1 := initializeValidatorState(t, val, height1)
	stateDB2 := initializeValidatorState(t, val, height2)

	testSuites := createTestSuites(t, []sm.Store{stateDB1, stateDB2}, 0)
	primary := testSuites[0]
	secondaries := testSuites[1:]

	// Simulate a router by listening for all outbound envelopes and proxying the
	// envelope to the respective peer (suite).
	wg := new(sync.WaitGroup)
	simulateRouter(wg, primary, testSuites, numEvidence*len(secondaries))

	// Send a list of valid evidence to the first reactor's, the one that is ahead,
	// evidence pool.
	evList := createEvidenceList(t, primary.pool, val, numEvidence)

	// Add each secondary suite (node) as a peer to the primary suite (node). This
	// will cause the primary to gossip all evidence to the secondaries.
	for _, suite := range secondaries {
		primary.peerUpdatesCh <- p2p.PeerUpdate{
			Status: p2p.PeerStatusUp,
			NodeID: suite.peerID,
		}
	}

	// only ones less than the peers height should make it through
	waitForEvidence(t, evList[:height2+2], secondaries...)

	require.Equal(t, numEvidence, int(primary.pool.Size()))
	require.Equal(t, int(height2+2), int(secondaries[0].pool.Size()))

	// The primary will continue to send the remaining evidence to the secondaries
	// so we wait until it has sent all the envelopes.
	wg.Wait()

	// ensure all channels are drained
	for _, suite := range testSuites {
		require.Empty(t, suite.evidenceOutCh)
	}
}

func TestReactorBroadcastEvidence_Pending(t *testing.T) {
	val := types.NewMockPV()
	height := int64(10)

	stateDB1 := initializeValidatorState(t, val, height)
	stateDB2 := initializeValidatorState(t, val, height)

	testSuites := createTestSuites(t, []sm.Store{stateDB1, stateDB2}, 0)
	primary := testSuites[0]
	secondary := testSuites[1]

	// Simulate a router by listening for all outbound envelopes and proxying the
	// envelopes to the respective peer (suite).
	wg := new(sync.WaitGroup)
	simulateRouter(wg, primary, testSuites, numEvidence)

	// add all evidence to the primary reactor
	evList := createEvidenceList(t, primary.pool, val, numEvidence)

	// Manually add half the evidence to the secondary which will mark them as
	// pending.
	for i := 0; i < numEvidence/2; i++ {
		require.NoError(t, secondary.pool.AddEvidence(evList[i]))
	}

	// the secondary should have half the evidence as pending
	require.Equal(t, uint32(numEvidence/2), secondary.pool.Size())

	// add the secondary reactor as a peer to the primary reactor
	primary.peerUpdatesCh <- p2p.PeerUpdate{
		Status: p2p.PeerStatusUp,
		NodeID: secondary.peerID,
	}

	// The secondary reactor should have received all the evidence ignoring the
	// already pending evidence.
	waitForEvidence(t, evList, secondary)

	for _, suite := range testSuites {
		require.Equal(t, numEvidence, int(suite.pool.Size()))
	}

	wg.Wait()

	// ensure all channels are drained
	for _, suite := range testSuites {
		require.Empty(t, suite.evidenceOutCh)
	}
}

func TestReactorBroadcastEvidence_Committed(t *testing.T) {
	val := types.NewMockPV()
	height := int64(10)

	stateDB1 := initializeValidatorState(t, val, height)
	stateDB2 := initializeValidatorState(t, val, height)

	testSuites := createTestSuites(t, []sm.Store{stateDB1, stateDB2}, 0)
	primary := testSuites[0]
	secondary := testSuites[1]

	// add all evidence to the primary reactor
	evList := createEvidenceList(t, primary.pool, val, numEvidence)

	// Manually add half the evidence to the secondary which will mark them as
	// pending.
	for i := 0; i < numEvidence/2; i++ {
		require.NoError(t, secondary.pool.AddEvidence(evList[i]))
	}

	// the secondary should have half the evidence as pending
	require.Equal(t, uint32(numEvidence/2), secondary.pool.Size())

	state, err := stateDB2.Load()
	require.NoError(t, err)

	// update the secondary's pool such that all pending evidence is committed
	state.LastBlockHeight++
	secondary.pool.Update(state, evList[:numEvidence/2])

	// the secondary should have half the evidence as committed
	require.Equal(t, uint32(0), secondary.pool.Size())

	// Simulate a router by listening for all outbound envelopes and proxying the
	// envelopes to the respective peer (suite).
	wg := new(sync.WaitGroup)
	simulateRouter(wg, primary, testSuites, numEvidence)

	// add the secondary reactor as a peer to the primary reactor
	primary.peerUpdatesCh <- p2p.PeerUpdate{
		Status: p2p.PeerStatusUp,
		NodeID: secondary.peerID,
	}

	// The secondary reactor should have received all the evidence ignoring the
	// already committed evidence.
	waitForEvidence(t, evList[numEvidence/2:], secondary)

	require.Equal(t, numEvidence, int(primary.pool.Size()))
	require.Equal(t, numEvidence/2, int(secondary.pool.Size()))

	wg.Wait()

	// ensure all channels are drained
	for _, suite := range testSuites {
		require.Empty(t, suite.evidenceOutCh)
	}
}

func TestReactorBroadcastEvidence_FullyConnected(t *testing.T) {
	numPeers := 7

	// create a stateDB for all test suites (nodes)
	stateDBs := make([]sm.Store, numPeers)
	val := types.NewMockPV()

	// We need all validators saved for heights at least as high as we have
	// evidence for.
	height := int64(numEvidence) + 10
	for i := 0; i < numPeers; i++ {
		stateDBs[i] = initializeValidatorState(t, val, height)
	}

	testSuites := createTestSuites(t, stateDBs, 0)

	// Simulate a router by listening for all outbound envelopes and proxying the
	// envelopes to the respective peer (suite).
	wg := new(sync.WaitGroup)
	for _, suite := range testSuites {
		simulateRouter(wg, suite, testSuites, numEvidence*(len(testSuites)-1))
	}

	evList := createEvidenceList(t, testSuites[0].pool, val, numEvidence)

	// every suite (reactor) connects to every other suite (reactor)
	for _, suiteI := range testSuites {
		for _, suiteJ := range testSuites {
			if suiteI.peerID != suiteJ.peerID {
				suiteI.peerUpdatesCh <- p2p.PeerUpdate{
					Status: p2p.PeerStatusUp,
					NodeID: suiteJ.peerID,
				}
			}
		}
	}

	// wait till all suites (reactors) received all evidence from other suites (reactors)
	waitForEvidence(t, evList, testSuites...)

	for _, suite := range testSuites {
		require.Equal(t, numEvidence, int(suite.pool.Size()))

		// commit state so we do not continue to repeat gossiping the same evidence
		state := suite.pool.State()
		state.LastBlockHeight++
		suite.pool.Update(state, evList)
	}

	wg.Wait()
}

func TestReactorBroadcastEvidence_RemovePeer(t *testing.T) {
	val := types.NewMockPV()
	height := int64(10)

	stateDB1 := initializeValidatorState(t, val, height)
	stateDB2 := initializeValidatorState(t, val, height)

	testSuites := createTestSuites(t, []sm.Store{stateDB1, stateDB2}, uint(numEvidence))
	primary := testSuites[0]
	secondary := testSuites[1]

	// Simulate a router by listening for all outbound envelopes and proxying the
	// envelopes to the respective peer (suite).
	wg := new(sync.WaitGroup)
	simulateRouter(wg, primary, testSuites, numEvidence/2)

	// add all evidence to the primary reactor
	evList := createEvidenceList(t, primary.pool, val, numEvidence)

	// add the secondary reactor as a peer to the primary reactor
	primary.peerUpdatesCh <- p2p.PeerUpdate{
		Status: p2p.PeerStatusUp,
		NodeID: secondary.peerID,
	}

	// have the secondary reactor receive only half the evidence
	waitForEvidence(t, evList[:numEvidence/2], secondary)

	// disconnect the peer
	primary.peerUpdatesCh <- p2p.PeerUpdate{
		Status: p2p.PeerStatusDown,
		NodeID: secondary.peerID,
	}

	// Ensure the secondary only received half of the evidence before being
	// disconnected.
	require.Equal(t, numEvidence/2, int(secondary.pool.Size()))

	wg.Wait()

	// The primary reactor should still be attempting to send the remaining half.
	//
	// NOTE: The channel is buffered (size numEvidence) as to ensure the primary
	// reactor will send all envelopes at once before receiving the signal to stop
	// gossiping.
	for i := 0; i < numEvidence/2; i++ {
		<-primary.evidenceOutCh
	}

	// ensure all channels are drained
	for _, suite := range testSuites {
		require.Empty(t, suite.evidenceOutCh)
	}
}

// nolint:lll
func TestEvidenceListSerialization(t *testing.T) {
	exampleVote := func(msgType byte) *types.Vote {
		var stamp, err = time.Parse(types.TimeFormat, "2017-12-25T03:00:01.234Z")
		require.NoError(t, err)

		return &types.Vote{
			Type:      tmproto.SignedMsgType(msgType),
			Height:    3,
			Round:     2,
			Timestamp: stamp,
			BlockID: types.BlockID{
				Hash: tmhash.Sum([]byte("blockID_hash")),
				PartSetHeader: types.PartSetHeader{
					Total: 1000000,
					Hash:  tmhash.Sum([]byte("blockID_part_set_header_hash")),
				},
			},
			ValidatorAddress: crypto.AddressHash([]byte("validator_address")),
			ValidatorIndex:   56789,
		}
	}

	val := &types.Validator{
		Address:     crypto.AddressHash([]byte("validator_address")),
		VotingPower: 10,
	}

	valSet := types.NewValidatorSet([]*types.Validator{val})

	dupl := types.NewDuplicateVoteEvidence(
		exampleVote(1),
		exampleVote(2),
		defaultEvidenceTime,
		valSet,
	)

	testCases := map[string]struct {
		evidenceList []types.Evidence
		expBytes     string
	}{
		"DuplicateVoteEvidence": {
			[]types.Evidence{dupl},
			"0a85020a82020a79080210031802224a0a208b01023386c371778ecb6368573e539afc3cc860ec3a2f614e54fe5652f4fc80122608c0843d122072db3d959635dff1bb567bedaa70573392c5159666a3f8caf11e413aac52207a2a0b08b1d381d20510809dca6f32146af1f4111082efb388211bc72c55bcd61e9ac3d538d5bb031279080110031802224a0a208b01023386c371778ecb6368573e539afc3cc860ec3a2f614e54fe5652f4fc80122608c0843d122072db3d959635dff1bb567bedaa70573392c5159666a3f8caf11e413aac52207a2a0b08b1d381d20510809dca6f32146af1f4111082efb388211bc72c55bcd61e9ac3d538d5bb03180a200a2a060880dbaae105",
		},
	}

	for name, tc := range testCases {
		tc := tc

		t.Run(name, func(t *testing.T) {
			protoEv := make([]tmproto.Evidence, len(tc.evidenceList))
			for i := 0; i < len(tc.evidenceList); i++ {
				ev, err := types.EvidenceToProto(tc.evidenceList[i])
				require.NoError(t, err)
				protoEv[i] = *ev
			}

			epl := tmproto.EvidenceList{
				Evidence: protoEv,
			}

			bz, err := epl.Marshal()
			require.NoError(t, err)

			require.Equal(t, tc.expBytes, hex.EncodeToString(bz))
		})
	}
}
