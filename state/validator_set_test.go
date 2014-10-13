package state

import (
	. "github.com/tendermint/tendermint/common"

	"bytes"
	"testing"
)

func randValidator() *Validator {
	return &Validator{
		Account: Account{
			Id:     RandUInt64(),
			PubKey: CRandBytes(32),
		},
		BondHeight:       RandUInt32(),
		UnbondHeight:     RandUInt32(),
		LastCommitHeight: RandUInt32(),
		VotingPower:      RandUInt64(),
		Accum:            int64(RandUInt64()),
	}
}

func randValidatorSet(numValidators int) *ValidatorSet {
	validators := make([]*Validator, numValidators)
	for i := 0; i < numValidators; i++ {
		validators[i] = randValidator()
	}
	return NewValidatorSet(validators)
}

func TestCopy(t *testing.T) {
	vset := randValidatorSet(10)
	vsetHash := vset.Hash()
	if len(vsetHash) == 0 {
		t.Fatalf("ValidatorSet had unexpected zero hash")
	}

	vsetCopy := vset.Copy()
	vsetCopyHash := vsetCopy.Hash()

	if !bytes.Equal(vsetHash, vsetCopyHash) {
		t.Fatalf("ValidatorSet copy had wrong hash. Orig: %X, Copy: %X", vsetHash, vsetCopyHash)
	}
}
