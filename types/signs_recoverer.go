package types

import (
	"fmt"

	"github.com/tendermint/tendermint/crypto/bls12381"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
)

// SignsRecoverer is used to recover threshold block, state, and vote-extension signatures
// it's possible to avoid recovering state and vote-extension for specific case
type SignsRecoverer struct {
	blockSigs            [][]byte
	stateSigs            [][]byte
	validatorProTxHashes [][]byte
	voteExts             [][]byte
	voteExtSigs          [][][]byte

	quorumReached bool
}

// WithQuorumReached sets a flag at SignsRecoverer to recovers threshold signatures for stateID and vote-extensions
func WithQuorumReached(quorumReached bool) func(*SignsRecoverer) {
	return func(r *SignsRecoverer) {
		r.quorumReached = quorumReached
	}
}

// NewSignsRecoverer creates and returns a new instance of SignsRecoverer
// the state fills with signatures from the votes
func NewSignsRecoverer(votes []*Vote, opts ...func(*SignsRecoverer)) *SignsRecoverer {
	sigs := SignsRecoverer{
		quorumReached: true,
	}
	for _, opt := range opts {
		opt(&sigs)
	}
	sigs.init(votes)
	return &sigs
}

// Recover recovers threshold signatures for block, state and vote-extensions
func (v *SignsRecoverer) Recover() (*QuorumSigns, error) {
	thresholdSigns := &QuorumSigns{}
	recoverFuncs := []func(signs *QuorumSigns) error{
		v.recoverBlockSig,
		v.recoverStateSig,
		v.recoverVoteExtensionSigs,
	}
	for _, fn := range recoverFuncs {
		err := fn(thresholdSigns)
		if err != nil {
			return nil, err
		}
	}
	return thresholdSigns, nil
}

func (v *SignsRecoverer) init(votes []*Vote) {
	v.blockSigs = nil
	v.stateSigs = nil
	v.validatorProTxHashes = nil
	v.voteExtSigs = nil
	for _, vote := range votes {
		v.addVoteSigs(vote)
	}
}

func (v *SignsRecoverer) addVoteSigs(vote *Vote) {
	if vote == nil {
		return
	}
	v.blockSigs = append(v.blockSigs, vote.BlockSignature)
	v.stateSigs = append(v.stateSigs, vote.StateSignature)
	v.validatorProTxHashes = append(v.validatorProTxHashes, vote.ValidatorProTxHash)
	v.addVoteExtensions(vote.VoteExtensions)
}

func (v *SignsRecoverer) addVoteExtensions(voteExtensions VoteExtensions) {
	extensions := voteExtensions[tmproto.VoteExtensionType_THRESHOLD_RECOVER]
	for i, ext := range extensions {
		if len(extensions) > len(v.voteExtSigs) {
			v.voteExts = append(v.voteExts, ext.Extension)
			v.voteExtSigs = append(v.voteExtSigs, nil)
		}
		v.voteExtSigs[i] = append(v.voteExtSigs[i], ext.Signature)
	}
}

func (v *SignsRecoverer) recoverStateSig(thresholdSigns *QuorumSigns) error {
	if !v.quorumReached {
		return nil
	}
	var err error
	thresholdSigns.StateSign, err = bls12381.RecoverThresholdSignatureFromShares(v.stateSigs, v.validatorProTxHashes)
	if err != nil {
		return fmt.Errorf("error recovering threshold state sig: %w", err)
	}
	return nil
}

func (v *SignsRecoverer) recoverBlockSig(thresholdSigns *QuorumSigns) error {
	var err error
	thresholdSigns.BlockSign, err = bls12381.RecoverThresholdSignatureFromShares(v.blockSigs, v.validatorProTxHashes)
	if err != nil {
		return fmt.Errorf("error recovering threshold block sig: %w", err)
	}
	return nil
}

func (v *SignsRecoverer) recoverVoteExtensionSigs(thresholdSigns *QuorumSigns) error {
	if !v.quorumReached {
		return nil
	}
	var err error
	thresholdSigns.ExtensionSigns = make([]ThresholdExtensionSign, len(v.voteExtSigs))
	for i, sigs := range v.voteExtSigs {
		if len(sigs) > 0 {
			thresholdSigns.ExtensionSigns[i].Extension = v.voteExts[i]
			thresholdSigns.ExtensionSigns[i].ThresholdSignature, err = bls12381.RecoverThresholdSignatureFromShares(sigs, v.validatorProTxHashes)
			if err != nil {
				return fmt.Errorf("error recovering threshold vote-extension sig: %w", err)
			}
		}
	}
	return nil
}
