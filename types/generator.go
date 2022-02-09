package types

import (
	"context"
	"sort"

	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/bls12381"
)

// RandValidatorSet returns a randomized validator set (size: +numValidators+),
// where each validator has the same default voting power.
func RandValidatorSet(n int) (*ValidatorSet, []PrivValidator) {
	return GenerateValidatorSet(NewValSetParam(crypto.RandProTxHashes(n)))
}

// ValSetParam is a structure of parameters to make a validator set
type ValSetParam struct {
	ProTxHash   ProTxHash
	VotingPower int64
}

// NewValSetParam returns a list of validator set parameters with for every proTxHashes
// with default voting power
func NewValSetParam(proTxHashes []crypto.ProTxHash) []ValSetParam {
	opts := make([]ValSetParam, len(proTxHashes))
	for i, proTxHash := range proTxHashes {
		opts[i] = ValSetParam{
			ProTxHash:   proTxHash,
			VotingPower: DefaultDashVotingPower,
		}
	}
	return opts
}

// ValSetOptions is the options for generation of validator set and private validators
type ValSetOptions struct {
	PrivValsMap      map[string]PrivValidator
	PrivValsAtHeight int64
}

// WithUpdatePrivValAt sets a list of the private validators to update validator set at passed the height
func WithUpdatePrivValAt(privVals []PrivValidator, height int64) func(opt *ValSetOptions) {
	return func(opt *ValSetOptions) {
		for _, pv := range privVals {
			proTxhash, _ := pv.GetProTxHash(context.Background())
			opt.PrivValsMap[proTxhash.String()] = pv
		}
		opt.PrivValsAtHeight = height
	}
}

// ValSetOptionFunc is a type of validator-set function to manage options of generator
type ValSetOptionFunc func(opt *ValSetOptions)

// GenerateValidatorSet generates a validator set and a list of private validators
func GenerateValidatorSet(valParams []ValSetParam, opts ...ValSetOptionFunc) (*ValidatorSet, []PrivValidator) {
	var (
		n              = len(valParams)
		proTxHashes    = make([]crypto.ProTxHash, n)
		valz           = make([]*Validator, n)
		privValidators = make([]PrivValidator, n)
		valzOptsMap    = make(map[string]ValSetParam)
	)
	for i, opt := range valParams {
		proTxHashes[i] = opt.ProTxHash
		valzOptsMap[opt.ProTxHash.String()] = opt
	}
	valSetOpts := ValSetOptions{
		PrivValsMap: make(map[string]PrivValidator),
	}
	for _, fn := range opts {
		fn(&valSetOpts)
	}
	orderedProTxHashes, privateKeys, thresholdPublicKey :=
		bls12381.CreatePrivLLMQDataOnProTxHashesDefaultThreshold(proTxHashes)
	quorumHash := crypto.RandQuorumHash()
	mockPVFunc := newMockPVFunc(valSetOpts, quorumHash, thresholdPublicKey)
	for i := 0; i < n; i++ {
		privValidators[i] = mockPVFunc(orderedProTxHashes[i], privateKeys[i])
		opt := valzOptsMap[orderedProTxHashes[i].String()]
		valz[i] = NewValidator(privateKeys[i].PubKey(), opt.VotingPower, orderedProTxHashes[i])
	}
	sort.Sort(PrivValidatorsByProTxHash(privValidators))
	return NewValidatorSet(valz, thresholdPublicKey, crypto.SmallQuorumType(), quorumHash, true), privValidators
}

// MakeGenesisValsFromValidatorSet converts ValidatorSet data into a list of GenesisValidator
func MakeGenesisValsFromValidatorSet(valz *ValidatorSet) []GenesisValidator {
	genVals := make([]GenesisValidator, len(valz.Validators))
	for i, val := range valz.Validators {
		genVals[i] = GenesisValidator{
			PubKey:    val.PubKey,
			Power:     DefaultDashVotingPower,
			ProTxHash: val.ProTxHash,
		}
	}
	return genVals
}

func newMockPVFunc(
	opts ValSetOptions,
	quorumHash crypto.QuorumHash,
	thresholdPubKey crypto.PubKey,
) func(crypto.ProTxHash, crypto.PrivKey) PrivValidator {
	return func(
		proTxHash crypto.ProTxHash,
		privKey crypto.PrivKey,
	) PrivValidator {
		privVal, ok := opts.PrivValsMap[proTxHash.String()]
		if ok && opts.PrivValsAtHeight > 0 {
			privVal.UpdatePrivateKey(context.Background(), privKey, quorumHash, thresholdPubKey, opts.PrivValsAtHeight)
			return privVal
		}
		return NewMockPVWithParams(
			privKey,
			proTxHash,
			quorumHash,
			thresholdPubKey,
			false,
			false,
		)
	}
}
