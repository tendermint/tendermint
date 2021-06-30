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
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	tmversion "github.com/tendermint/tendermint/proto/tendermint/version"
	sm "github.com/tendermint/tendermint/state"
	smmocks "github.com/tendermint/tendermint/state/mocks"
	"github.com/tendermint/tendermint/types"
	"github.com/tendermint/tendermint/version"
)

type voteData struct {
	vote1 *types.Vote
	vote2 *types.Vote
	valid bool
}

func TestVerifyDuplicateVoteEvidence(t *testing.T) {
	quorumHash := crypto.RandQuorumHash()
	val := types.NewMockPVForQuorum(quorumHash)
	val2 := types.NewMockPVForQuorum(quorumHash)
	quorumType := btcjson.LLMQType_5_60
	pubKey, err := val.GetPubKey(quorumHash)
	require.NoError(t, err)
	valSet := types.NewValidatorSet([]*types.Validator{val.ExtractIntoValidator(quorumHash)},
		pubKey, quorumType, quorumHash, true)

	blockID := makeBlockID([]byte("blockhash"), 1000, []byte("partshash"))
	blockID2 := makeBlockID([]byte("blockhash2"), 1000, []byte("partshash"))
	blockID3 := makeBlockID([]byte("blockhash"), 10000, []byte("partshash"))
	blockID4 := makeBlockID([]byte("blockhash"), 10000, []byte("partshash2"))

	stateID := makeStateID([]byte("lastapphash"))

	const chainID = "mychain"

	vote1 := makeVote(t, val, chainID, quorumType, quorumHash, 0, 10, 2, 1, blockID, stateID)
	v1 := vote1.ToProto()
	err = val.SignVote(chainID, quorumType, quorumHash, v1)
	require.NoError(t, err)
	badVote := makeVote(t, val, chainID, quorumType, quorumHash, 0, 10, 2, 1, blockID, stateID)
	bv := badVote.ToProto()
	err = val2.SignVote(chainID, quorumType, quorumHash, bv)
	require.NoError(t, err)

	vote1.BlockSignature = v1.BlockSignature
	badVote.BlockSignature = bv.BlockSignature

	cases := []voteData{
		{vote1, makeVote(t, val, chainID, quorumType, quorumHash, 0, 10, 2, 1, blockID2, stateID), true}, // different block ids
		{vote1, makeVote(t, val, chainID, quorumType, quorumHash, 0, 10, 2, 1, blockID3, stateID), true},
		{vote1, makeVote(t, val, chainID, quorumType, quorumHash, 0, 10, 2, 1, blockID4, stateID), true},
		{vote1, makeVote(t, val, chainID, quorumType, quorumHash, 0, 10, 2, 1, blockID, stateID), false},     // wrong block id
		{vote1, makeVote(t, val, "mychain2", quorumType, quorumHash, 0, 10, 2, 1, blockID2, stateID), false}, // wrong chain id
		{vote1, makeVote(t, val, chainID, quorumType, quorumHash, 0, 11, 2, 1, blockID2, stateID), false},    // wrong height
		{vote1, makeVote(t, val, chainID, quorumType, quorumHash, 0, 10, 3, 1, blockID2, stateID), false},    // wrong round
		{vote1, makeVote(t, val, chainID, quorumType, quorumHash, 0, 10, 2, 2, blockID2, stateID), false},    // wrong step
		{vote1, makeVote(t, val2, chainID, quorumType, quorumHash, 0, 10, 2, 1, blockID2, stateID), false},   // wrong validator
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
	goodEv := types.NewMockDuplicateVoteEvidenceWithValidator(10, defaultEvidenceTime, val, chainID, quorumType, quorumHash)
	goodEv.ValidatorPower = types.DefaultDashVotingPower
	goodEv.TotalVotingPower = types.DefaultDashVotingPower
	badEv := types.NewMockDuplicateVoteEvidenceWithValidator(10, defaultEvidenceTime, val, chainID, quorumType, quorumHash)
	badEv.ValidatorPower = types.DefaultDashVotingPower + 1
	badEv.TotalVotingPower = types.DefaultDashVotingPower
	badTimeEv := types.NewMockDuplicateVoteEvidenceWithValidator(10, defaultEvidenceTime.Add(1*time.Minute), val, chainID, quorumType, quorumHash)
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
