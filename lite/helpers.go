package lite

import (
	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/ed25519"
	"github.com/tendermint/tendermint/crypto/secp256k1"

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

// Change replaces the key at index i.
func (pkz privKeys) Change(i int) privKeys {
	res := make(privKeys, len(pkz))
	copy(res, pkz)
	res[i] = ed25519.GenPrivKey()
	return res
}

// Extend adds n more keys (to remove, just take a slice).
func (pkz privKeys) Extend(n int) privKeys {
	extra := genPrivKeys(n)
	return append(pkz, extra...)
}

// GenSecpPrivKeys produces an array of secp256k1 private keys to generate commits.
func GenSecpPrivKeys(n int) privKeys {
	res := make(privKeys, n)
	for i := range res {
		res[i] = secp256k1.GenPrivKey()
	}
	return res
}

// ExtendSecp adds n more secp256k1 keys (to remove, just take a slice).
func (pkz privKeys) ExtendSecp(n int) privKeys {
	extra := GenSecpPrivKeys(n)
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
func (pkz privKeys) signHeader(header *types.Header, first, last int) *types.Commit {
	commitSigs := make([]*types.CommitSig, len(pkz))

	// We need this list to keep the ordering.
	vset := pkz.ToValidators(1, 0)

	// Fill in the votes we want.
	for i := first; i < last && i < len(pkz); i++ {
		vote := makeVote(header, vset, pkz[i])
		commitSigs[vote.ValidatorIndex] = vote.CommitSig()
	}
	blockID := types.BlockID{Hash: header.Hash()}
	return types.NewCommit(blockID, commitSigs)
}

func makeVote(header *types.Header, valset *types.ValidatorSet, key crypto.PrivKey) *types.Vote {
	addr := key.PubKey().Address()
	idx, _ := valset.GetByAddress(addr)
	vote := &types.Vote{
		ValidatorAddress: addr,
		ValidatorIndex:   idx,
		Height:           header.Height,
		Round:            1,
		Timestamp:        tmtime.Now(),
		Type:             types.PrecommitType,
		BlockID:          types.BlockID{Hash: header.Hash()},
	}
	// Sign it
	signBytes := vote.SignBytes(header.ChainID)
	// TODO Consider reworking makeVote API to return an error
	sig, err := key.Sign(signBytes)
	if err != nil {
		panic(err)
	}
	vote.Signature = sig

	return vote
}

func genHeader(chainID string, height int64, txs types.Txs,
	valset, nextValset *types.ValidatorSet, appHash, consHash, resHash []byte) *types.Header {

	return &types.Header{
		ChainID:  chainID,
		Height:   height,
		Time:     tmtime.Now(),
		NumTxs:   int64(len(txs)),
		TotalTxs: int64(len(txs)),
		// LastBlockID
		// LastCommitHash
		ValidatorsHash:     valset.Hash(),
		NextValidatorsHash: nextValset.Hash(),
		DataHash:           txs.Hash(),
		AppHash:            appHash,
		ConsensusHash:      consHash,
		LastResultsHash:    resHash,
	}
}

// GenSignedHeader calls genHeader and signHeader and combines them into a SignedHeader.
func (pkz privKeys) GenSignedHeader(chainID string, height int64, txs types.Txs,
	valset, nextValset *types.ValidatorSet, appHash, consHash, resHash []byte, first, last int) types.SignedHeader {

	header := genHeader(chainID, height, txs, valset, nextValset, appHash, consHash, resHash)
	check := types.SignedHeader{
		Header: header,
		Commit: pkz.signHeader(header, first, last),
	}
	return check
}

// GenFullCommit calls genHeader and signHeader and combines them into a FullCommit.
func (pkz privKeys) GenFullCommit(chainID string, height int64, txs types.Txs,
	valset, nextValset *types.ValidatorSet, appHash, consHash, resHash []byte, first, last int) FullCommit {

	header := genHeader(chainID, height, txs, valset, nextValset, appHash, consHash, resHash)
	commit := types.SignedHeader{
		Header: header,
		Commit: pkz.signHeader(header, first, last),
	}
	return NewFullCommit(commit, valset, nextValset)
}
