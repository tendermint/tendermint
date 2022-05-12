package light_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/ed25519"
	tmtime "github.com/tendermint/tendermint/libs/time"
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

// genPrivKeys produces an array of private keys to generate commits.
func genPrivKeys(n int) privKeys {
	res := make(privKeys, n)
	for i := range res {
		res[i] = ed25519.GenPrivKey()
	}
	return res
}

// Extend adds n more keys (to remove, just take a slice).
func (pkz privKeys) Extend(n int) privKeys {
	extra := genPrivKeys(n)
	return append(pkz, extra...)
}

// ToValidators produces a valset from the set of keys.
// The first key has weight `init` and it increases by `inc` every step
// so we can have all the same weight, or a simple linear distribution
// (should be enough for testing).
func (pkz privKeys) ToValidators(init, inc int64) *types.ValidatorSet {
	res := make([]*types.Validator, len(pkz))
	for i, k := range pkz {
		res[i] = types.NewValidator(k.PubKey(), init+int64(i)*inc)
	}
	return types.NewValidatorSet(res)
}

// signHeader properly signs the header with all keys from first to last exclusive.
func (pkz privKeys) signHeader(t testing.TB, header *types.Header, valSet *types.ValidatorSet, first, last int) *types.Commit {
	t.Helper()

	commitSigs := make([]types.CommitSig, len(pkz))
	for i := 0; i < len(pkz); i++ {
		commitSigs[i] = types.NewCommitSigAbsent()
	}

	blockID := types.BlockID{
		Hash:          header.Hash(),
		PartSetHeader: types.PartSetHeader{Total: 1, Hash: crypto.CRandBytes(32)},
	}

	// Fill in the votes we want.
	for i := first; i < last && i < len(pkz); i++ {
		vote := makeVote(t, header, valSet, pkz[i], blockID)
		commitSigs[vote.ValidatorIndex] = vote.CommitSig()
	}

	return &types.Commit{
		Height:     header.Height,
		Round:      1,
		BlockID:    blockID,
		Signatures: commitSigs,
	}
}

func makeVote(t testing.TB, header *types.Header, valset *types.ValidatorSet, key crypto.PrivKey, blockID types.BlockID) *types.Vote {
	t.Helper()

	addr := key.PubKey().Address()
	idx, _ := valset.GetByAddress(addr)
	vote := &types.Vote{
		ValidatorAddress: addr,
		ValidatorIndex:   idx,
		Height:           header.Height,
		Round:            1,
		Timestamp:        tmtime.Now(),
		Type:             tmproto.PrecommitType,
		BlockID:          blockID,
	}

	v := vote.ToProto()
	// Sign it
	signBytes := types.VoteSignBytes(header.ChainID, v)
	sig, err := key.Sign(signBytes)
	require.NoError(t, err)

	vote.Signature = sig

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
		ProposerAddress:    valset.Validators[0].Address,
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

func (pkz privKeys) ChangeKeys(delta int) privKeys {
	newKeys := pkz[delta:]
	return newKeys.Extend(delta)
}

// genLightBlocksWithKeys generates the header and validator set to create
// blocks to height. BlockIntervals are in per minute.
// NOTE: Expected to have a large validator set size ~ 100 validators.
func genLightBlocksWithKeys(
	t testing.TB,
	numBlocks int64,
	valSize int,
	valVariation float32,
	bTime time.Time,
) (map[int64]*types.SignedHeader, map[int64]*types.ValidatorSet, map[int64]privKeys) {
	t.Helper()

	var (
		headers         = make(map[int64]*types.SignedHeader, numBlocks)
		valset          = make(map[int64]*types.ValidatorSet, numBlocks+1)
		keymap          = make(map[int64]privKeys, numBlocks+1)
		keys            = genPrivKeys(valSize)
		totalVariation  = valVariation
		valVariationInt int
		newKeys         privKeys
	)

	valVariationInt = int(totalVariation)
	totalVariation = -float32(valVariationInt)
	newKeys = keys.ChangeKeys(valVariationInt)
	keymap[1] = keys
	keymap[2] = newKeys

	// genesis header and vals
	lastHeader := keys.GenSignedHeader(t, chainID, 1, bTime.Add(1*time.Minute), nil,
		keys.ToValidators(2, 0), newKeys.ToValidators(2, 0), hash("app_hash"), hash("cons_hash"),
		hash("results_hash"), 0, len(keys))
	currentHeader := lastHeader
	headers[1] = currentHeader
	valset[1] = keys.ToValidators(2, 0)
	keys = newKeys

	for height := int64(2); height <= numBlocks; height++ {
		totalVariation += valVariation
		valVariationInt = int(totalVariation)
		totalVariation = -float32(valVariationInt)
		newKeys = keys.ChangeKeys(valVariationInt)
		currentHeader = keys.GenSignedHeaderLastBlockID(t, chainID, height, bTime.Add(time.Duration(height)*time.Minute),
			nil,
			keys.ToValidators(2, 0), newKeys.ToValidators(2, 0), hash("app_hash"), hash("cons_hash"),
			hash("results_hash"), 0, len(keys), types.BlockID{Hash: lastHeader.Hash()})
		headers[height] = currentHeader
		valset[height] = keys.ToValidators(2, 0)
		lastHeader = currentHeader
		keys = newKeys
		keymap[height+1] = keys
	}

	return headers, valset, keymap
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
