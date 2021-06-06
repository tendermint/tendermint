package light_test

import (
	"github.com/dashevo/dashd-go/btcjson"
	"time"

	"github.com/tendermint/tendermint/crypto/bls12381"
	"github.com/tendermint/tendermint/crypto/ed25519"

	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/tmhash"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	tmversion "github.com/tendermint/tendermint/proto/tendermint/version"
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
func genPrivKeys(n int, keyType crypto.KeyType) privKeys {
	res := make(privKeys, n)
	for i := range res {
		switch keyType {
		case crypto.BLS12381:
			res[i] = bls12381.GenPrivKey()
		case crypto.Ed25519:
			res[i] = ed25519.GenPrivKey()
		default:
			panic("genPrivKeys: unsupported keyType received")
		}
	}
	return res
}

func exposeMockPVKeys(pvs []*types.MockPV) privKeys {
	res := make(privKeys, len(pvs))
	for i, pval := range pvs {
		res[i] = pval.PrivKey
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
	extra := genPrivKeys(n, crypto.BLS12381)
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
func (pkz privKeys) ToValidators(thresholdPublicKey crypto.PubKey) *types.ValidatorSet {
	res := make([]*types.Validator, len(pkz))
	for i, k := range pkz {
		res[i] = types.NewValidatorDefaultVotingPower(k.PubKey(), crypto.Sha256(k.PubKey().Address()))
	}
	// Quorum hash is pseudorandom
	return types.NewValidatorSet(res, thresholdPublicKey, btcjson.LLMQType_5_60, crypto.Sha256(thresholdPublicKey.Bytes()), true)
}

// signHeader properly signs the header with all keys from first to last exclusive.
func (pkz privKeys) signHeader(header *types.Header, valSet *types.ValidatorSet, first, last int) *types.Commit {
	commitSigs := make([]types.CommitSig, len(pkz))
	var blockSigs [][]byte
	var stateSigs [][]byte
	var blsIDs [][]byte
	for i := 0; i < len(pkz); i++ {
		commitSigs[i] = types.NewCommitSigAbsent()
	}

	blockID := types.BlockID{
		Hash:          header.Hash(),
		PartSetHeader: types.PartSetHeader{Total: 1, Hash: crypto.CRandBytes(32)},
	}

	stateID := types.StateID{
		LastAppHash: header.AppHash,
	}

	// Fill in the votes we want.
	for i := first; i < last && i < len(pkz); i++ {
		// Verify that the private key matches the validator proTxHash
		privateKey := pkz[i]
		proTxHash, val := valSet.GetByIndex(int32(i))
		if !privateKey.PubKey().Equals(val.PubKey) {
			panic("light client keys do not match")
		}
		vote := makeVote(header, valSet, proTxHash, pkz[i], blockID, stateID)
		commitSigs[vote.ValidatorIndex] = vote.CommitSig()
		blockSigs = append(blockSigs, vote.BlockSignature)
		stateSigs = append(stateSigs, vote.StateSignature)
		blsIDs = append(blsIDs, vote.ValidatorProTxHash)
	}

	thresholdBlockSig, _ := bls12381.RecoverThresholdSignatureFromShares(blockSigs, blsIDs)
	thresholdStateSig, _ := bls12381.RecoverThresholdSignatureFromShares(stateSigs, blsIDs)

	return types.NewCommit(header.Height, 1, blockID, stateID, commitSigs, valSet.QuorumHash, thresholdBlockSig, thresholdStateSig)
}

func makeVote(header *types.Header, valset *types.ValidatorSet, proTxHash crypto.ProTxHash,
	key crypto.PrivKey, blockID types.BlockID, stateID types.StateID) *types.Vote {

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
		StateID:            stateID,
	}

	v := vote.ToProto()
	// SignDigest it
	signId:= types.VoteBlockSignId(header.ChainID, v, valset.QuorumType, valset.QuorumHash)
	sig, err := key.SignDigest(signId)
	if err != nil {
		panic(err)
	}

	// SignDigest it
	stateSignId := types.VoteStateSignId(header.ChainID, v, valset.QuorumType, valset.QuorumHash)
	sigState, err := key.SignDigest(stateSignId)
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
		Version: tmversion.Consensus{Block: version.BlockProtocol, App: 0},
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
func (pkz privKeys) GenSignedHeader(chainID string, height int64, bTime time.Time, txs types.Txs,
	valset, nextValset *types.ValidatorSet, appHash, consHash, resHash []byte, first, last int) *types.SignedHeader {

	header := genHeader(chainID, height, bTime, txs, valset, nextValset, appHash, consHash, resHash)
	return &types.SignedHeader{
		Header: header,
		Commit: pkz.signHeader(header, valset, first, last),
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
		Commit: pkz.signHeader(header, valset, first, last),
	}
}

func (pkz privKeys) ChangeKeys(delta int) privKeys {
	newKeys := pkz[delta:]
	return newKeys.Extend(delta)
}

// Generates the header and validator set to create a full entire mock node with blocks to height (
// blockSize) and with variation in validator sets. BlockIntervals are in per minute.
// NOTE: Expected to have a large validator set size ~ 100 validators.
func genMockNodeWithKeys(
	chainID string,
	blockSize int64,
	valSize int,
	bTime time.Time) (
	map[int64]*types.SignedHeader,
	map[int64]*types.ValidatorSet,
	map[int64]privKeys) {

	var (
		headers            = make(map[int64]*types.SignedHeader, blockSize)
		valsets            = make(map[int64]*types.ValidatorSet, blockSize+1)
		keymap             = make(map[int64]privKeys, blockSize+1)
		valset0, privVals0 = types.GenerateMockValidatorSet(valSize)
		keys               = exposeMockPVKeys(privVals0)
		newKeys            privKeys
	)

	nextValSet, nextPrivVals := types.GenerateMockValidatorSetUsingProTxHashes(valset0.GetProTxHashes())
	newKeys = exposeMockPVKeys(nextPrivVals)
	keymap[1] = keys
	keymap[2] = newKeys

	// genesis header and vals
	lastHeader := keys.GenSignedHeader(chainID, 1, bTime.Add(1*time.Minute), nil,
		valset0, nextValSet, hash("app_hash"), hash("cons_hash"),
		hash("results_hash"), 0, len(keys))
	currentHeader := lastHeader
	headers[1] = currentHeader
	valsets[1] = valset0
	keys = newKeys
	currentValset := nextValSet

	for height := int64(2); height <= blockSize; height++ {
		nextValSet, nextPrivVals := types.GenerateMockValidatorSetUsingProTxHashes(valset0.GetProTxHashes())
		newKeys = exposeMockPVKeys(nextPrivVals)
		currentHeader = keys.GenSignedHeaderLastBlockID(chainID, height, bTime.Add(time.Duration(height)*time.Minute),
			nil,
			currentValset, nextValSet, hash("app_hash"), hash("cons_hash"),
			hash("results_hash"), 0, len(keys), types.BlockID{Hash: lastHeader.Hash()})
		headers[height] = currentHeader
		valsets[height] = currentValset
		lastHeader = currentHeader
		keys = newKeys
		currentValset = nextValSet
		keymap[height+1] = keys
	}

	return headers, valsets, keymap
}

func genMockNode(
	chainID string,
	blockSize int64,
	valSize int,
	bTime time.Time) (
	string,
	map[int64]*types.SignedHeader,
	map[int64]*types.ValidatorSet) {
	headers, valset, _ := genMockNodeWithKeys(chainID, blockSize, valSize, bTime)
	return chainID, headers, valset
}

func hash(s string) []byte {
	return tmhash.Sum([]byte(s))
}
