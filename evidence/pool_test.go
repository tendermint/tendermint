package evidence

import (
	"os"
	"testing"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	dbm "github.com/tendermint/tm-db"

	"github.com/tendermint/tendermint/crypto/tmhash"
	"github.com/tendermint/tendermint/evidence/mocks"
	"github.com/tendermint/tendermint/libs/bytes"
	"github.com/tendermint/tendermint/libs/log"
	tmrand "github.com/tendermint/tendermint/libs/rand"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	sm "github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/store"
	"github.com/tendermint/tendermint/types"
	tmtime "github.com/tendermint/tendermint/types/time"
)

func TestMain(m *testing.M) {

	code := m.Run()
	os.Exit(code)
}

const evidenceChainID = "test_chain"

func TestEvidencePool(t *testing.T) {
	var (
		val          = types.NewMockPV()
		height       = int64(52)
		stateDB      = initializeValidatorState(val, height)
		evidenceDB   = dbm.NewMemDB()
		blockStore   = &mocks.BlockStore{}
		evidenceTime = time.Date(2019, 1, 1, 0, 0, 0, 0, time.UTC)

		goodEvidence = types.NewMockDuplicateVoteEvidenceWithValidator(height, evidenceTime, val, evidenceChainID)
		badEvidence  = types.NewMockDuplicateVoteEvidenceWithValidator(1, evidenceTime, val, evidenceChainID)
	)

	blockStore.On("LoadBlockMeta", mock.AnythingOfType("int64")).Return(
		&types.BlockMeta{Header: types.Header{Time: evidenceTime}},
	)

	pool, err := NewPool(stateDB, evidenceDB, blockStore)
	require.NoError(t, err)

	// bad evidence
	err = pool.AddEvidence(badEvidence)
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "is too old; min height is 32 and evidence can not be older than")
	}
	assert.False(t, pool.IsPending(badEvidence))
	assert.True(t, pool.IsEvidenceExpired(badEvidence))

	// good evidence
	evAdded := make(chan struct{})
	go func() {
		<-pool.EvidenceWaitChan()
		close(evAdded)
	}()

	err = pool.AddEvidence(goodEvidence)
	require.NoError(t, err)

	select {
	case <-evAdded:
	case <-time.After(5 * time.Second):
		t.Fatal("evidence was not added to list after 5s")
	}

	assert.Equal(t, 1, pool.evidenceList.Len())

	// if we send it again, it shouldnt add and return an error
	err = pool.AddEvidence(goodEvidence)
	assert.NoError(t, err)
	assert.Equal(t, 1, pool.evidenceList.Len())
}

func TestProposingAndCommittingEvidence(t *testing.T) {
	var (
		val          = types.NewMockPV()
		height       = int64(1)
		stateDB      = initializeValidatorState(val, height)
		evidenceDB   = dbm.NewMemDB()
		blockStore   = &mocks.BlockStore{}
		evidenceTime = time.Date(2019, 1, 1, 0, 0, 0, 0, time.UTC)
	)

	blockStore.On("LoadBlockMeta", mock.AnythingOfType("int64")).Return(
		&types.BlockMeta{Header: types.Header{Time: evidenceTime}},
	)

	pool, err := NewPool(stateDB, evidenceDB, blockStore)
	require.NoError(t, err)

	// evidence not seen yet:
	evidence := types.NewMockDuplicateVoteEvidenceWithValidator(height, evidenceTime, val, evidenceChainID)
	assert.False(t, pool.IsCommitted(evidence))

	// evidence seen but not yet committed:
	assert.NoError(t, pool.AddEvidence(evidence))
	assert.False(t, pool.IsCommitted(evidence))

	// test evidence is proposed
	proposedEvidence := pool.AllPendingEvidence()
	assert.Equal(t, proposedEvidence[0], evidence)

	// evidence seen and committed:
	pool.MarkEvidenceAsCommitted(height, proposedEvidence)
	assert.True(t, pool.IsCommitted(evidence))
	assert.False(t, pool.IsPending(evidence))
	assert.Equal(t, 0, pool.evidenceList.Len())

	// evidence should
}

func TestAddEvidence(t *testing.T) {
	var (
		val          = types.NewMockPV()
		valAddr      = val.PrivKey.PubKey().Address()
		height       = int64(30)
		stateDB      = initializeValidatorState(val, height)
		evidenceDB   = dbm.NewMemDB()
		blockStoreDB = dbm.NewMemDB()
		blockStore   = initializeBlockStore(blockStoreDB, sm.LoadState(stateDB), valAddr)
		evidenceTime = time.Date(2019, 1, 1, 0, 0, 0, 0, time.UTC)
	)

	pool, err := NewPool(stateDB, evidenceDB, blockStore)
	require.NoError(t, err)

	testCases := []struct {
		evHeight      int64
		evTime        time.Time
		expErr        bool
		evDescription string
	}{
		{height, time.Now(), false, "valid evidence"},
		{height, evidenceTime, false, "valid evidence (despite old time)"},
		{int64(1), time.Now(), false, "valid evidence (despite old height)"},
		{int64(1), evidenceTime, true,
			"evidence from height 1 (created at: 2019-01-01 00:00:00 +0000 UTC) is too old"},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.evDescription, func(t *testing.T) {
			ev := types.NewMockDuplicateVoteEvidence(tc.evHeight, tc.evTime, evidenceChainID)
			err := pool.AddEvidence(ev)
			if tc.expErr {
				assert.Error(t, err)
				t.Log(err)
			}
		})
	}
}

func TestEvidencePoolUpdate(t *testing.T) {
	var (
		val          = types.NewMockPV()
		valAddr      = val.PrivKey.PubKey().Address()
		height       = int64(21)
		stateDB      = initializeValidatorState(val, height)
		evidenceDB   = dbm.NewMemDB()
		blockStoreDB = dbm.NewMemDB()
		state        = sm.LoadState(stateDB)
		blockStore   = initializeBlockStore(blockStoreDB, state, valAddr)
	)

	pool, err := NewPool(stateDB, evidenceDB, blockStore)
	require.NoError(t, err)

	// create new block (no need to save it to blockStore)
	evidence := types.NewMockDuplicateVoteEvidence(height, time.Now(), evidenceChainID)
	lastCommit := makeCommit(height, valAddr)
	block := types.MakeBlock(height+1, []types.Tx{}, lastCommit, []types.Evidence{evidence})
	// update state (partially)
	state.LastBlockHeight = height + 1

	pool.Update(block, state)

	// a) Update marks evidence as committed
	assert.True(t, pool.IsCommitted(evidence))
}

func TestAddingAndPruningPOLC(t *testing.T) {
	var (
		val          = types.NewMockPV()
		valAddr      = val.PrivKey.PubKey().Address()
		stateDB      = initializeValidatorState(val, 1)
		evidenceDB   = dbm.NewMemDB()
		blockStoreDB = dbm.NewMemDB()
		state        = sm.LoadState(stateDB)
		blockStore   = initializeBlockStore(blockStoreDB, state, valAddr)
		height       = state.ConsensusParams.Evidence.MaxAgeNumBlocks * 2
		evidenceTime = time.Date(2019, 1, 1, 0, 0, 0, 0, time.UTC)
		firstBlockID = types.BlockID{
			Hash: tmrand.Bytes(tmhash.Size),
			PartSetHeader: types.PartSetHeader{
				Total: 1,
				Hash:  tmrand.Bytes(tmhash.Size),
			},
		}
	)

	voteA := makeVote(1, 1, 0, val.PrivKey.PubKey().Address(), firstBlockID, evidenceTime)
	vA := voteA.ToProto()
	err := val.SignVote(evidenceChainID, vA)
	require.NoError(t, err)
	voteA.Signature = vA.Signature

	pubKey, _ := types.NewMockPV().GetPubKey()
	polc := &types.ProofOfLockChange{
		Votes:  []*types.Vote{voteA},
		PubKey: pubKey,
	}

	pool, err := NewPool(stateDB, evidenceDB, blockStore)
	require.NoError(t, err)

	err = pool.AddPOLC(polc)
	assert.NoError(t, err)

	// should be able to retrieve polc
	newPolc, err := pool.RetrievePOLC(1, 1)
	assert.NoError(t, err)
	assert.True(t, polc.Equal(newPolc))

	// should not be able to retrieve because it doesn't exist
	emptyPolc, err := pool.RetrievePOLC(2, 1)
	assert.NoError(t, err)
	assert.Nil(t, emptyPolc)

	lastCommit := makeCommit(height-1, valAddr)
	block := types.MakeBlock(height, []types.Tx{}, lastCommit, []types.Evidence{})
	// update state (partially)
	state.LastBlockHeight = height
	pool.state.LastBlockHeight = height

	// update should prune the polc
	pool.Update(block, state)

	emptyPolc, err = pool.RetrievePOLC(1, 1)
	assert.NoError(t, err)
	assert.Nil(t, emptyPolc)

}

func TestRecoverPendingEvidence(t *testing.T) {
	var (
		val             = types.NewMockPV()
		valAddr         = val.PrivKey.PubKey().Address()
		height          = int64(30)
		stateDB         = initializeValidatorState(val, height)
		evidenceDB      = dbm.NewMemDB()
		blockStoreDB    = dbm.NewMemDB()
		state           = sm.LoadState(stateDB)
		blockStore      = initializeBlockStore(blockStoreDB, state, valAddr)
		evidenceTime    = time.Date(2019, 1, 1, 0, 0, 0, 0, time.UTC)
		goodEvidence    = types.NewMockDuplicateVoteEvidenceWithValidator(height, time.Now(), val, evidenceChainID)
		expiredEvidence = types.NewMockDuplicateVoteEvidenceWithValidator(int64(1), evidenceTime, val, evidenceChainID)
	)

	// load good evidence
	goodKey := keyPending(goodEvidence)
	evi, err := types.EvidenceToProto(goodEvidence)
	require.NoError(t, err)
	goodEvidenceBytes, err := proto.Marshal(evi)
	require.NoError(t, err)
	_ = evidenceDB.Set(goodKey, goodEvidenceBytes)

	// load expired evidence
	expiredKey := keyPending(expiredEvidence)
	eevi, err := types.EvidenceToProto(expiredEvidence)
	require.NoError(t, err)

	expiredEvidenceBytes, err := proto.Marshal(eevi)
	require.NoError(t, err)

	_ = evidenceDB.Set(expiredKey, expiredEvidenceBytes)
	pool, err := NewPool(stateDB, evidenceDB, blockStore)
	require.NoError(t, err)
	assert.Equal(t, 1, pool.evidenceList.Len())
	assert.True(t, pool.IsPending(goodEvidence))
}

// Comprehensive set of test cases relating to the adding, upgrading and overall
// processing of PotentialAmnesiaEvidence and AmnesiaEvidence
func TestAddingPotentialAmnesiaEvidence(t *testing.T) {
	var (
		val     = types.NewMockPV()
		val2    = types.NewMockPV()
		pubKey  = val.PrivKey.PubKey()
		pubKey2 = val2.PrivKey.PubKey()
		valSet  = &types.ValidatorSet{
			Validators: []*types.Validator{
				val.ExtractIntoValidator(1),
				val2.ExtractIntoValidator(3),
			},
			Proposer: val.ExtractIntoValidator(1),
		}
		height     = int64(30)
		stateDB    = initializeStateFromValidatorSet(valSet, height)
		evidenceDB = dbm.NewMemDB()
		state      = sm.LoadState(stateDB)
		blockStore = &mocks.BlockStore{}
		//evidenceTime    = time.Date(2019, 1, 1, 0, 0, 0, 0, time.UTC)
		firstBlockID = types.BlockID{
			Hash: []byte("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"),
			PartSetHeader: types.PartSetHeader{
				Total: 1,
				Hash:  []byte("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"),
			},
		}
		secondBlockID = types.BlockID{
			Hash: []byte("bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"),
			PartSetHeader: types.PartSetHeader{
				Total: 1,
				Hash:  []byte("bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"),
			},
		}
		evidenceTime = time.Date(2019, 1, 1, 0, 0, 0, 0, time.UTC)
	)

	blockStore.On("LoadBlockMeta", mock.AnythingOfType("int64")).Return(
		&types.BlockMeta{Header: types.Header{Time: evidenceTime}},
	)

	// TEST SETUP
	pool, err := NewPool(stateDB, evidenceDB, blockStore)
	require.NoError(t, err)

	pool.SetLogger(log.TestingLogger())

	voteA := makeVote(height, 0, 0, pubKey.Address(), firstBlockID, evidenceTime)
	vA := voteA.ToProto()
	err = val.SignVote(evidenceChainID, vA)
	voteA.Signature = vA.Signature
	require.NoError(t, err)
	voteB := makeVote(height, 1, 0, pubKey.Address(), secondBlockID, evidenceTime.Add(3*time.Second))
	vB := voteB.ToProto()
	err = val.SignVote(evidenceChainID, vB)
	voteB.Signature = vB.Signature
	require.NoError(t, err)
	voteC := makeVote(height, 2, 0, pubKey.Address(), firstBlockID, evidenceTime.Add(2*time.Second))
	vC := voteC.ToProto()
	err = val.SignVote(evidenceChainID, vC)
	voteC.Signature = vC.Signature
	require.NoError(t, err)
	ev := &types.PotentialAmnesiaEvidence{
		VoteA:     voteA,
		VoteB:     voteB,
		Timestamp: evidenceTime,
	}

	polc := &types.ProofOfLockChange{
		Votes:  []*types.Vote{voteB},
		PubKey: pubKey2,
	}
	err = pool.AddPOLC(polc)
	require.NoError(t, err)

	polc, err = pool.RetrievePOLC(height, 1)
	require.NoError(t, err)
	require.NotEmpty(t, polc)

	secondValVote := makeVote(height, 1, 0, pubKey2.Address(), secondBlockID, evidenceTime.Add(1*time.Second))
	vv2 := secondValVote.ToProto()
	err = val2.SignVote(evidenceChainID, vv2)
	require.NoError(t, err)
	secondValVote.Signature = vv2.Signature

	validPolc := &types.ProofOfLockChange{
		Votes:  []*types.Vote{secondValVote},
		PubKey: pubKey,
	}

	// CASE A
	pool.logger.Info("CASE A")
	// we expect the evidence pool to find the polc but log an error as the polc is not valid -> vote was
	// not from a validator in this set. However, an error isn't thrown because the evidence pool
	// should still be able to save the regular potential amnesia evidence.
	err = pool.AddEvidence(ev)
	assert.NoError(t, err)

	// evidence requires trial period until it is available -> we expect no evidence to be returned
	assert.Equal(t, 0, len(pool.PendingEvidence(1)))
	assert.True(t, pool.IsOnTrial(ev))

	nextHeight := pool.nextEvidenceTrialEndedHeight
	assert.Greater(t, nextHeight, int64(0))

	// CASE B
	pool.logger.Info("CASE B")
	// evidence is not ready to be upgraded so we return the height we expect the evidence to be.
	nextHeight = pool.upgradePotentialAmnesiaEvidence()
	assert.Equal(t, height+pool.state.ConsensusParams.Evidence.ProofTrialPeriod, nextHeight)

	// CASE C
	pool.logger.Info("CASE C")
	// now evidence is ready to be upgraded to amnesia evidence -> we expect -1 to be the next height as their is
	// no more pending potential amnesia evidence left
	lastCommit := makeCommit(height+1, pubKey.Address())
	block := types.MakeBlock(height+2, []types.Tx{}, lastCommit, []types.Evidence{})
	state.LastBlockHeight = height + 2

	pool.Update(block, state)
	assert.Equal(t, int64(-1), pool.nextEvidenceTrialEndedHeight)

	assert.Equal(t, 1, len(pool.PendingEvidence(1)))

	// CASE D
	pool.logger.Info("CASE D")
	// evidence of voting back in the past which is instantly punishable -> amnesia evidence is made directly
	ev2 := &types.PotentialAmnesiaEvidence{
		VoteA:     voteC,
		VoteB:     voteB,
		Timestamp: evidenceTime,
	}
	err = pool.AddEvidence(ev2)
	assert.NoError(t, err)
	expectedAe := &types.AmnesiaEvidence{
		PotentialAmnesiaEvidence: ev2,
		Polc:                     types.NewEmptyPOLC(),
	}

	assert.True(t, pool.IsPending(expectedAe))
	assert.Equal(t, 2, len(pool.AllPendingEvidence()))

	// CASE E
	pool.logger.Info("CASE E")
	// test for receiving amnesia evidence
	ae := types.NewAmnesiaEvidence(ev, types.NewEmptyPOLC())
	// we need to run the trial period ourselves so amnesia evidence should not be added, instead
	// we should extract out the potential amnesia evidence and trying to add that before realising
	// that we already have it -> no error
	err = pool.AddEvidence(ae)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(pool.AllPendingEvidence()))

	voteD := makeVote(height, 2, 0, pubKey.Address(), firstBlockID, evidenceTime.Add(4*time.Second))
	vD := voteD.ToProto()
	err = val.SignVote(evidenceChainID, vD)
	require.NoError(t, err)
	voteD.Signature = vD.Signature

	// CASE F
	pool.logger.Info("CASE F")
	// a new amnesia evidence is seen. It has an empty polc so we should extract the potential amnesia evidence
	// and start our own trial
	newPe := &types.PotentialAmnesiaEvidence{
		VoteA:     voteB,
		VoteB:     voteD,
		Timestamp: evidenceTime,
	}
	newAe := &types.AmnesiaEvidence{
		PotentialAmnesiaEvidence: newPe,
		Polc:                     types.NewEmptyPOLC(),
	}
	err = pool.AddEvidence(newAe)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(pool.AllPendingEvidence()))
	assert.True(t, pool.IsOnTrial(newPe))

	// CASE G
	pool.logger.Info("CASE G")
	// Finally, we receive an amnesia evidence containing a valid polc for an earlier potential amnesia evidence
	// that we have already upgraded to. We should ad this new amnesia evidence in replace of the prior
	// amnesia evidence with an empty polc that we have
	aeWithPolc := &types.AmnesiaEvidence{
		PotentialAmnesiaEvidence: ev,
		Polc:                     validPolc,
	}
	err = pool.AddEvidence(aeWithPolc)
	assert.NoError(t, err)
	assert.True(t, pool.IsPending(aeWithPolc))
	assert.Equal(t, 2, len(pool.AllPendingEvidence()))
	t.Log(pool.AllPendingEvidence())

}

func initializeStateFromValidatorSet(valSet *types.ValidatorSet, height int64) dbm.DB {
	stateDB := dbm.NewMemDB()
	state := sm.State{
		ChainID:                     evidenceChainID,
		LastBlockHeight:             height,
		LastBlockTime:               tmtime.Now(),
		Validators:                  valSet,
		NextValidators:              valSet.CopyIncrementProposerPriority(1),
		LastValidators:              valSet,
		LastHeightValidatorsChanged: 1,
		ConsensusParams: tmproto.ConsensusParams{
			Block: tmproto.BlockParams{
				MaxBytes: 22020096,
				MaxGas:   -1,
			},
			Evidence: tmproto.EvidenceParams{
				MaxAgeNumBlocks:  20,
				MaxAgeDuration:   48 * time.Hour,
				MaxNum:           50,
				ProofTrialPeriod: 1,
			},
		},
	}

	// save all states up to height
	for i := int64(0); i <= height; i++ {
		state.LastBlockHeight = i
		sm.SaveState(stateDB, state)
	}

	return stateDB
}

func initializeValidatorState(privVal types.PrivValidator, height int64) dbm.DB {

	pubKey, _ := privVal.GetPubKey()
	validator := &types.Validator{Address: pubKey.Address(), VotingPower: 0, PubKey: pubKey}

	// create validator set and state
	valSet := &types.ValidatorSet{
		Validators: []*types.Validator{validator},
		Proposer:   validator,
	}

	return initializeStateFromValidatorSet(valSet, height)
}

// initializeBlockStore creates a block storage and populates it w/ a dummy
// block at +height+.
func initializeBlockStore(db dbm.DB, state sm.State, valAddr []byte) *store.BlockStore {
	blockStore := store.NewBlockStore(db)

	for i := int64(1); i <= state.LastBlockHeight; i++ {
		lastCommit := makeCommit(i-1, valAddr)
		block, _ := state.MakeBlock(i, []types.Tx{}, lastCommit, nil,
			state.Validators.GetProposer().Address)

		const parts = 1
		partSet := block.MakePartSet(parts)

		seenCommit := makeCommit(i, valAddr)
		blockStore.SaveBlock(block, partSet, seenCommit)
	}

	return blockStore
}

func makeCommit(height int64, valAddr []byte) *types.Commit {
	commitSigs := []types.CommitSig{{
		BlockIDFlag:      types.BlockIDFlagCommit,
		ValidatorAddress: valAddr,
		Timestamp:        time.Now(),
		Signature:        []byte("Signature"),
	}}
	return types.NewCommit(height, 0, types.BlockID{}, commitSigs)
}

func makeVote(height int64, round, index int32, addr bytes.HexBytes,
	blockID types.BlockID, time time.Time) *types.Vote {
	return &types.Vote{
		Type:             tmproto.SignedMsgType(2),
		Height:           height,
		Round:            round,
		BlockID:          blockID,
		Timestamp:        time,
		ValidatorAddress: addr,
		ValidatorIndex:   index,
	}
}
