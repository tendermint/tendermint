package types

import (
	"fmt"

	"github.com/tendermint/tendermint/crypto/bls12381"
)

// SigsRecoverer is used to recover threshold block, state, and vote-extension signatures
// it's possible to avoid recovering state and vote-extension for specific case
type SigsRecoverer struct {
	blockSigs   [][]byte
	stateSigs   [][]byte
	blsIDs      [][]byte
	voteExtSigs [][][]byte

	shouldRecoveryStateSig   bool
	shouldRecoverVoteExtSigs bool
}

// NewSignsRecoverer creates and returns a new instance of SigsRecoverer
// the state fills with signatures from the votes
func NewSignsRecoverer(votes []*Vote) *SigsRecoverer {
	sigs := SigsRecoverer{
		shouldRecoveryStateSig:   true,
		shouldRecoverVoteExtSigs: true,
	}
	sigs.Init(votes)
	return &sigs
}

// Init initializes a state with a list of votes
func (v *SigsRecoverer) Init(votes []*Vote) {
	v.blockSigs = nil
	v.stateSigs = nil
	v.blsIDs = nil
	v.voteExtSigs = nil
	for _, vote := range votes {
		v.addVoteSigs(vote)
	}
}

// Recover recovers threshold signatures for block, state and vote-extensions
func (v *SigsRecoverer) Recover() (*ThresholdVoteSigns, error) {
	thresholdSigns := &ThresholdVoteSigns{}
	recoverFuncs := []func(*ThresholdVoteSigns) error{
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

func (v *SigsRecoverer) addVoteSigs(vote *Vote) {
	if vote == nil {
		return
	}
	v.blockSigs = append(v.blockSigs, vote.BlockSignature)
	v.stateSigs = append(v.stateSigs, vote.StateSignature)
	v.blsIDs = append(v.blsIDs, vote.ValidatorProTxHash)
	v.addVoteExtensions(vote.VoteExtensions)
}

func (v *SigsRecoverer) addVoteExtensions(voteExtensions []VoteExtension) {
	m := 0
	for j, ext := range voteExtensions {
		if !ext.IsRecoverable() {
			m++
			continue
		}
		k := j - m
		if k >= len(v.voteExtSigs) {
			v.voteExtSigs = append(v.voteExtSigs, nil)
		}
		v.voteExtSigs[k] = append(v.voteExtSigs[k], ext.Signature)
	}
}

func (v *SigsRecoverer) recoveryOnlyBlockSig() {
	v.shouldRecoveryStateSig = false
	v.shouldRecoverVoteExtSigs = false
}

func (v *SigsRecoverer) recoverStateSig(thresholdSigns *ThresholdVoteSigns) error {
	//fmt.Printf("[debug] v.shouldRecoveryStateSig = %v\n", v.shouldRecoverVoteExtSigs)
	if !v.shouldRecoveryStateSig {
		return nil
	}
	var err error
	thresholdSigns.StateSign, err = bls12381.RecoverThresholdSignatureFromShares(v.stateSigs, v.blsIDs)
	if err != nil {
		return fmt.Errorf("error recovering threshold state sig: %w", err)
	}
	return nil
}

func (v *SigsRecoverer) recoverBlockSig(thresholdSigns *ThresholdVoteSigns) error {
	var err error
	thresholdSigns.BlockSign, err = bls12381.RecoverThresholdSignatureFromShares(v.blockSigs, v.blsIDs)
	if err != nil {
		return fmt.Errorf("error recovering threshold block sig: %w", err)
	}
	return nil
}

func (v *SigsRecoverer) recoverVoteExtensionSigs(thresholdSigns *ThresholdVoteSigns) error {
	if !v.shouldRecoverVoteExtSigs {
		return nil
	}
	var err error
	thresholdSigns.VoteExtSigns = make([][]byte, len(v.voteExtSigs))
	for i, sigs := range v.voteExtSigs {
		if len(sigs) > 0 {
			thresholdSigns.VoteExtSigns[i], err = bls12381.RecoverThresholdSignatureFromShares(sigs, v.blsIDs)
			if err != nil {
				return fmt.Errorf("error recovering threshold vote-extensin sig: %w", err)
			}
		}
	}
	return nil
}
