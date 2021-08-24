package light_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

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

	sh := &metadata.SignedHeader{
		Header: header,
		Commit: commit,
	}

	testCases := []struct {
		name      string
		sh        *metadata.SignedHeader
		vals      *consensus.ValidatorSet
		expectErr bool
	}{
		{"valid light block", sh, vals, false},
		{"hashes don't match", sh, vals2, true},
		{"invalid validator set", sh, vals3, true},
		{"invalid signed header", &metadata.SignedHeader{
			Header: header,
			Commit: test.MakeRandomCommit(time.Now()),
		}, vals, true},
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

	sh := &metadata.SignedHeader{
		Header: header,
		Commit: commit,
	}

	testCases := []struct {
		name       string
		sh         *metadata.SignedHeader
		vals       *consensus.ValidatorSet
		toProtoErr bool
		toBlockErr bool
	}{
		{"valid light block", sh, vals, false, false},
		{"empty signed header", &metadata.SignedHeader{}, vals, false, false},
		{"empty validator set", sh, &consensus.ValidatorSet{}, false, true},
		{"empty light block", &metadata.SignedHeader{}, &consensus.ValidatorSet{}, false, true},
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
