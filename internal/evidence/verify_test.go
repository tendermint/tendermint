package evidence_test

import (
	"context"
	"github.com/dashevo/dashd-go/btcjson"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"

	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/tmhash"
	"github.com/tendermint/tendermint/internal/evidence"
	"github.com/tendermint/tendermint/internal/evidence/mocks"
	sm "github.com/tendermint/tendermint/internal/state"
	smmocks "github.com/tendermint/tendermint/internal/state/mocks"
	"github.com/tendermint/tendermint/libs/log"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	"github.com/tendermint/tendermint/types"
)

type voteData struct {
	vote1 *types.Vote
	vote2 *types.Vote
	valid bool
}

func TestVerifyDuplicateVoteEvidence(t *testing.T) {
	val := types.NewMockPV()
	val2 := types.NewMockPV()
	quorumType := crypto.SmallQuorumType()
	quorumHash := crypto.RandQuorumHash()
	validator1 := val.ExtractIntoValidator(context.Background(), quorumHash)
	valSet := types.NewValidatorSet([]*types.Validator{validator1}, validator1.PubKey, quorumType, quorumHash, true)

	blockID := makeBlockID([]byte("blockhash"), 1000, []byte("partshash"))
	blockID2 := makeBlockID([]byte("blockhash2"), 1000, []byte("partshash"))
	blockID3 := makeBlockID([]byte("blockhash"), 10000, []byte("partshash"))
	blockID4 := makeBlockID([]byte("blockhash"), 10000, []byte("partshash2"))

	const chainID = "mychain"

	stateID := types.RandStateID()

	vote1 := makeVote(t, val, chainID, 0, 10, 2, 1, blockID, quorumType, quorumHash, stateID)
	v1 := vote1.ToProto()
	err := val.SignVote(context.Background(), chainID, crypto.SmallQuorumType(), quorumHash, v1, stateID, nil)
	require.NoError(t, err)
	badVote := makeVote(t, val, chainID, 0, 10, 2, 1, blockID, quorumType, quorumHash, stateID)
	bv := badVote.ToProto()
	err = val2.SignVote(context.Background(), chainID, crypto.SmallQuorumType(), quorumHash, bv, stateID, nil)
	require.NoError(t, err)

	vote1.BlockSignature = v1.BlockSignature
	vote1.StateSignature = v1.StateSignature
	badVote.BlockSignature = bv.BlockSignature
	badVote.StateSignature = bv.StateSignature

	cases := []voteData{
		{vote1, makeVote(t, val, chainID, 0, 10, 2, 1, blockID2, quorumType, quorumHash, stateID), true}, // different block ids
		{vote1, makeVote(t, val, chainID, 0, 10, 2, 1, blockID3, quorumType, quorumHash, stateID), true},
		{vote1, makeVote(t, val, chainID, 0, 10, 2, 1, blockID4, quorumType, quorumHash, stateID), true},
		{vote1, makeVote(t, val, chainID, 0, 10, 2, 1, blockID, quorumType, quorumHash, stateID), false},     // wrong block id
		{vote1, makeVote(t, val, "mychain2", 0, 10, 2, 1, blockID2, quorumType, quorumHash, stateID), false}, // wrong chain id
		{vote1, makeVote(t, val, chainID, 0, 11, 2, 1, blockID2, quorumType, quorumHash, stateID), false},    // wrong height
		{vote1, makeVote(t, val, chainID, 0, 10, 3, 1, blockID2, quorumType, quorumHash, stateID), false},    // wrong round
		{vote1, makeVote(t, val, chainID, 0, 10, 2, 2, blockID2, quorumType, quorumHash, stateID), false},    // wrong step
		{vote1, makeVote(t, val2, chainID, 0, 10, 2, 1, blockID2, quorumType, quorumHash, stateID), false},   // wrong validator
		{vote1, badVote, false}, // signed by wrong key
	}

	require.NoError(t, err)
	for _, c := range cases {
		ev := &types.DuplicateVoteEvidence{
			VoteA:            c.vote1,
			VoteB:            c.vote2,
			ValidatorPower:   1,
			TotalVotingPower: 1,
			Timestamp:        defaultEvidenceTime,
		}
		if c.valid {
			assert.Nil(t, evidence.VerifyDuplicateVote(ev, chainID, valSet), "evidence should be valid")
		} else {
			assert.NotNil(t, evidence.VerifyDuplicateVote(ev, chainID, valSet), "evidence should be invalid")
		}
	}

	// create good evidence and correct validator power
	goodEv, err := types.NewMockDuplicateVoteEvidenceWithValidator(10, defaultEvidenceTime, val, chainID, crypto.SmallQuorumType(), quorumHash)
	require.NoError(t, err)
	goodEv.ValidatorPower = 1
	goodEv.TotalVotingPower = 1
	badEv, err := types.NewMockDuplicateVoteEvidenceWithValidator(10, defaultEvidenceTime, val, chainID, crypto.SmallQuorumType(), quorumHash)
	require.NoError(t, err)
	badTimeEv, err :=  types.NewMockDuplicateVoteEvidenceWithValidator(10, defaultEvidenceTime.Add(1*time.Minute), val, chainID, crypto.SmallQuorumType(), quorumHash)
	require.NoError(t, err)
	badTimeEv.ValidatorPower = 1
	badTimeEv.TotalVotingPower = 1
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

	pool, err := evidence.NewPool(log.TestingLogger(), dbm.NewMemDB(), stateStore, blockStore)
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
	t *testing.T, val types.PrivValidator, chainID string, valIndex int32, height int64,
	round int32, step int, blockID types.BlockID, quorumType btcjson.LLMQType, quorumHash crypto.QuorumHash, stateID types.StateID) *types.Vote {
	proTxHash, err := val.GetProTxHash(context.Background())
	require.NoError(t, err)
	v := &types.Vote{
		ValidatorProTxHash: proTxHash,
		ValidatorIndex:   valIndex,
		Height:           height,
		Round:            round,
		Type:             tmproto.SignedMsgType(step),
		BlockID:          blockID,
	}

	vpb := v.ToProto()
	err = val.SignVote(context.Background(), chainID, quorumType, quorumHash, vpb, stateID, nil)
	if err != nil {
		panic(err)
	}
	v.BlockSignature = vpb.BlockSignature
	return v
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
