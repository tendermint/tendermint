package types

import (
	"math"
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/tendermint/tendermint/crypto"
	tmversion "github.com/tendermint/tendermint/proto/tendermint/version"
	"github.com/tendermint/tendermint/version"
)

func TestLightBlockValidateBasic(t *testing.T) {
	header := makeRandHeader()
	stateID := RandStateID()
	commit := randCommit(stateID)
	vals, _ := GenerateValidatorSet(5)
	header.Height = commit.Height
	header.LastBlockID = commit.BlockID
	header.ValidatorsHash = vals.Hash()
	header.Version.Block = version.BlockProtocol
	vals2, _ := GenerateValidatorSet(3)
	vals3 := vals.Copy()
	vals3.Proposer = &Validator{}
	commit.BlockID.Hash = header.Hash()

	sh := &SignedHeader{
		Header: &header,
		Commit: commit,
	}

	testCases := []struct {
		name      string
		sh        *SignedHeader
		vals      *ValidatorSet
		expectErr bool
	}{
		{"valid light block", sh, vals, false},
		{"hashes don't match", sh, vals2, true},
		{"invalid validator set", sh, vals3, true},
		{"invalid signed header", &SignedHeader{Header: &header, Commit: randCommit(stateID)}, vals, true},
	}

	for _, tc := range testCases {
		lightBlock := LightBlock{
			SignedHeader: tc.sh,
			ValidatorSet: tc.vals,
		}
		err := lightBlock.ValidateBasic(header.ChainID)
		if tc.expectErr {
			assert.Error(t, err, tc.name)
		} else {
			assert.NoError(t, err, tc.name)
		}
	}

}

func TestLightBlockProtobuf(t *testing.T) {
	header := makeRandHeader()
	commit := randCommit(RandStateID())
	vals, _ := GenerateValidatorSet(5)
	header.Height = commit.Height
	header.LastBlockID = commit.BlockID
	header.Version.Block = version.BlockProtocol
	header.ValidatorsHash = vals.Hash()
	vals3 := vals.Copy()
	vals3.Proposer = &Validator{}
	commit.BlockID.Hash = header.Hash()
	commit.QuorumHash = vals.QuorumHash

	sh := &SignedHeader{
		Header: &header,
		Commit: commit,
	}

	testCases := []struct {
		name       string
		sh         *SignedHeader
		vals       *ValidatorSet
		toProtoErr bool
		toBlockErr bool
	}{
		{"valid light block", sh, vals, false, false},
		{"empty signed header", &SignedHeader{}, vals, false, false},
		{"empty validator set", sh, &ValidatorSet{}, false, true},
		{"empty light block", &SignedHeader{}, &ValidatorSet{}, false, true},
	}

	for _, tc := range testCases {
		lightBlock := &LightBlock{
			SignedHeader: tc.sh,
			ValidatorSet: tc.vals,
		}
		lbp, err := lightBlock.ToProto()
		if tc.toProtoErr {
			assert.Error(t, err, tc.name)
		} else {
			assert.NoError(t, err, tc.name)
		}

		lb, err := LightBlockFromProto(lbp)
		if tc.toBlockErr {
			assert.Error(t, err, tc.name)
		} else {
			assert.NoError(t, err, tc.name)
			assert.Equal(t, lightBlock, lb)
		}
	}

}

func TestSignedHeaderValidateBasic(t *testing.T) {
	commit := randCommit(RandStateID())
	chainID := "𠜎"
	timestamp := time.Date(math.MaxInt64, 0, 0, 0, 0, 0, math.MaxInt64, time.UTC)
	h := Header{
		Version:            tmversion.Consensus{Block: version.BlockProtocol, App: math.MaxInt64},
		ChainID:            chainID,
		Height:             commit.Height,
		Time:               timestamp,
		LastBlockID:        commit.BlockID,
		LastCommitHash:     commit.Hash(),
		DataHash:           commit.Hash(),
		ValidatorsHash:     commit.Hash(),
		NextValidatorsHash: commit.Hash(),
		ConsensusHash:      commit.Hash(),
		AppHash:            commit.Hash(),
		LastResultsHash:    commit.Hash(),
		EvidenceHash:       commit.Hash(),
		ProposerProTxHash:  crypto.ProTxHashFromSeedBytes([]byte("proposer_pro_tx_hash")),
	}

	validSignedHeader := SignedHeader{Header: &h, Commit: commit}
	validSignedHeader.Commit.BlockID.Hash = validSignedHeader.Hash()
	invalidSignedHeader := SignedHeader{}

	testCases := []struct {
		testName  string
		shHeader  *Header
		shCommit  *Commit
		expectErr bool
	}{
		{"Valid Signed Header", validSignedHeader.Header, validSignedHeader.Commit, false},
		{"Invalid Signed Header", invalidSignedHeader.Header, validSignedHeader.Commit, true},
		{"Invalid Signed Header", validSignedHeader.Header, invalidSignedHeader.Commit, true},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.testName, func(t *testing.T) {
			sh := SignedHeader{
				Header: tc.shHeader,
				Commit: tc.shCommit,
			}
			err := sh.ValidateBasic(validSignedHeader.Header.ChainID)
			assert.Equalf(
				t,
				tc.expectErr,
				err != nil,
				"Validate Basic had an unexpected result",
				err,
			)
		})
	}
}

func TestLightBlock_StateID(t *testing.T) {

	tests := []struct {
		name        string
		commit      *Commit
		want        StateID
		shouldPanic bool
	}{
		{
			"State ID OK",
			randCommit(StateID{12, []byte("12345678901234567890123456789012")}),
			StateID{12, []byte("12345678901234567890123456789012")},
			false,
		},
		{
			"Short app hash",
			randCommit(StateID{12, []byte("12345678901234567890")}),
			StateID{12, []byte("12345678901234567890")},
			false,
		},
		{
			"Nil app hash",
			randCommit(StateID{12, nil}),
			StateID{12, []byte{}},
			false,
		},
	}
	// nolint:scopelint
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lb := LightBlock{
				SignedHeader: &SignedHeader{Commit: tt.commit},
			}
			if got := lb.StateID(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("LightBlock.StateID() = %v, want %v", got, tt.want)
			}
		})
	}
}
func TestLightBlock_StateID_nocommit(t *testing.T) {
	lb := LightBlock{}
	assert.Panics(t, func() { lb.StateID() })
}
