package lite_test

import (
	"fmt"
	"time"

	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/ed25519"
	"github.com/tendermint/tendermint/crypto/tmhash"

	tmproto "github.com/tendermint/tendermint/proto/types"
	"github.com/tendermint/tendermint/types"
	tmtime "github.com/tendermint/tendermint/types/time"
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

// // Change replaces the key at index i.
// func (pkz privKeys) Change(i int) privKeys {
// 	res := make(privKeys, len(pkz))
// 	copy(res, pkz)
// 	res[i] = ed25519.GenPrivKey()
// 	return res
// }

// Extend adds n more keys (to remove, just take a slice).
func (pkz privKeys) Extend(n int) privKeys {
	extra := genPrivKeys(n)
	return append(pkz, extra...)
}

// // GenSecpPrivKeys produces an array of secp256k1 private keys to generate commits.
// func GenSecpPrivKeys(n int) privKeys {
// 	res := make(privKeys, n)
// 	for i := range res {
// 		res[i] = secp256k1.GenPrivKey()
// 	}
// 	return res
// }

// // ExtendSecp adds n more secp256k1 keys (to remove, just take a slice).
// func (pkz privKeys) ExtendSecp(n int) privKeys {
// 	extra := GenSecpPrivKeys(n)
// 	return append(pkz, extra...)
// }

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
func (pkz privKeys) signHeader(header *types.Header, first, last int) *types.Commit {
	commitSigs := make([]types.CommitSig, len(pkz))
	for i := 0; i < len(pkz); i++ {
		commitSigs[i] = types.NewCommitSigAbsent()
	}

	// We need this list to keep the ordering.
	vset := pkz.ToValidators(1, 1)

	blockID := types.BlockID{
		Hash:        header.Hash(),
		PartsHeader: types.PartSetHeader{Total: 1, Hash: crypto.CRandBytes(32)},
	}

	// Fill in the votes we want.
	for i := first; i < last && i < len(pkz); i++ {
		vote, _ := makeVote(header, vset, pkz[i], blockID) //TODO: error here

		commitSigs[vote.ValidatorIndex] = vote.CommitSig()
	}

	return types.NewCommit(header.Height, 1, blockID, commitSigs)
}

func makeVote(header *types.Header, valset *types.ValidatorSet,
	key crypto.PrivKey, blockID types.BlockID) (*types.Vote, error) {

	addr := key.PubKey().Address()
	idx, _, ok := valset.GetByAddress(addr)
	if !ok {
		return nil, fmt.Errorf("addressL %v, is not associated with a validator", addr)
	}
	vote := &types.Vote{
		ValidatorAddress: addr,
		ValidatorIndex:   idx,
		Height:           header.Height,
		Round:            1,
		Timestamp:        tmtime.Now(),
		Type:             tmproto.PrecommitType,
		BlockID:          blockID,
	}
	// Sign it
	signBytes := vote.SignBytes(header.ChainID)
	sig, err := key.Sign(signBytes)
	if err != nil {
		return nil, fmt.Errorf("could not sign bytes: %w", err)
	}
	vote.Signature = sig

	return vote, nil
}

func genHeader(chainID string, height int64, bTime time.Time, txs types.Txs,
	valset, nextValset *types.ValidatorSet, appHash, consHash, resHash []byte) *types.Header {

	return &types.Header{
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
func (pkz privKeys) GenSignedHeader(chainID string, height int64, bTime time.Time, txs types.Txs,
	valset, nextValset *types.ValidatorSet, appHash, consHash, resHash []byte, first, last int) *types.SignedHeader {

	header := genHeader(chainID, height, bTime, txs, valset, nextValset, appHash, consHash, resHash)
	return &types.SignedHeader{
		Header: header,
		Commit: pkz.signHeader(header, first, last),
	}
}

// GenSignedHeaderLastBlockID calls genHeader and signHeader and combines them into a SignedHeader.
func (pkz privKeys) GenSignedHeaderLastBlockID(chainID string, height int64, bTime time.Time, txs types.Txs,
	valset, nextValset *types.ValidatorSet, appHash, consHash, resHash []byte, first, last int,
	lastBlockID types.BlockID) *types.SignedHeader {

	header := genHeader(chainID, height, bTime, txs, valset, nextValset, appHash, consHash, resHash)
	header.LastBlockID = lastBlockID
	return &types.SignedHeader{
		Header: header,
		Commit: pkz.signHeader(header, first, last),
	}
}

func (pkz privKeys) ChangeKeys(delta int) privKeys {
	newKeys := pkz[delta:]
	return newKeys.Extend(delta)
}

// Generates the header and validator set to create a full entire mock node with blocks to height (
// blockSize) and with variation in validator sets. BlockIntervals are in per minute.
// NOTE: Expected to have a large validator set size ~ 100 validators.
func GenMockNode(
	chainID string,
	blockSize int64,
	valSize int,
	valVariation float32,
	bTime time.Time) (
	string,
	map[int64]*types.SignedHeader,
	map[int64]*types.ValidatorSet) {

	var (
		headers         = make(map[int64]*types.SignedHeader, blockSize)
		valset          = make(map[int64]*types.ValidatorSet, blockSize)
		keys            = genPrivKeys(valSize)
		totalVariation  = valVariation
		valVariationInt int
		newKeys         privKeys
	)

	valVariationInt = int(totalVariation)
	totalVariation = -float32(valVariationInt)
	newKeys = keys.ChangeKeys(valVariationInt)

	// genesis header and vals
	lastHeader := keys.GenSignedHeader(chainID, 1, bTime.Add(1*time.Minute), nil,
		keys.ToValidators(2, 2), newKeys.ToValidators(2, 2), hash("app_hash"), hash("cons_hash"),
		hash("results_hash"), 0, len(keys))
	currentHeader := lastHeader
	headers[1] = currentHeader
	valset[1] = keys.ToValidators(2, 2)
	keys = newKeys

	for height := int64(2); height <= blockSize; height++ {
		totalVariation += valVariation
		valVariationInt = int(totalVariation)
		totalVariation = -float32(valVariationInt)
		newKeys = keys.ChangeKeys(valVariationInt)
		currentHeader = keys.GenSignedHeaderLastBlockID(chainID, height, bTime.Add(time.Duration(height)*time.Minute),
			nil,
			keys.ToValidators(2, 2), newKeys.ToValidators(2, 2), hash("app_hash"), hash("cons_hash"),
			hash("results_hash"), 0, len(keys), types.BlockID{Hash: lastHeader.Hash()})
		headers[height] = currentHeader
		valset[height] = keys.ToValidators(2, 2)
		lastHeader = currentHeader
		keys = newKeys
	}

	return chainID, headers, valset
}

func hash(s string) []byte {
	return tmhash.Sum([]byte(s))
}
