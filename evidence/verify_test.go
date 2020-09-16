package evidence_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	dbm "github.com/tendermint/tm-db"

	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/tmhash"
	"github.com/tendermint/tendermint/evidence"
	"github.com/tendermint/tendermint/evidence/mocks"
	"github.com/tendermint/tendermint/libs/log"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	tmversion "github.com/tendermint/tendermint/proto/tendermint/version"
	sm "github.com/tendermint/tendermint/state"
	smmocks "github.com/tendermint/tendermint/state/mocks"
	"github.com/tendermint/tendermint/types"
	"github.com/tendermint/tendermint/version"
)

func TestVerifyLightClientAttack_Lunatic(t *testing.T) {
	commonVals, commonPrivVals := types.RandValidatorSet(2, 10)

	newVal, newPrivVal := types.RandValidator(false, 9)

	conflictingVals, err := types.ValidatorSetFromExistingValidators(append(commonVals.Validators, newVal))
	require.NoError(t, err)
	conflictingPrivVals := append(commonPrivVals, newPrivVal)

	commonHeader := makeHeaderRandom(4)
	commonHeader.Time = defaultEvidenceTime.Add(-1 * time.Hour)
	trustedHeader := makeHeaderRandom(10)

	conflictingHeader := makeHeaderRandom(10)
	conflictingHeader.ValidatorsHash = conflictingVals.Hash()

	// we are simulating a duplicate vote attack where all the validators in the conflictingVals set
	// vote twice
	blockID := makeBlockID(conflictingHeader.Hash(), 1000, []byte("partshash"))
	voteSet := types.NewVoteSet(evidenceChainID, 10, 1, tmproto.SignedMsgType(2), conflictingVals)
	commit, err := types.MakeCommit(blockID, 10, 1, voteSet, conflictingPrivVals, defaultEvidenceTime)
	require.NoError(t, err)
	ev := &types.LightClientAttackEvidence{
		ConflictingBlock: &types.LightBlock{
			SignedHeader: &types.SignedHeader{
				Header: conflictingHeader,
				Commit: commit,
			},
			ValidatorSet: conflictingVals,
		},
		CommonHeight: 4,
	}

	commonSignedHeader := &types.SignedHeader{
		Header: commonHeader,
		Commit: &types.Commit{},
	}
	trustedBlockID := makeBlockID(trustedHeader.Hash(), 1000, []byte("partshash"))
	vals, privVals := types.RandValidatorSet(3, 8)
	trustedVoteSet := types.NewVoteSet(evidenceChainID, 10, 1, tmproto.SignedMsgType(2), vals)
	trustedCommit, err := types.MakeCommit(trustedBlockID, 10, 1, trustedVoteSet, privVals, defaultEvidenceTime)
	require.NoError(t, err)
	trustedSignedHeader := &types.SignedHeader{
		Header: trustedHeader,
		Commit: trustedCommit,
	}

	// good pass -> no error
	err = evidence.VerifyLightClientAttack(ev, commonSignedHeader, trustedSignedHeader, commonVals,
		defaultEvidenceTime.Add(1*time.Minute), 2*time.Hour)
	assert.NoError(t, err)

	// trusted and conflicting hashes are the same -> an error should be returned
	err = evidence.VerifyLightClientAttack(ev, commonSignedHeader, ev.ConflictingBlock.SignedHeader, commonVals,
		defaultEvidenceTime.Add(1*time.Minute), 2*time.Hour)
	assert.Error(t, err)

	state := sm.State{
		LastBlockTime:   defaultEvidenceTime.Add(1 * time.Minute),
		LastBlockHeight: 11,
		ConsensusParams: *types.DefaultConsensusParams(),
	}
	stateStore := &smmocks.Store{}
	stateStore.On("LoadValidators", int64(4)).Return(commonVals, nil)
	stateStore.On("Load").Return(state, nil)
	blockStore := &mocks.BlockStore{}
	blockStore.On("LoadBlockMeta", int64(4)).Return(&types.BlockMeta{Header: *commonHeader})
	blockStore.On("LoadBlockMeta", int64(10)).Return(&types.BlockMeta{Header: *trustedHeader})
	blockStore.On("LoadBlockCommit", int64(4)).Return(commit)
	blockStore.On("LoadBlockCommit", int64(10)).Return(trustedCommit)

	pool, err := evidence.NewPool(dbm.NewMemDB(), stateStore, blockStore)
	require.NoError(t, err)
	pool.SetLogger(log.TestingLogger())

	evList := types.EvidenceList{ev}
	err = pool.CheckEvidence(evList)
	assert.NoError(t, err)

	pendingEvs := pool.PendingEvidence(2)
	assert.Equal(t, 1, len(pendingEvs))

	pubKey, err := newPrivVal.GetPubKey()
	require.NoError(t, err)
	lastCommit := makeCommit(state.LastBlockHeight, pubKey.Address())
	block := types.MakeBlock(state.LastBlockHeight, []types.Tx{}, lastCommit, []types.Evidence{ev})

	abciEv := pool.ABCIEvidence(block.Height, block.Evidence.Evidence)
	expectedAbciEv := make([]abci.Evidence, len(commonVals.Validators))

	// we expect evidence to be made for all validators in the common validator set
	for idx, val := range commonVals.Validators {
		ev := abci.Evidence{
			Type:             abci.EvidenceType_LIGHT_CLIENT_ATTACK,
			Validator:        types.TM2PB.Validator(val),
			Height:           commonHeader.Height,
			Time:             commonHeader.Time,
			TotalVotingPower: commonVals.TotalVotingPower(),
		}
		expectedAbciEv[idx] = ev
	}

	assert.Equal(t, expectedAbciEv, abciEv)
}

func TestVerifyLightClientAttack_Equivocation(t *testing.T) {
	conflictingVals, conflictingPrivVals := types.RandValidatorSet(5, 10)
	trustedHeader := makeHeaderRandom(10)

	conflictingHeader := makeHeaderRandom(10)
	conflictingHeader.ValidatorsHash = conflictingVals.Hash()

	trustedHeader.ValidatorsHash = conflictingHeader.ValidatorsHash
	trustedHeader.NextValidatorsHash = conflictingHeader.NextValidatorsHash
	trustedHeader.ConsensusHash = conflictingHeader.ConsensusHash
	trustedHeader.AppHash = conflictingHeader.AppHash
	trustedHeader.LastResultsHash = conflictingHeader.LastResultsHash

	// we are simulating a duplicate vote attack where all the validators in the conflictingVals set
	// except the last validator vote twice
	blockID := makeBlockID(conflictingHeader.Hash(), 1000, []byte("partshash"))
	voteSet := types.NewVoteSet(evidenceChainID, 10, 1, tmproto.SignedMsgType(2), conflictingVals)
	commit, err := types.MakeCommit(blockID, 10, 1, voteSet, conflictingPrivVals[:4], defaultEvidenceTime)
	require.NoError(t, err)
	ev := &types.LightClientAttackEvidence{
		ConflictingBlock: &types.LightBlock{
			SignedHeader: &types.SignedHeader{
				Header: conflictingHeader,
				Commit: commit,
			},
			ValidatorSet: conflictingVals,
		},
		CommonHeight: 10,
	}

	trustedBlockID := makeBlockID(trustedHeader.Hash(), 1000, []byte("partshash"))
	trustedVoteSet := types.NewVoteSet(evidenceChainID, 10, 1, tmproto.SignedMsgType(2), conflictingVals)
	trustedCommit, err := types.MakeCommit(trustedBlockID, 10, 1, trustedVoteSet, conflictingPrivVals, defaultEvidenceTime)
	require.NoError(t, err)
	trustedSignedHeader := &types.SignedHeader{
		Header: trustedHeader,
		Commit: trustedCommit,
	}

	// good pass -> no error
	err = evidence.VerifyLightClientAttack(ev, trustedSignedHeader, trustedSignedHeader, nil,
		defaultEvidenceTime.Add(1*time.Minute), 2*time.Hour)
	assert.NoError(t, err)

	// trusted and conflicting hashes are the same -> an error should be returned
	err = evidence.VerifyLightClientAttack(ev, trustedSignedHeader, ev.ConflictingBlock.SignedHeader, nil,
		defaultEvidenceTime.Add(1*time.Minute), 2*time.Hour)
	assert.Error(t, err)

	// conflicting header has different next validators hash which should have been correctly derived from
	// the previous round
	ev.ConflictingBlock.Header.NextValidatorsHash = crypto.CRandBytes(tmhash.Size)
	err = evidence.VerifyLightClientAttack(ev, trustedSignedHeader, trustedSignedHeader, nil,
		defaultEvidenceTime.Add(1*time.Minute), 2*time.Hour)
	assert.Error(t, err)
	// revert next validators hash
	ev.ConflictingBlock.Header.NextValidatorsHash = trustedHeader.NextValidatorsHash

	state := sm.State{
		LastBlockTime:   defaultEvidenceTime.Add(1 * time.Minute),
		LastBlockHeight: 11,
		ConsensusParams: *types.DefaultConsensusParams(),
	}
	stateStore := &smmocks.Store{}
	stateStore.On("LoadValidators", int64(10)).Return(conflictingVals, nil)
	stateStore.On("Load").Return(state, nil)
	blockStore := &mocks.BlockStore{}
	blockStore.On("LoadBlockMeta", int64(10)).Return(&types.BlockMeta{Header: *trustedHeader})
	blockStore.On("LoadBlockCommit", int64(10)).Return(trustedCommit)

	pool, err := evidence.NewPool(dbm.NewMemDB(), stateStore, blockStore)
	require.NoError(t, err)
	pool.SetLogger(log.TestingLogger())

	evList := types.EvidenceList{ev}
	err = pool.CheckEvidence(evList)
	assert.NoError(t, err)

	pendingEvs := pool.PendingEvidence(2)
	assert.Equal(t, 1, len(pendingEvs))

	pubKey, err := conflictingPrivVals[0].GetPubKey()
	require.NoError(t, err)
	lastCommit := makeCommit(state.LastBlockHeight, pubKey.Address())
	block := types.MakeBlock(state.LastBlockHeight, []types.Tx{}, lastCommit, []types.Evidence{ev})

	abciEv := pool.ABCIEvidence(block.Height, block.Evidence.Evidence)
	expectedAbciEv := make([]abci.Evidence, len(conflictingVals.Validators)-1)

	// we expect evidence to be made for all validators except the last one
	for idx, val := range conflictingVals.Validators {
		if idx == 4 { // skip the last validator
			continue
		}
		ev := abci.Evidence{
			Type:             abci.EvidenceType_LIGHT_CLIENT_ATTACK,
			Validator:        types.TM2PB.Validator(val),
			Height:           ev.ConflictingBlock.Height,
			Time:             ev.ConflictingBlock.Time,
			TotalVotingPower: ev.ConflictingBlock.ValidatorSet.TotalVotingPower(),
		}
		expectedAbciEv[idx] = ev
	}

	assert.Equal(t, expectedAbciEv, abciEv)
}

func TestVerifyLightClientAttack_Amnesia(t *testing.T) {
	conflictingVals, conflictingPrivVals := types.RandValidatorSet(5, 10)

	conflictingHeader := makeHeaderRandom(10)
	conflictingHeader.ValidatorsHash = conflictingVals.Hash()
	trustedHeader := makeHeaderRandom(10)
	trustedHeader.ValidatorsHash = conflictingHeader.ValidatorsHash
	trustedHeader.NextValidatorsHash = conflictingHeader.NextValidatorsHash
	trustedHeader.AppHash = conflictingHeader.AppHash
	trustedHeader.ConsensusHash = conflictingHeader.ConsensusHash
	trustedHeader.LastResultsHash = conflictingHeader.LastResultsHash

	// we are simulating an amnesia attack where all the validators in the conflictingVals set
	// except the last validator vote twice. However this time the commits are of different rounds.
	blockID := makeBlockID(conflictingHeader.Hash(), 1000, []byte("partshash"))
	voteSet := types.NewVoteSet(evidenceChainID, 10, 0, tmproto.SignedMsgType(2), conflictingVals)
	commit, err := types.MakeCommit(blockID, 10, 0, voteSet, conflictingPrivVals, defaultEvidenceTime)
	require.NoError(t, err)
	ev := &types.LightClientAttackEvidence{
		ConflictingBlock: &types.LightBlock{
			SignedHeader: &types.SignedHeader{
				Header: conflictingHeader,
				Commit: commit,
			},
			ValidatorSet: conflictingVals,
		},
		CommonHeight: 10,
	}

	trustedBlockID := makeBlockID(trustedHeader.Hash(), 1000, []byte("partshash"))
	trustedVoteSet := types.NewVoteSet(evidenceChainID, 10, 1, tmproto.SignedMsgType(2), conflictingVals)
	trustedCommit, err := types.MakeCommit(trustedBlockID, 10, 1, trustedVoteSet, conflictingPrivVals, defaultEvidenceTime)
	require.NoError(t, err)
	trustedSignedHeader := &types.SignedHeader{
		Header: trustedHeader,
		Commit: trustedCommit,
	}

	// good pass -> no error
	err = evidence.VerifyLightClientAttack(ev, trustedSignedHeader, trustedSignedHeader, nil,
		defaultEvidenceTime.Add(1*time.Minute), 2*time.Hour)
	assert.NoError(t, err)

	// trusted and conflicting hashes are the same -> an error should be returned
	err = evidence.VerifyLightClientAttack(ev, trustedSignedHeader, ev.ConflictingBlock.SignedHeader, nil,
		defaultEvidenceTime.Add(1*time.Minute), 2*time.Hour)
	assert.Error(t, err)

	state := sm.State{
		LastBlockTime:   defaultEvidenceTime.Add(1 * time.Minute),
		LastBlockHeight: 11,
		ConsensusParams: *types.DefaultConsensusParams(),
	}
	stateStore := &smmocks.Store{}
	stateStore.On("LoadValidators", int64(10)).Return(conflictingVals, nil)
	stateStore.On("Load").Return(state, nil)
	blockStore := &mocks.BlockStore{}
	blockStore.On("LoadBlockMeta", int64(10)).Return(&types.BlockMeta{Header: *trustedHeader})
	blockStore.On("LoadBlockCommit", int64(10)).Return(trustedCommit)

	pool, err := evidence.NewPool(dbm.NewMemDB(), stateStore, blockStore)
	require.NoError(t, err)
	pool.SetLogger(log.TestingLogger())

	evList := types.EvidenceList{ev}
	err = pool.CheckEvidence(evList)
	assert.NoError(t, err)

	pendingEvs := pool.PendingEvidence(2)
	assert.Equal(t, 1, len(pendingEvs))

	pubKey, err := conflictingPrivVals[0].GetPubKey()
	require.NoError(t, err)
	lastCommit := makeCommit(state.LastBlockHeight, pubKey.Address())
	block := types.MakeBlock(state.LastBlockHeight, []types.Tx{}, lastCommit, []types.Evidence{ev})

	abciEv := pool.ABCIEvidence(block.Height, block.Evidence.Evidence)
	// as we are unable to find out which subset of validators in the commit were malicious, no information
	// is sent to the application. We expect the array to be empty
	emptyEvidenceBlock := types.MakeBlock(state.LastBlockHeight, []types.Tx{}, lastCommit, []types.Evidence{})
	expectedAbciEv := pool.ABCIEvidence(emptyEvidenceBlock.Height, emptyEvidenceBlock.Evidence.Evidence)

	assert.Equal(t, expectedAbciEv, abciEv)
}

type voteData struct {
	vote1 *types.Vote
	vote2 *types.Vote
	valid bool
}

func TestVerifyDuplicateVoteEvidence(t *testing.T) {
	val := types.NewMockPV()
	val2 := types.NewMockPV()
	valSet := types.NewValidatorSet([]*types.Validator{val.ExtractIntoValidator(1)})

	blockID := makeBlockID([]byte("blockhash"), 1000, []byte("partshash"))
	blockID2 := makeBlockID([]byte("blockhash2"), 1000, []byte("partshash"))
	blockID3 := makeBlockID([]byte("blockhash"), 10000, []byte("partshash"))
	blockID4 := makeBlockID([]byte("blockhash"), 10000, []byte("partshash2"))

	const chainID = "mychain"

	vote1 := makeVote(t, val, chainID, 0, 10, 2, 1, blockID, defaultEvidenceTime)
	v1 := vote1.ToProto()
	err := val.SignVote(chainID, v1)
	require.NoError(t, err)
	badVote := makeVote(t, val, chainID, 0, 10, 2, 1, blockID, defaultEvidenceTime)
	bv := badVote.ToProto()
	err = val2.SignVote(chainID, bv)
	require.NoError(t, err)

	vote1.Signature = v1.Signature
	badVote.Signature = bv.Signature

	cases := []voteData{
		{vote1, makeVote(t, val, chainID, 0, 10, 2, 1, blockID2, defaultEvidenceTime), true}, // different block ids
		{vote1, makeVote(t, val, chainID, 0, 10, 2, 1, blockID3, defaultEvidenceTime), true},
		{vote1, makeVote(t, val, chainID, 0, 10, 2, 1, blockID4, defaultEvidenceTime), true},
		{vote1, makeVote(t, val, chainID, 0, 10, 2, 1, blockID, defaultEvidenceTime), false},     // wrong block id
		{vote1, makeVote(t, val, "mychain2", 0, 10, 2, 1, blockID2, defaultEvidenceTime), false}, // wrong chain id
		{vote1, makeVote(t, val, chainID, 0, 11, 2, 1, blockID2, defaultEvidenceTime), false},    // wrong height
		{vote1, makeVote(t, val, chainID, 0, 10, 3, 1, blockID2, defaultEvidenceTime), false},    // wrong round
		{vote1, makeVote(t, val, chainID, 0, 10, 2, 2, blockID2, defaultEvidenceTime), false},    // wrong step
		{vote1, makeVote(t, val2, chainID, 0, 10, 2, 1, blockID2, defaultEvidenceTime), false},   // wrong validator
		// a different vote time doesn't matter
		{vote1, makeVote(t, val, chainID, 0, 10, 2, 1, blockID2, time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)), true},
		{vote1, badVote, false}, // signed by wrong key
	}

	require.NoError(t, err)
	for _, c := range cases {
		ev := &types.DuplicateVoteEvidence{
			VoteA: c.vote1,
			VoteB: c.vote2,
		}
		if c.valid {
			assert.Nil(t, evidence.VerifyDuplicateVote(ev, chainID, valSet), "evidence should be valid")
		} else {
			assert.NotNil(t, evidence.VerifyDuplicateVote(ev, chainID, valSet), "evidence should be invalid")
		}
	}

	goodEv := types.NewMockDuplicateVoteEvidenceWithValidator(10, defaultEvidenceTime, val, chainID)
	state := sm.State{
		ChainID:         chainID,
		LastBlockTime:   defaultEvidenceTime.Add(1 * time.Minute),
		LastBlockHeight: 11,
		ConsensusParams: *types.DefaultConsensusParams(),
	}
	stateStore := &smmocks.Store{}
	stateStore.On("LoadValidators", int64(10)).Return(valSet, nil)
	stateStore.On("Load").Return(state, nil)
	blockStore := &mocks.BlockStore{}
	blockStore.On("LoadBlockMeta", int64(10)).Return(&types.BlockMeta{Header: types.Header{Time: defaultEvidenceTime}})

	pool, err := evidence.NewPool(dbm.NewMemDB(), stateStore, blockStore)
	require.NoError(t, err)

	evList := types.EvidenceList{goodEv}
	err = pool.CheckEvidence(evList)
	assert.NoError(t, err)
}

func makeVote(
	t *testing.T, val types.PrivValidator, chainID string, valIndex int32, height int64,
	round int32, step int, blockID types.BlockID, time time.Time) *types.Vote {
	pubKey, err := val.GetPubKey()
	require.NoError(t, err)
	v := &types.Vote{
		ValidatorAddress: pubKey.Address(),
		ValidatorIndex:   valIndex,
		Height:           height,
		Round:            round,
		Type:             tmproto.SignedMsgType(step),
		BlockID:          blockID,
		Timestamp:        time,
	}

	vpb := v.ToProto()
	err = val.SignVote(chainID, vpb)
	if err != nil {
		panic(err)
	}
	v.Signature = vpb.Signature
	return v
}

func makeHeaderRandom(height int64) *types.Header {
	return &types.Header{
		Version:            tmversion.Consensus{Block: version.BlockProtocol, App: 1},
		ChainID:            evidenceChainID,
		Height:             height,
		Time:               defaultEvidenceTime,
		LastBlockID:        makeBlockID([]byte("headerhash"), 1000, []byte("partshash")),
		LastCommitHash:     crypto.CRandBytes(tmhash.Size),
		DataHash:           crypto.CRandBytes(tmhash.Size),
		ValidatorsHash:     crypto.CRandBytes(tmhash.Size),
		NextValidatorsHash: crypto.CRandBytes(tmhash.Size),
		ConsensusHash:      crypto.CRandBytes(tmhash.Size),
		AppHash:            crypto.CRandBytes(tmhash.Size),
		LastResultsHash:    crypto.CRandBytes(tmhash.Size),
		EvidenceHash:       crypto.CRandBytes(tmhash.Size),
		ProposerAddress:    crypto.CRandBytes(crypto.AddressSize),
	}
}

func makeBlockID(hash []byte, partSetSize uint32, partSetHash []byte) types.BlockID {
	var (
		h   = make([]byte, tmhash.Size)
		psH = make([]byte, tmhash.Size)
	)
	copy(h, hash)
	copy(psH, partSetHash)
	return types.BlockID{
		Hash: h,
		PartSetHeader: types.PartSetHeader{
			Total: partSetSize,
			Hash:  psH,
		},
	}
}
