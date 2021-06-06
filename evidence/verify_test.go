package evidence_test

import (
	"github.com/dashevo/dashd-go/btcjson"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	dbm "github.com/tendermint/tm-db"

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

	conflictingVals, conflictingPrivVals := types.GenerateValidatorSet(3)

	commonVals := types.NewValidatorSet(conflictingVals.Validators[0:2], conflictingVals.ThresholdPublicKey,
		conflictingVals.QuorumType, conflictingVals.QuorumHash, true)

	commonHeader := makeHeaderRandom(4)
	commonHeader.Time = defaultEvidenceTime
	trustedHeader := makeHeaderRandom(10)
	trustedHeader.Time = defaultEvidenceTime.Add(1 * time.Hour)

	conflictingHeader := makeHeaderRandom(10)
	conflictingHeader.Time = defaultEvidenceTime.Add(1 * time.Hour)
	conflictingHeader.ValidatorsHash = conflictingVals.Hash()

	// we are simulating a lunatic light client attack
	blockID := makeBlockID(conflictingHeader.Hash(), 1000, []byte("partshash"))
	stateID := makeStateID(conflictingHeader.AppHash)
	voteSet := types.NewVoteSet(evidenceChainID, 10, 1, tmproto.SignedMsgType(2), conflictingVals)
	commit, err := types.MakeCommit(blockID, stateID, 10, 1, voteSet, conflictingPrivVals)
	require.NoError(t, err)
	ev := &types.LightClientAttackEvidence{
		ConflictingBlock: &types.LightBlock{
			SignedHeader: &types.SignedHeader{
				Header: conflictingHeader,
				Commit: commit,
			},
			ValidatorSet: conflictingVals,
		},
		CommonHeight:        4,
		TotalVotingPower:    2 * types.DefaultDashVotingPower,
		ByzantineValidators: commonVals.Validators,
		Timestamp:           defaultEvidenceTime,
	}

	commonSignedHeader := &types.SignedHeader{
		Header: commonHeader,
		Commit: &types.Commit{},
	}
	trustedBlockID := makeBlockID(trustedHeader.Hash(), 1000, []byte("partshash"))
	trustedStateID := makeStateID(trustedHeader.AppHash)
	vals, privVals := types.GenerateValidatorSet(3)
	trustedVoteSet := types.NewVoteSet(evidenceChainID, 10, 1, tmproto.SignedMsgType(2), vals)
	trustedCommit, err := types.MakeCommit(trustedBlockID, trustedStateID, 10, 1, trustedVoteSet, privVals)
	require.NoError(t, err)
	trustedSignedHeader := &types.SignedHeader{
		Header: trustedHeader,
		Commit: trustedCommit,
	}

	// good pass -> no error
	err = evidence.VerifyLightClientAttack(ev, commonSignedHeader, trustedSignedHeader, commonVals,
		defaultEvidenceTime.Add(2*time.Hour), 3*time.Hour)
	assert.NoError(t, err)

	// trusted and conflicting hashes are the same -> an error should be returned
	err = evidence.VerifyLightClientAttack(ev, commonSignedHeader, ev.ConflictingBlock.SignedHeader, commonVals,
		defaultEvidenceTime.Add(2*time.Hour), 3*time.Hour)
	assert.Error(t, err)

	// evidence with different total validator power should fail
	ev.TotalVotingPower = types.DefaultDashVotingPower + 1
	err = evidence.VerifyLightClientAttack(ev, commonSignedHeader, trustedSignedHeader, commonVals,
		defaultEvidenceTime.Add(2*time.Hour), 3*time.Hour)
	assert.Error(t, err)
	ev.TotalVotingPower = 2 * types.DefaultDashVotingPower

	forwardConflictingHeader := makeHeaderRandom(11)
	forwardConflictingHeader.Time = defaultEvidenceTime.Add(30 * time.Minute)
	forwardConflictingHeader.ValidatorsHash = conflictingVals.Hash()
	forwardBlockID := makeBlockID(forwardConflictingHeader.Hash(), 1000, []byte("partshash"))
	forwardStateID := makeStateID(forwardConflictingHeader.AppHash)
	forwardVoteSet := types.NewVoteSet(evidenceChainID, 11, 1, tmproto.SignedMsgType(2), conflictingVals)
	forwardCommit, err := types.MakeCommit(forwardBlockID, forwardStateID, 11, 1, forwardVoteSet, conflictingPrivVals)
	require.NoError(t, err)
	forwardLunaticEv := &types.LightClientAttackEvidence{
		ConflictingBlock: &types.LightBlock{
			SignedHeader: &types.SignedHeader{
				Header: forwardConflictingHeader,
				Commit: forwardCommit,
			},
			ValidatorSet: conflictingVals,
		},
		CommonHeight:        4,
		TotalVotingPower:    200,
		ByzantineValidators: commonVals.Validators,
		Timestamp:           defaultEvidenceTime,
	}
	err = evidence.VerifyLightClientAttack(forwardLunaticEv, commonSignedHeader, trustedSignedHeader, commonVals,
		defaultEvidenceTime.Add(2*time.Hour), 3*time.Hour)
	assert.NoError(t, err)

	state := sm.State{
		LastBlockTime:   defaultEvidenceTime.Add(2 * time.Hour),
		LastBlockHeight: 11,
		ConsensusParams: *types.DefaultConsensusParams(),
	}
	stateStore := &smmocks.Store{}
	stateStore.On("LoadValidators", int64(4)).Return(commonVals, nil)
	stateStore.On("Load").Return(state, nil)
	blockStore := &mocks.BlockStore{}
	blockStore.On("LoadBlockMeta", int64(4)).Return(&types.BlockMeta{Header: *commonHeader})
	blockStore.On("LoadBlockMeta", int64(10)).Return(&types.BlockMeta{Header: *trustedHeader})
	blockStore.On("LoadBlockMeta", int64(11)).Return(nil)
	blockStore.On("LoadBlockCommit", int64(4)).Return(commit)
	blockStore.On("LoadBlockCommit", int64(10)).Return(trustedCommit)
	blockStore.On("Height").Return(int64(10))

	pool, err := evidence.NewPool(dbm.NewMemDB(), stateStore, blockStore)
	require.NoError(t, err)
	pool.SetLogger(log.TestingLogger())

	evList := types.EvidenceList{ev}
	err = pool.CheckEvidence(evList)
	assert.NoError(t, err)

	pendingEvs, _ := pool.PendingEvidence(state.ConsensusParams.Evidence.MaxBytes)
	assert.Equal(t, 1, len(pendingEvs))

	// if we submit evidence only against a single byzantine validator when we see there are more validators then this
	// should return an error
	ev.ByzantineValidators = []*types.Validator{commonVals.Validators[0]}
	err = pool.CheckEvidence(evList)
	assert.Error(t, err)
	ev.ByzantineValidators = commonVals.Validators // restore evidence

	// If evidence is submitted with an altered timestamp it should return an error
	ev.Timestamp = defaultEvidenceTime.Add(1 * time.Minute)
	err = pool.CheckEvidence(evList)
	assert.Error(t, err)

	evList = types.EvidenceList{forwardLunaticEv}
	err = pool.CheckEvidence(evList)
	assert.NoError(t, err)
}

func TestVerifyLightClientAttack_Equivocation(t *testing.T) {
	conflictingVals, conflictingPrivVals := types.GenerateValidatorSet(5)
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
	stateID := makeStateID(conflictingHeader.AppHash)
	voteSet := types.NewVoteSet(evidenceChainID, 10, 1, tmproto.SignedMsgType(2), conflictingVals)
	commit, err := types.MakeCommit(blockID, stateID, 10, 1, voteSet, conflictingPrivVals[:4])
	require.NoError(t, err)
	ev := &types.LightClientAttackEvidence{
		ConflictingBlock: &types.LightBlock{
			SignedHeader: &types.SignedHeader{
				Header: conflictingHeader,
				Commit: commit,
			},
			ValidatorSet: conflictingVals,
		},
		CommonHeight:        10,
		ByzantineValidators: conflictingVals.Validators[:4],
		TotalVotingPower:    5 * types.DefaultDashVotingPower,
		Timestamp:           defaultEvidenceTime,
	}

	trustedBlockID := makeBlockID(trustedHeader.Hash(), 1000, []byte("partshash"))
	trustedStateID := makeStateID(trustedHeader.AppHash)
	trustedVoteSet := types.NewVoteSet(evidenceChainID, 10, 1, tmproto.SignedMsgType(2), conflictingVals)
	trustedCommit, err := types.MakeCommit(trustedBlockID, trustedStateID, 10, 1, trustedVoteSet, conflictingPrivVals)
	require.NoError(t, err)
	trustedSignedHeader := &types.SignedHeader{
		Header: trustedHeader,
		Commit: trustedCommit,
	}

	// good pass -> no error
	err = evidence.VerifyLightClientAttack(ev, trustedSignedHeader, trustedSignedHeader, conflictingVals,
		defaultEvidenceTime.Add(1*time.Minute), 2*time.Hour)
	assert.NoError(t, err)

	// trusted and conflicting hashes are the same -> an error should be returned
	err = evidence.VerifyLightClientAttack(ev, trustedSignedHeader, ev.ConflictingBlock.SignedHeader, conflictingVals,
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

	pendingEvs, _ := pool.PendingEvidence(state.ConsensusParams.Evidence.MaxBytes)
	assert.Equal(t, 1, len(pendingEvs))
}

func TestVerifyLightClientAttack_Amnesia(t *testing.T) {
	conflictingVals, conflictingPrivVals := types.GenerateValidatorSet(5)

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
	stateID := makeStateID(conflictingHeader.AppHash)
	voteSet := types.NewVoteSet(evidenceChainID, 10, 0, tmproto.SignedMsgType(2), conflictingVals)
	commit, err := types.MakeCommit(blockID, stateID, 10, 0, voteSet, conflictingPrivVals)
	require.NoError(t, err)
	ev := &types.LightClientAttackEvidence{
		ConflictingBlock: &types.LightBlock{
			SignedHeader: &types.SignedHeader{
				Header: conflictingHeader,
				Commit: commit,
			},
			ValidatorSet: conflictingVals,
		},
		CommonHeight:        10,
		ByzantineValidators: nil, // with amnesia evidence no validators are submitted as abci evidence
		TotalVotingPower:    5 * types.DefaultDashVotingPower,
		Timestamp:           defaultEvidenceTime,
	}

	trustedBlockID := makeBlockID(trustedHeader.Hash(), 1000, []byte("partshash"))
	trustedStateID := makeStateID(trustedHeader.AppHash)
	trustedVoteSet := types.NewVoteSet(evidenceChainID, 10, 1, tmproto.SignedMsgType(2), conflictingVals)
	trustedCommit, err := types.MakeCommit(trustedBlockID, trustedStateID, 10, 1, trustedVoteSet, conflictingPrivVals)
	require.NoError(t, err)
	trustedSignedHeader := &types.SignedHeader{
		Header: trustedHeader,
		Commit: trustedCommit,
	}

	// good pass -> no error
	err = evidence.VerifyLightClientAttack(ev, trustedSignedHeader, trustedSignedHeader, conflictingVals,
		defaultEvidenceTime.Add(1*time.Minute), 2*time.Hour)
	assert.NoError(t, err)

	// trusted and conflicting hashes are the same -> an error should be returned
	err = evidence.VerifyLightClientAttack(ev, trustedSignedHeader, ev.ConflictingBlock.SignedHeader, conflictingVals,
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

	pendingEvs, _ := pool.PendingEvidence(state.ConsensusParams.Evidence.MaxBytes)
	assert.Equal(t, 1, len(pendingEvs))
}

type voteData struct {
	vote1 *types.Vote
	vote2 *types.Vote
	valid bool
}

func TestVerifyDuplicateVoteEvidence(t *testing.T) {
	val := types.NewMockPV()
	val2 := types.NewMockPV()
	randQuorumHash := crypto.RandQuorumHash()
	quorumType := btcjson.LLMQType_5_60
	valSet := types.NewValidatorSet([]*types.Validator{val.ExtractIntoValidator(0, randQuorumHash)},
		val.PrivKey.PubKey(), quorumType, randQuorumHash, true)

	blockID := makeBlockID([]byte("blockhash"), 1000, []byte("partshash"))
	blockID2 := makeBlockID([]byte("blockhash2"), 1000, []byte("partshash"))
	blockID3 := makeBlockID([]byte("blockhash"), 10000, []byte("partshash"))
	blockID4 := makeBlockID([]byte("blockhash"), 10000, []byte("partshash2"))

	stateID := makeStateID([]byte("lastapphash"))

	const chainID = "mychain"

	vote1 := makeVote(t, val, chainID, quorumType, randQuorumHash, 0, 10, 2, 1, blockID, stateID)
	v1 := vote1.ToProto()
	err := val.SignVote(chainID, quorumType, randQuorumHash, v1)
	require.NoError(t, err)
	badVote := makeVote(t, val, chainID, quorumType, randQuorumHash, 0, 10, 2, 1, blockID, stateID)
	bv := badVote.ToProto()
	err = val2.SignVote(chainID, quorumType, randQuorumHash, bv)
	require.NoError(t, err)

	vote1.BlockSignature = v1.BlockSignature
	badVote.BlockSignature = bv.BlockSignature

	cases := []voteData{
		{vote1, makeVote(t, val, chainID, quorumType, randQuorumHash, 0, 10, 2, 1, blockID2, stateID), true}, // different block ids
		{vote1, makeVote(t, val, chainID, quorumType, randQuorumHash, 0, 10, 2, 1, blockID3, stateID), true},
		{vote1, makeVote(t, val, chainID, quorumType, randQuorumHash, 0, 10, 2, 1, blockID4, stateID), true},
		{vote1, makeVote(t, val, chainID, quorumType, randQuorumHash, 0, 10, 2, 1, blockID, stateID), false},     // wrong block id
		{vote1, makeVote(t, val, "mychain2", quorumType, randQuorumHash, 0, 10, 2, 1, blockID2, stateID), false}, // wrong chain id
		{vote1, makeVote(t, val, chainID, quorumType, randQuorumHash, 0, 11, 2, 1, blockID2, stateID), false},    // wrong height
		{vote1, makeVote(t, val, chainID, quorumType, randQuorumHash, 0, 10, 3, 1, blockID2, stateID), false},    // wrong round
		{vote1, makeVote(t, val, chainID, quorumType, randQuorumHash, 0, 10, 2, 2, blockID2, stateID), false},    // wrong step
		{vote1, makeVote(t, val2, chainID, quorumType, randQuorumHash, 0, 10, 2, 1, blockID2, stateID), false},   // wrong validator
		{vote1, badVote, false}, // signed by wrong key
	}

	require.NoError(t, err)
	for _, c := range cases {
		ev := &types.DuplicateVoteEvidence{
			VoteA:            c.vote1,
			VoteB:            c.vote2,
			ValidatorPower:   types.DefaultDashVotingPower,
			TotalVotingPower: types.DefaultDashVotingPower,
			Timestamp:        defaultEvidenceTime,
		}
		if c.valid {
			assert.Nil(t, evidence.VerifyDuplicateVote(ev, chainID, valSet), "evidence should be valid")
		} else {
			assert.NotNil(t, evidence.VerifyDuplicateVote(ev, chainID, valSet), "evidence should be invalid")
		}
	}

	// create good evidence and correct validator power
	goodEv := types.NewMockDuplicateVoteEvidenceWithValidator(10, defaultEvidenceTime, val, chainID, quorumType, randQuorumHash)
	goodEv.ValidatorPower = types.DefaultDashVotingPower
	goodEv.TotalVotingPower = types.DefaultDashVotingPower
	badEv := types.NewMockDuplicateVoteEvidenceWithValidator(10, defaultEvidenceTime, val, chainID, quorumType, randQuorumHash)
	badEv.ValidatorPower = types.DefaultDashVotingPower + 1
	badEv.TotalVotingPower = types.DefaultDashVotingPower
	badTimeEv := types.NewMockDuplicateVoteEvidenceWithValidator(10, defaultEvidenceTime.Add(1*time.Minute), val, chainID, quorumType, randQuorumHash)
	badTimeEv.ValidatorPower = types.DefaultDashVotingPower
	badTimeEv.TotalVotingPower = types.DefaultDashVotingPower
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

	// evidence with a different validator power should fail
	evList = types.EvidenceList{badEv}
	err = pool.CheckEvidence(evList)
	assert.Error(t, err)

	// evidence with a different timestamp should fail
	evList = types.EvidenceList{badTimeEv}
	err = pool.CheckEvidence(evList)
	assert.Error(t, err)
}

func makeVote(
	t *testing.T, val types.PrivValidator, chainID string, quorumType btcjson.LLMQType, quorumHash crypto.QuorumHash, valIndex int32, height int64,
	round int32, step int, blockID types.BlockID, stateID types.StateID) *types.Vote {
	proTxHash, err := val.GetProTxHash()
	require.NoError(t, err)
	v := &types.Vote{
		ValidatorProTxHash: proTxHash,
		ValidatorIndex:     valIndex,
		Height:             height,
		Round:              round,
		Type:               tmproto.SignedMsgType(step),
		BlockID:            blockID,
		StateID:            stateID,
	}

	vpb := v.ToProto()
	err = val.SignVote(chainID, quorumType, quorumHash, vpb)
	if err != nil {
		panic(err)
	}
	v.BlockSignature = vpb.BlockSignature
	v.StateSignature = vpb.StateSignature
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
		ProposerProTxHash:  crypto.CRandBytes(crypto.DefaultHashSize),
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

func makeStateID(lastAppHash []byte) types.StateID {
	var (
		ah = make([]byte, tmhash.Size)
	)
	copy(ah, lastAppHash)
	return types.StateID{
		LastAppHash: ah,
	}
}
