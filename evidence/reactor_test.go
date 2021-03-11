package evidence_test

import (
	"encoding/hex"
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/fortytw2/leaktest"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	dbm "github.com/tendermint/tm-db"

	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/tmhash"
	"github.com/tendermint/tendermint/evidence"
	"github.com/tendermint/tendermint/evidence/mocks"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/p2p"
	"github.com/tendermint/tendermint/p2p/p2ptest"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	sm "github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/types"
)

var (
	numEvidence = 10

	rng = rand.New(rand.NewSource(time.Now().UnixNano()))
)

type reactorTestSuite struct {
	network      *p2ptest.Network
	logger       log.Logger
	reactors     map[p2p.NodeID]*evidence.Reactor
	pools        map[p2p.NodeID]*evidence.Pool
	dataChannels map[p2p.NodeID]*p2p.Channel
	peers        map[p2p.NodeID]*p2p.PeerUpdates
	peerChans    map[p2p.NodeID]chan p2p.PeerUpdate
}

func setup(t *testing.T, stateStores []sm.Store, chBuf uint) *reactorTestSuite {
	t.Helper()

	numStateStores := len(stateStores)

	pID := make([]byte, 16)
	_, err := rng.Read(pID)
	require.NoError(t, err)

	rts := &reactorTestSuite{
		logger:       log.TestingLogger().With("testCase", t.Name()),
		network:      p2ptest.MakeNetwork(t, numStateStores),
		reactors:     make(map[p2p.NodeID]*evidence.Reactor, numStateStores),
		pools:        make(map[p2p.NodeID]*evidence.Pool, numStateStores),
		dataChannels: make(map[p2p.NodeID]*p2p.Channel, numStateStores),
		peers:        make(map[p2p.NodeID]*p2p.PeerUpdates, numStateStores),
		peerChans:    make(map[p2p.NodeID]chan p2p.PeerUpdate, numStateStores),
	}

	rts.dataChannels = rts.network.MakeChannelsNoCleanup(t, evidence.EvidenceChannel, new(tmproto.Evidence), int(chBuf))
	require.Len(t, rts.network.RandomNode().PeerManager.Peers(), 0)

	idx := 0
	evidenceTime := time.Date(2019, 1, 1, 0, 0, 0, 0, time.UTC)
	for nodeID, node := range rts.network.Nodes {
		logger := rts.logger.With("validator", idx)
		evidenceDB := dbm.NewMemDB()
		blockStore := &mocks.BlockStore{}
		blockStore.On("LoadBlockMeta", mock.AnythingOfType("int64")).Return(
			&types.BlockMeta{Header: types.Header{Time: evidenceTime}},
		)
		rts.pools[nodeID], err = evidence.NewPool(logger, evidenceDB, stateStores[idx], blockStore)
		require.NoError(t, err)

		rts.peerChans[nodeID] = make(chan p2p.PeerUpdate)
		rts.peers[nodeID] = p2p.NewPeerUpdates(rts.peerChans[nodeID])
		node.PeerManager.Register(rts.peers[nodeID])

		rts.reactors[nodeID] = evidence.NewReactor(logger, rts.dataChannels[nodeID], rts.peers[nodeID], rts.pools[nodeID])

		require.NoError(t, rts.reactors[nodeID].Start())
		require.True(t, rts.reactors[nodeID].IsRunning())

		idx++
	}

	rts.network.Start(t)
	require.Len(t, rts.network.RandomNode().PeerManager.Peers(), numStateStores-1,
		"network does not have expected number of nodes")

	t.Cleanup(func() {
		for _, r := range rts.reactors {
			require.NoError(t, r.Stop())
			require.False(t, r.IsRunning())
		}

		leaktest.Check(t)
	})

	return rts
}

func (rts *reactorTestSuite) getPrimaryAndSecondary(t *testing.T) (primary *p2ptest.Node, secondary *p2ptest.Node) {
	t.Helper()
	require.True(t, len(rts.network.Nodes) >= 2, "insufficient network size %d", len(rts.network.Nodes))

	attempts := 0
	for {
		require.True(t, attempts < 20, "could not resolve two random distinct nodes") // avoid spinning forever
		attempts++

		primary = rts.network.RandomNode()
		secondary = rts.network.RandomNode()
		if primary.NodeID != secondary.NodeID {
			break
		}
	}

	return // primary, secondary
}

func (rts *reactorTestSuite) waitForEvidence(t *testing.T, evList types.EvidenceList, ids ...p2p.NodeID) {
	t.Helper()

	wg := sync.WaitGroup{}

	for id := range rts.pools {
		if len(ids) > 0 && !p2ptest.NodeInSlice(id, ids) {
			continue
		}

		wg.Add(1)
		go func(pool *evidence.Pool) {
			defer wg.Done()

			var localEvList []types.Evidence

			for len(localEvList) != len(evList) {
				// each evidence should not be more than 500 bytes
				localEvList, _ = pool.PendingEvidence(int64(len(evList) * 5000))
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
					"evidence for pool %d in pool does not match; got: %v, expected: %v", i, gotEv, expectedEv,
				)
			}
		}(rts.pools[id])
	}
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

func TestReactorMultiDisconnect(t *testing.T) {
	val := types.NewMockPV()
	height := int64(numEvidence) + 10

	stateDB1 := initializeValidatorState(t, val, height)
	stateDB2 := initializeValidatorState(t, val, height)

	rts := setup(t, []sm.Store{stateDB1, stateDB2}, 20)
	primary, secondary := rts.getPrimaryAndSecondary(t)

	_ = createEvidenceList(t, rts.pools[primary.NodeID], val, numEvidence)

	rts.peerChans[primary.NodeID] <- p2p.PeerUpdate{
		Status: p2p.PeerStatusUp,
		NodeID: secondary.NodeID,
	}

	// Ensure "disconnecting" the secondary peer from the primary more than once
	// is handled gracefully.
	rts.peerChans[primary.NodeID] <- p2p.PeerUpdate{
		Status: p2p.PeerStatusDown,
		NodeID: secondary.NodeID,
	}
	rts.peerChans[primary.NodeID] <- p2p.PeerUpdate{
		Status: p2p.PeerStatusDown,
		NodeID: secondary.NodeID,
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

	rts := setup(t, stateDBs, 0)

	// Create a series of fixtures where each suite contains a reactor and
	// evidence pool. In addition, we mark a primary suite and the rest are
	// secondaries where each secondary is added as a peer via a PeerUpdate to the
	// primary. As a result, the primary will gossip all evidence to each secondary.
	primary := rts.network.RandomNode()
	secondaries := make([]*p2ptest.Node, 0, len(rts.network.NodeIDs())-1)
	secondaryIDs := make([]p2p.NodeID, 0, cap(secondaries))
	for id := range rts.network.Nodes {
		if id == primary.NodeID {
			continue
		}

		secondaries = append(secondaries, rts.network.Nodes[id])
		secondaryIDs = append(secondaryIDs, id)
	}

	evList := createEvidenceList(t, rts.pools[primary.NodeID], val, numEvidence)

	// Add each secondary suite (node) as a peer to the primary suite (node). This
	// will cause the primary to gossip all evidence to the secondaries.
	for _, suite := range secondaries {
		rts.peerChans[primary.NodeID] <- p2p.PeerUpdate{
			Status: p2p.PeerStatusUp,
			NodeID: suite.NodeID,
		}
	}

	// Wait till all secondary suites (reactor) received all evidence from the
	// primary suite (node).
	rts.waitForEvidence(t, evList, secondaryIDs...)

	for _, pool := range rts.pools {
		require.Equal(t, numEvidence, int(pool.Size()))
	}

	// ensure all channels are drained
	for _, ech := range rts.dataChannels {
		require.Empty(t, ech)
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

	rts := setup(t, []sm.Store{stateDB1, stateDB2}, 0)
	primary := rts.network.RandomNode()
	secondaries := make([]*p2ptest.Node, 0, len(rts.network.NodeIDs())-1)
	secondaryIDs := make([]p2p.NodeID, 0, cap(secondaries))
	nodes := rts.network.Nodes
	for id := range nodes {
		if id == primary.NodeID {
			continue
		}
		secondaries = append(secondaries, nodes[id])
		secondaryIDs = append(secondaryIDs, id)
	}

	// Send a list of valid evidence to the first reactor's, the one that is ahead,
	// evidence pool.
	evList := createEvidenceList(t, rts.pools[primary.NodeID], val, numEvidence)

	// Add each secondary suite (node) as a peer to the primary suite (node). This
	// will cause the primary to gossip all evidence to the secondaries.
	for _, secondary := range secondaries {
		rts.peerChans[primary.NodeID] <- p2p.PeerUpdate{
			Status: p2p.PeerStatusUp,
			NodeID: secondary.NodeID,
		}
	}

	// only ones less than the peers height should make it through
	rts.waitForEvidence(t, evList[:height2+2], secondaryIDs...)

	require.Equal(t, numEvidence, int(rts.pools[primary.NodeID].Size()))
	require.Equal(t, int(height2+2), int(rts.pools[secondaries[0].NodeID].Size()))

	// ensure all channels are drained
	for _, ech := range rts.dataChannels {
		require.Empty(t, ech)
	}
}

func TestReactorBroadcastEvidence_Pending(t *testing.T) {
	val := types.NewMockPV()
	height := int64(10)

	stateDB1 := initializeValidatorState(t, val, height)
	stateDB2 := initializeValidatorState(t, val, height)

	rts := setup(t, []sm.Store{stateDB1, stateDB2}, 0)
	primary, secondary := rts.getPrimaryAndSecondary(t)

	evList := createEvidenceList(t, rts.pools[primary.NodeID], val, numEvidence)

	// Manually add half the evidence to the secondary which will mark them as
	// pending.
	for i := 0; i < numEvidence/2; i++ {
		require.NoError(t, rts.pools[secondary.NodeID].AddEvidence(evList[i]))
	}

	// the secondary should have half the evidence as pending
	require.Equal(t, uint32(numEvidence/2), rts.pools[secondary.NodeID].Size())

	// add the secondary reactor as a peer to the primary reactor
	rts.peerChans[primary.NodeID] <- p2p.PeerUpdate{
		Status: p2p.PeerStatusUp,
		NodeID: secondary.NodeID,
	}

	// The secondary reactor should have received all the evidence ignoring the
	// already pending evidence.
	rts.waitForEvidence(t, evList, secondary.NodeID)

	for _, pool := range rts.pools {
		require.Equal(t, numEvidence, int(pool.Size()))
	}

	// ensure all channels are drained
	for _, ech := range rts.dataChannels {
		require.Empty(t, ech)
	}
}

func TestReactorBroadcastEvidence_Committed(t *testing.T) {
	val := types.NewMockPV()
	height := int64(10)

	stateDB1 := initializeValidatorState(t, val, height)
	stateDB2 := initializeValidatorState(t, val, height)

	rts := setup(t, []sm.Store{stateDB1, stateDB2}, 0)
	primary, secondary := rts.getPrimaryAndSecondary(t)

	// add all evidence to the primary reactor
	evList := createEvidenceList(t, rts.pools[primary.NodeID], val, numEvidence)

	// Manually add half the evidence to the secondary which will mark them as
	// pending.
	for i := 0; i < numEvidence/2; i++ {
		require.NoError(t, rts.pools[secondary.NodeID].AddEvidence(evList[i]))
	}

	// the secondary should have half the evidence as pending
	require.Equal(t, uint32(numEvidence/2), rts.pools[secondary.NodeID].Size())

	state, err := stateDB2.Load()
	require.NoError(t, err)

	// update the secondary's pool such that all pending evidence is committed
	state.LastBlockHeight++
	rts.pools[secondary.NodeID].Update(state, evList[:numEvidence/2])

	// the secondary should have half the evidence as committed
	require.Equal(t, uint32(0), rts.pools[secondary.NodeID].Size())

	// add the secondary reactor as a peer to the primary reactor
	rts.peerChans[primary.NodeID] <- p2p.PeerUpdate{
		Status: p2p.PeerStatusUp,
		NodeID: secondary.NodeID,
	}

	// The secondary reactor should have received all the evidence ignoring the
	// already committed evidence.
	rts.waitForEvidence(t, evList[numEvidence/2:], secondary.NodeID)

	require.Equal(t, numEvidence, int(rts.pools[primary.NodeID].Size()))
	require.Equal(t, numEvidence/2, int(rts.pools[primary.NodeID].Size()))

	// ensure all channels are drained
	for _, ech := range rts.dataChannels {
		require.Empty(t, ech)
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

	rts := setup(t, stateDBs, 0)

	evList := createEvidenceList(t, rts.pools[rts.network.RandomNode().NodeID], val, numEvidence)

	// every suite (reactor) connects to every other suite (reactor)
	for outerID, outerChan := range rts.peerChans {
		for innerID := range rts.peerChans {
			if outerID != innerID {
				outerChan <- p2p.PeerUpdate{
					Status: p2p.PeerStatusUp,
					NodeID: innerID,
				}
			}
		}
	}

	// wait till all suites (reactors) received all evidence from other suites (reactors)
	rts.waitForEvidence(t, evList)

	for _, pool := range rts.pools {
		require.Equal(t, numEvidence, int(pool.Size()))

		// commit state so we do not continue to repeat gossiping the same evidence
		state := pool.State()
		state.LastBlockHeight++
		pool.Update(state, evList)
	}
}

func TestReactorBroadcastEvidence_RemovePeer(t *testing.T) {
	val := types.NewMockPV()
	height := int64(10)

	stateDB1 := initializeValidatorState(t, val, height)
	stateDB2 := initializeValidatorState(t, val, height)

	rts := setup(t, []sm.Store{stateDB1, stateDB2}, uint(numEvidence))
	primary, secondary := rts.getPrimaryAndSecondary(t)
	// add all evidence to the primary reactor
	evList := createEvidenceList(t, rts.pools[primary.NodeID], val, numEvidence)

	// add the secondary reactor as a peer to the primary reactor
	rts.peerChans[primary.NodeID] <- p2p.PeerUpdate{
		Status: p2p.PeerStatusUp,
		NodeID: secondary.NodeID,
	}

	// have the secondary reactor receive only half the evidence
	rts.waitForEvidence(t, evList[:numEvidence/2], secondary.NodeID)

	// disconnect the peer
	rts.peerChans[primary.NodeID] <- p2p.PeerUpdate{
		Status: p2p.PeerStatusDown,
		NodeID: secondary.NodeID,
	}

	// Ensure the secondary only received half of the evidence before being
	// disconnected.
	require.Equal(t, numEvidence/2, int(rts.pools[secondary.NodeID].Size()))

	// The primary reactor should still be attempting to send the remaining half.
	//
	// NOTE: The channel is buffered (size numEvidence) as to ensure the primary
	// reactor will send all envelopes at once before receiving the signal to stop
	// gossiping.
	for i := 0; i < numEvidence/2; i++ {
		<-rts.dataChannels[primary.NodeID].In
	}

	// ensure all channels are drained
	for _, ech := range rts.dataChannels {
		require.Empty(t, ech)
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
