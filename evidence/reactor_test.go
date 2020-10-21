package evidence_test

import (
	"encoding/hex"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/go-kit/kit/log/term"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	dbm "github.com/tendermint/tm-db"

	cfg "github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/tmhash"
	"github.com/tendermint/tendermint/evidence"
	"github.com/tendermint/tendermint/evidence/mocks"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/p2p"
	ep "github.com/tendermint/tendermint/proto/tendermint/evidence"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	sm "github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/types"
)

// evidenceLogger is a TestingLogger which uses a different
// color for each validator ("validator" key must exist).
func evidenceLogger() log.Logger {
	return log.TestingLoggerWithColorFn(func(keyvals ...interface{}) term.FgBgColor {
		for i := 0; i < len(keyvals)-1; i += 2 {
			if keyvals[i] == "validator" {
				return term.FgBgColor{Fg: term.Color(uint8(keyvals[i+1].(int) + 1))}
			}
		}
		return term.FgBgColor{}
	})
}

// connect N evidence reactors through N switches
func makeAndConnectReactorsAndPools(config *cfg.Config, stateStores []sm.Store) ([]*evidence.Reactor,
	[]*evidence.Pool) {
	N := len(stateStores)

	reactors := make([]*evidence.Reactor, N)
	pools := make([]*evidence.Pool, N)
	logger := evidenceLogger()
	evidenceTime := time.Date(2019, 1, 1, 0, 0, 0, 0, time.UTC)

	for i := 0; i < N; i++ {
		evidenceDB := dbm.NewMemDB()
		blockStore := &mocks.BlockStore{}
		blockStore.On("LoadBlockMeta", mock.AnythingOfType("int64")).Return(
			&types.BlockMeta{Header: types.Header{Time: evidenceTime}},
		)
		pool, err := evidence.NewPool(evidenceDB, stateStores[i], blockStore)
		if err != nil {
			panic(err)
		}
		pools[i] = pool
		reactors[i] = evidence.NewReactor(pool)
		reactors[i].SetLogger(logger.With("validator", i))
	}

	p2p.MakeConnectedSwitches(config.P2P, N, func(i int, s *p2p.Switch) *p2p.Switch {
		s.AddReactor("EVIDENCE", reactors[i])
		return s

	}, p2p.Connect2Switches)

	return reactors, pools
}

// wait for all evidence on all reactors
func waitForEvidence(t *testing.T, evs types.EvidenceList, pools []*evidence.Pool) {
	// wait for the evidence in all evpools
	wg := new(sync.WaitGroup)
	for i := 0; i < len(pools); i++ {
		wg.Add(1)
		go _waitForEvidence(t, wg, evs, i, pools)
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	timer := time.After(timeout)
	select {
	case <-timer:
		t.Fatal("Timed out waiting for evidence")
	case <-done:
	}
}

// wait for all evidence on a single evpool
func _waitForEvidence(
	t *testing.T,
	wg *sync.WaitGroup,
	evs types.EvidenceList,
	poolIdx int,
	pools []*evidence.Pool,
) {
	evpool := pools[poolIdx]
	var evList []types.Evidence
	currentPoolSize := 0
	for currentPoolSize != len(evs) {
		evList, _ = evpool.PendingEvidence(int64(len(evs) * 500)) // each evidence should not be more than 500 bytes
		currentPoolSize = len(evList)
		time.Sleep(time.Millisecond * 100)
	}

	// put the reaped evidence in a map so we can quickly check we got everything
	evMap := make(map[string]types.Evidence)
	for _, e := range evList {
		evMap[string(e.Hash())] = e
	}
	for i, expectedEv := range evs {
		gotEv := evMap[string(expectedEv.Hash())]
		assert.Equal(t, expectedEv, gotEv,
			fmt.Sprintf("evidence at index %d on pool %d don't match: %v vs %v",
				i, poolIdx, expectedEv, gotEv))
	}

	wg.Done()
}

func sendEvidence(t *testing.T, evpool *evidence.Pool, val types.PrivValidator, n int) types.EvidenceList {
	evList := make([]types.Evidence, n)
	for i := 0; i < n; i++ {
		ev := types.NewMockDuplicateVoteEvidenceWithValidator(int64(i+1),
			time.Date(2019, 1, 1, 0, 0, 0, 0, time.UTC), val, evidenceChainID)
		err := evpool.AddEvidence(ev)
		require.NoError(t, err)
		evList[i] = ev
	}
	return evList
}

var (
	numEvidence = 10
	timeout     = 120 * time.Second // ridiculously high because CircleCI is slow
)

func TestReactorBroadcastEvidence(t *testing.T) {
	config := cfg.TestConfig()
	N := 7

	// create statedb for everyone
	stateDBs := make([]sm.Store, N)
	val := types.NewMockPV()
	// we need validators saved for heights at least as high as we have evidence for
	height := int64(numEvidence) + 10
	for i := 0; i < N; i++ {
		stateDBs[i] = initializeValidatorState(val, height)
	}

	// make reactors from statedb
	reactors, pools := makeAndConnectReactorsAndPools(config, stateDBs)

	// set the peer height on each reactor
	for _, r := range reactors {
		for _, peer := range r.Switch.Peers().List() {
			ps := peerState{height}
			peer.Set(types.PeerStateKey, ps)
		}
	}

	// send a bunch of valid evidence to the first reactor's evpool
	// and wait for them all to be received in the others
	evList := sendEvidence(t, pools[0], val, numEvidence)
	waitForEvidence(t, evList, pools)
}

type peerState struct {
	height int64
}

func (ps peerState) GetHeight() int64 {
	return ps.height
}

func TestReactorSelectiveBroadcast(t *testing.T) {
	config := cfg.TestConfig()

	val := types.NewMockPV()
	height1 := int64(numEvidence) + 10
	height2 := int64(numEvidence) / 2

	// DB1 is ahead of DB2
	stateDB1 := initializeValidatorState(val, height1)
	stateDB2 := initializeValidatorState(val, height2)

	// make reactors from statedb
	reactors, pools := makeAndConnectReactorsAndPools(config, []sm.Store{stateDB1, stateDB2})

	// set the peer height on each reactor
	for _, r := range reactors {
		for _, peer := range r.Switch.Peers().List() {
			ps := peerState{height1}
			peer.Set(types.PeerStateKey, ps)
		}
	}

	// update the first reactor peer's height to be very small
	peer := reactors[0].Switch.Peers().List()[0]
	ps := peerState{height2}
	peer.Set(types.PeerStateKey, ps)

	// send a bunch of valid evidence to the first reactor's evpool
	evList := sendEvidence(t, pools[0], val, numEvidence)

	// only ones less than the peers height should make it through
	waitForEvidence(t, evList[:numEvidence/2-1], pools[1:2])

	// peers should still be connected
	peers := reactors[1].Switch.Peers().List()
	assert.Equal(t, 1, len(peers))
}

func exampleVote(t byte) *types.Vote {
	var stamp, err = time.Parse(types.TimeFormat, "2017-12-25T03:00:01.234Z")
	if err != nil {
		panic(err)
	}

	return &types.Vote{
		Type:      tmproto.SignedMsgType(t),
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

// nolint:lll //ignore line length for tests
func TestEvidenceVectors(t *testing.T) {

	dupl := types.NewDuplicateVoteEvidence(exampleVote(1), exampleVote(2))

	testCases := []struct {
		testName     string
		evidenceList []types.Evidence
		expBytes     string
	}{
		{"DuplicateVoteEvidence", []types.Evidence{dupl}, "0af9010af6010a79080210031802224a0a208b01023386c371778ecb6368573e539afc3cc860ec3a2f614e54fe5652f4fc80122608c0843d122072db3d959635dff1bb567bedaa70573392c5159666a3f8caf11e413aac52207a2a0b08b1d381d20510809dca6f32146af1f4111082efb388211bc72c55bcd61e9ac3d538d5bb031279080110031802224a0a208b01023386c371778ecb6368573e539afc3cc860ec3a2f614e54fe5652f4fc80122608c0843d122072db3d959635dff1bb567bedaa70573392c5159666a3f8caf11e413aac52207a2a0b08b1d381d20510809dca6f32146af1f4111082efb388211bc72c55bcd61e9ac3d538d5bb03"},
	}

	for _, tc := range testCases {
		tc := tc

		evi := make([]*tmproto.Evidence, len(tc.evidenceList))
		for i := 0; i < len(tc.evidenceList); i++ {
			ev, err := types.EvidenceToProto(tc.evidenceList[i])
			require.NoError(t, err, tc.testName)
			evi[i] = ev
		}

		epl := ep.List{
			Evidence: evi,
		}

		bz, err := epl.Marshal()
		require.NoError(t, err, tc.testName)

		require.Equal(t, tc.expBytes, hex.EncodeToString(bz), tc.testName)

	}

}
