package light_test

import (
	"testing"
	"time"

	"github.com/dashevo/dashd-go/btcjson"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/tendermint/tendermint/crypto"
	provider_mocks "github.com/tendermint/tendermint/light/provider/mocks"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	"github.com/tendermint/tendermint/types"
	"github.com/tendermint/tendermint/version"
)

// privKeys is a helper type for testing.
//
// It lets us simulate signing with many keys.  The main use case is to create
// a set, and call GenSignedHeader to get properly signed header for testing.
//
// You can set different weights of validators each time you call ToValidators,
// and can optionally extend the validator set later with Extend.
type privKeys []crypto.PrivKey

func exposeMockPVKeys(pvs []types.PrivValidator, quorumHash crypto.QuorumHash) privKeys {
	res := make(privKeys, len(pvs))
	for i, pval := range pvs {
		mockPV := pval.(*types.MockPV)
		res[i] = mockPV.PrivateKeys[quorumHash.String()].PrivKey
	}
	return res
}

// ToValidators produces a valset from the set of keys.
// The first key has weight `init` and it increases by `inc` every step
// so we can have all the same weight, or a simple linear distribution
// (should be enough for testing).
func (pkz privKeys) ToValidators(thresholdPublicKey crypto.PubKey) *types.ValidatorSet {
	res := make([]*types.Validator, len(pkz))

	for i, k := range pkz {
		res[i] = types.NewValidatorDefaultVotingPower(k.PubKey(), crypto.Checksum(k.PubKey().Address()))
	}

	// Quorum hash is pseudorandom
	return types.NewValidatorSet(
		res,
		thresholdPublicKey,
		btcjson.LLMQType_5_60,
		crypto.Checksum(thresholdPublicKey.Bytes()),
		true,
	)
}

// signHeader properly signs the header with all keys from first to last exclusive.
func (pkz privKeys) signHeader(t testing.TB, header *types.Header, valSet *types.ValidatorSet, first, last int) *types.Commit {
	t.Helper()

	blockID := types.BlockID{
		Hash:          header.Hash(),
		PartSetHeader: types.PartSetHeader{Total: 1, Hash: crypto.CRandBytes(32)},
	}

	stateID := types.StateID{
		Height:      header.Height - 1,
		LastAppHash: header.AppHash,
	}

	votes := make([]*types.Vote, len(pkz))
	// Fill in the votes we want.
	for i := first; i < last && i < len(pkz); i++ {
		// Verify that the private key matches the validator proTxHash
		privateKey := pkz[i]
		proTxHash, val := valSet.GetByIndex(int32(i))
		if val == nil {
			panic("no val")
		}
		if privateKey == nil {
			panic("no priv key")
		}
		if !privateKey.PubKey().Equals(val.PubKey) {
			panic("light client keys do not match")
		}
		votes[i] = makeVote(t, header, valSet, proTxHash, pkz[i], blockID, stateID)
	}
	thresholdSigns, err := types.NewSignsRecoverer(votes).Recover()
	require.NoError(t, err)
	quorumSigns := &types.CommitSigns{
		QuorumSigns: *thresholdSigns,
		QuorumHash:  valSet.QuorumHash,
	}
	return types.NewCommit(header.Height, 1, blockID, stateID, quorumSigns)
}

func makeVote(t testing.TB, header *types.Header, valset *types.ValidatorSet, proTxHash crypto.ProTxHash,
	key crypto.PrivKey, blockID types.BlockID, stateID types.StateID) *types.Vote {
	t.Helper()

	idx, val := valset.GetByProTxHash(proTxHash)
	if val == nil {
		panic("val must exist")
	}
	vote := &types.Vote{
		ValidatorProTxHash: proTxHash,
		ValidatorIndex:     idx,
		Height:             header.Height,
		Round:              1,
		Type:               tmproto.PrecommitType,
		BlockID:            blockID,
	}

	v := vote.ToProto()
	// SignDigest the vote
	signID := types.VoteBlockSignID(header.ChainID, v, valset.QuorumType, valset.QuorumHash)
	sig, err := key.SignDigest(signID)
	if err != nil {
		panic(err)
	}

	// SignDigest the state
	stateSignID := stateID.SignID(header.ChainID, valset.QuorumType, valset.QuorumHash)
	sigState, err := key.SignDigest(stateSignID)
	if err != nil {
		panic(err)
	}

	vote.BlockSignature = sig
	vote.StateSignature = sigState

	return vote
}

func genHeader(chainID string, height int64, bTime time.Time, txs types.Txs,
	valset, nextValset *types.ValidatorSet, appHash, consHash, resHash []byte) *types.Header {

	return &types.Header{
		Version: version.Consensus{Block: version.BlockProtocol, App: 0},
		ChainID: chainID,
		Height:  height,
		Time:    bTime,
		// LastBlockID
		// LastCommitHash
		ValidatorsHash:     valset.Hash(),
		NextValidatorsHash: nextValset.Hash(),
		DataHash:           txs.Hash(),
		AppHash:            appHash,
		ConsensusHash:      consHash,
		LastResultsHash:    resHash,
		ProposerProTxHash:  valset.Validators[0].ProTxHash,
	}
}

// GenSignedHeader calls genHeader and signHeader and combines them into a SignedHeader.
func (pkz privKeys) GenSignedHeader(t testing.TB, chainID string, height int64, bTime time.Time, txs types.Txs,
	valset, nextValset *types.ValidatorSet, appHash, consHash, resHash []byte, first, last int) *types.SignedHeader {

	t.Helper()

	header := genHeader(chainID, height, bTime, txs, valset, nextValset, appHash, consHash, resHash)
	return &types.SignedHeader{
		Header: header,
		Commit: pkz.signHeader(t, header, valset, first, last),
	}
}

// GenSignedHeaderLastBlockID calls genHeader and signHeader and combines them into a SignedHeader.
func (pkz privKeys) GenSignedHeaderLastBlockID(t testing.TB, chainID string, height int64, bTime time.Time, txs types.Txs,
	valset, nextValset *types.ValidatorSet, appHash, consHash, resHash []byte, first, last int,
	lastBlockID types.BlockID) *types.SignedHeader {

	t.Helper()

	header := genHeader(chainID, height, bTime, txs, valset, nextValset, appHash, consHash, resHash)
	header.LastBlockID = lastBlockID
	return &types.SignedHeader{
		Header: header,
		Commit: pkz.signHeader(t, header, valset, first, last),
	}
}

// genLightBlocksWithValidatorsRotatingEveryBlock generates the header and validator set to create
// blocks to height. BlockIntervals are in per minute.
// NOTE: Expected to have a large validator set size ~ 100 validators.
func genLightBlocksWithValidatorsRotatingEveryBlock(
	t testing.TB,
	chainID string,
	numBlocks int64,
	valSize int,
	bTime time.Time) (
	map[int64]*types.SignedHeader,
	map[int64]*types.ValidatorSet,
	[]types.PrivValidator) {
	t.Helper()

	var (
		headers = make(map[int64]*types.SignedHeader, numBlocks)
		valset  = make(map[int64]*types.ValidatorSet, numBlocks+1)
	)

	vals, privVals := types.RandValidatorSet(valSize)
	keys := exposeMockPVKeys(privVals, vals.QuorumHash)

	newVals, newPrivVals := types.GenerateValidatorSet(
		types.NewValSetParam(vals.GetProTxHashes()),
		types.WithUpdatePrivValAt(privVals, 1),
	)
	newKeys := exposeMockPVKeys(newPrivVals, newVals.QuorumHash)

	// genesis header and vals
	lastHeader := keys.GenSignedHeader(t, chainID, 1, bTime.Add(1*time.Minute), nil,
		vals, newVals, hash("app_hash"), hash("cons_hash"),
		hash("results_hash"), 0, len(keys))
	currentHeader := lastHeader
	headers[1] = currentHeader
	valset[1] = vals
	keys = newKeys

	for height := int64(2); height <= numBlocks; height++ {
		newVals, newPrivVals := types.GenerateValidatorSet(
			types.NewValSetParam(vals.GetProTxHashes()),
			types.WithUpdatePrivValAt(privVals, height),
		)
		newKeys = exposeMockPVKeys(newPrivVals, newVals.QuorumHash)

		currentHeader = keys.GenSignedHeaderLastBlockID(t, chainID, height, bTime.Add(time.Duration(height)*time.Minute),
			nil,
			vals, newVals, hash("app_hash"), hash("cons_hash"),
			hash("results_hash"), 0, len(keys), types.BlockID{Hash: lastHeader.Hash()})
		headers[height] = currentHeader
		valset[height] = vals
		lastHeader = currentHeader
		keys = newKeys
	}

	return headers, valset, newPrivVals
}

func mockNodeFromHeadersAndVals(headers map[int64]*types.SignedHeader,
	vals map[int64]*types.ValidatorSet) *provider_mocks.Provider {
	mockNode := &provider_mocks.Provider{}
	for i, header := range headers {
		lb := &types.LightBlock{SignedHeader: header, ValidatorSet: vals[i]}
		mockNode.On("LightBlock", mock.Anything, i).Return(lb, nil)
	}
	return mockNode
}

func hash(s string) []byte {
	return crypto.Checksum([]byte(s))
}
