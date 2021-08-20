package light_test

import (
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tendermint/tendermint/crypto"
	test "github.com/tendermint/tendermint/internal/test/factory"
	"github.com/tendermint/tendermint/pkg/consensus"
	"github.com/tendermint/tendermint/pkg/light"
	"github.com/tendermint/tendermint/pkg/metadata"
	"github.com/tendermint/tendermint/version"
)

func TestLightBlockValidateBasic(t *testing.T) {
	header := test.MakeRandomHeader()
	commit := test.MakeRandomCommit(time.Now())
	vals, _ := test.RandValidatorPrivValSet(5, 1)
	header.Height = commit.Height
	header.LastBlockID = commit.BlockID
	header.ValidatorsHash = vals.Hash()
	header.Version.Block = version.BlockProtocol
	vals2, _ := test.RandValidatorPrivValSet(3, 1)
	vals3 := vals.Copy()
	vals3.Proposer = &consensus.Validator{}
	commit.BlockID.Hash = header.Hash()

	sh := &light.SignedHeader{
		Header: header,
		Commit: commit,
	}

	testCases := []struct {
		name      string
		sh        *light.SignedHeader
		vals      *consensus.ValidatorSet
		expectErr bool
	}{
		{"valid light block", sh, vals, false},
		{"hashes don't match", sh, vals2, true},
		{"invalid validator set", sh, vals3, true},
		{"invalid signed header", &light.SignedHeader{Header: header, Commit: test.MakeRandomCommit(time.Now())}, vals, true},
	}

	for _, tc := range testCases {
		lightBlock := light.LightBlock{
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
	header := test.MakeRandomHeader()
	commit := test.MakeRandomCommit(time.Now())
	vals, _ := test.RandValidatorPrivValSet(5, 1)
	header.Height = commit.Height
	header.LastBlockID = commit.BlockID
	header.Version.Block = version.BlockProtocol
	header.ValidatorsHash = vals.Hash()
	vals3 := vals.Copy()
	vals3.Proposer = &consensus.Validator{}
	commit.BlockID.Hash = header.Hash()

	sh := &light.SignedHeader{
		Header: header,
		Commit: commit,
	}

	testCases := []struct {
		name       string
		sh         *light.SignedHeader
		vals       *consensus.ValidatorSet
		toProtoErr bool
		toBlockErr bool
	}{
		{"valid light block", sh, vals, false, false},
		{"empty signed header", &light.SignedHeader{}, vals, false, false},
		{"empty validator set", sh, &consensus.ValidatorSet{}, false, true},
		{"empty light block", &light.SignedHeader{}, &consensus.ValidatorSet{}, false, true},
	}

	for _, tc := range testCases {
		lightBlock := &light.LightBlock{
			SignedHeader: tc.sh,
			ValidatorSet: tc.vals,
		}
		lbp, err := lightBlock.ToProto()
		if tc.toProtoErr {
			assert.Error(t, err, tc.name)
		} else {
			assert.NoError(t, err, tc.name)
		}

		lb, err := light.LightBlockFromProto(lbp)
		if tc.toBlockErr {
			assert.Error(t, err, tc.name)
		} else {
			assert.NoError(t, err, tc.name)
			assert.Equal(t, lightBlock, lb)
		}
	}

}

func TestSignedHeaderValidateBasic(t *testing.T) {
	commit := test.MakeRandomCommit(time.Now())
	chainID := "𠜎"
	timestamp := time.Date(math.MaxInt64, 0, 0, 0, 0, 0, math.MaxInt64, time.UTC)
	h := metadata.Header{
		Version:            version.Consensus{Block: version.BlockProtocol, App: math.MaxInt64},
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
		ProposerAddress:    crypto.AddressHash([]byte("proposer_address")),
	}

	validSignedHeader := light.SignedHeader{Header: &h, Commit: commit}
	validSignedHeader.Commit.BlockID.Hash = validSignedHeader.Hash()
	invalidSignedHeader := light.SignedHeader{}

	testCases := []struct {
		testName  string
		shHeader  *metadata.Header
		shCommit  *metadata.Commit
		expectErr bool
	}{
		{"Valid Signed Header", validSignedHeader.Header, validSignedHeader.Commit, false},
		{"Invalid Signed Header", invalidSignedHeader.Header, validSignedHeader.Commit, true},
		{"Invalid Signed Header", validSignedHeader.Header, invalidSignedHeader.Commit, true},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.testName, func(t *testing.T) {
			sh := light.SignedHeader{
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

func TestSignedHeaderProtoBuf(t *testing.T) {
	commit := test.MakeRandomCommit(time.Now())
	h := test.MakeRandomHeader()

	sh := light.SignedHeader{Header: h, Commit: commit}

	testCases := []struct {
		msg     string
		sh1     *light.SignedHeader
		expPass bool
	}{
		{"empty SignedHeader 2", &light.SignedHeader{}, true},
		{"success", &sh, true},
		{"failure nil", nil, false},
	}
	for _, tc := range testCases {
		protoSignedHeader := tc.sh1.ToProto()

		sh, err := light.SignedHeaderFromProto(protoSignedHeader)

		if tc.expPass {
			require.NoError(t, err, tc.msg)
			require.Equal(t, tc.sh1, sh, tc.msg)
		} else {
			require.Error(t, err, tc.msg)
		}
	}
}
