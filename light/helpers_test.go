package light_test

import (
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/ed25519"
	"github.com/tendermint/tendermint/crypto/tmhash"
	tmtime "github.com/tendermint/tendermint/libs/time"
	provider_mocks "github.com/tendermint/tendermint/light/provider/mocks"
	"github.com/tendermint/tendermint/pkg/consensus"
	"github.com/tendermint/tendermint/pkg/light"
	"github.com/tendermint/tendermint/pkg/mempool"
	"github.com/tendermint/tendermint/pkg/metadata"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
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
func (pkz privKeys) ToValidators(init, inc int64) *consensus.ValidatorSet {
	res := make([]*consensus.Validator, len(pkz))
	for i, k := range pkz {
		res[i] = consensus.NewValidator(k.PubKey(), init+int64(i)*inc)
	}
	return consensus.NewValidatorSet(res)
}

// signHeader properly signs the header with all keys from first to last exclusive.
func (pkz privKeys) signHeader(header *metadata.Header, valSet *consensus.ValidatorSet,
	first, last int) *metadata.Commit {
	commitSigs := make([]metadata.CommitSig, len(pkz))
	for i := 0; i < len(pkz); i++ {
		commitSigs[i] = metadata.NewCommitSigAbsent()
	}

	blockID := metadata.BlockID{
		Hash:          header.Hash(),
		PartSetHeader: metadata.PartSetHeader{Total: 1, Hash: crypto.CRandBytes(32)},
	}

	// Fill in the votes we want.
	for i := first; i < last && i < len(pkz); i++ {
		vote := makeVote(header, valSet, pkz[i], blockID)
		commitSigs[vote.ValidatorIndex] = vote.CommitSig()
	}

	return metadata.NewCommit(header.Height, 1, blockID, commitSigs)
}

func makeVote(header *metadata.Header, valset *consensus.ValidatorSet,
	key crypto.PrivKey, blockID metadata.BlockID) *consensus.Vote {

	addr := key.PubKey().Address()
	idx, _ := valset.GetByAddress(addr)
	vote := &consensus.Vote{
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
	signBytes := consensus.VoteSignBytes(header.ChainID, v)
	sig, err := key.Sign(signBytes)
	if err != nil {
		panic(err)
	}

	vote.Signature = sig

	return vote
}

func genHeader(chainID string, height int64, bTime time.Time, txs mempool.Txs,
	valset, nextValset *consensus.ValidatorSet, appHash, consHash, resHash []byte) *metadata.Header {

	return &metadata.Header{
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
func (pkz privKeys) GenSignedHeader(chainID string, height int64, bTime time.Time, txs mempool.Txs,
	valset, nextValset *consensus.ValidatorSet, appHash, consHash, resHash []byte, first, last int,
) *metadata.SignedHeader {

	header := genHeader(chainID, height, bTime, txs, valset, nextValset, appHash, consHash, resHash)
	return &metadata.SignedHeader{
		Header: header,
		Commit: pkz.signHeader(header, valset, first, last),
	}
}

// GenSignedHeaderLastBlockID calls genHeader and signHeader and combines them into a SignedHeader.
func (pkz privKeys) GenSignedHeaderLastBlockID(chainID string, height int64, bTime time.Time, txs mempool.Txs,
	valset, nextValset *consensus.ValidatorSet, appHash, consHash, resHash []byte, first, last int,
	lastBlockID metadata.BlockID) *metadata.SignedHeader {

	header := genHeader(chainID, height, bTime, txs, valset, nextValset, appHash, consHash, resHash)
	header.LastBlockID = lastBlockID
	return &metadata.SignedHeader{
		Header: header,
		Commit: pkz.signHeader(header, valset, first, last),
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
	chainID string,
	numBlocks int64,
	valSize int,
	valVariation float32,
	bTime time.Time) (
	map[int64]*metadata.SignedHeader,
	map[int64]*consensus.ValidatorSet,
	map[int64]privKeys) {

	var (
		headers         = make(map[int64]*metadata.SignedHeader, numBlocks)
		valset          = make(map[int64]*consensus.ValidatorSet, numBlocks+1)
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
	lastHeader := keys.GenSignedHeader(chainID, 1, bTime.Add(1*time.Minute), nil,
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
		currentHeader = keys.GenSignedHeaderLastBlockID(chainID, height, bTime.Add(time.Duration(height)*time.Minute),
			nil,
			keys.ToValidators(2, 0), newKeys.ToValidators(2, 0), hash("app_hash"), hash("cons_hash"),
			hash("results_hash"), 0, len(keys), metadata.BlockID{Hash: lastHeader.Hash()})
		headers[height] = currentHeader
		valset[height] = keys.ToValidators(2, 0)
		lastHeader = currentHeader
		keys = newKeys
		keymap[height+1] = keys
	}

	return headers, valset, keymap
}

func mockNodeFromHeadersAndVals(headers map[int64]*metadata.SignedHeader,
	vals map[int64]*consensus.ValidatorSet) *provider_mocks.Provider {
	mockNode := &provider_mocks.Provider{}
	for i, header := range headers {
		lb := &light.LightBlock{SignedHeader: header, ValidatorSet: vals[i]}
		mockNode.On("LightBlock", mock.Anything, i).Return(lb, nil)
	}
	return mockNode
}

func hash(s string) []byte {
	return tmhash.Sum([]byte(s))
}
