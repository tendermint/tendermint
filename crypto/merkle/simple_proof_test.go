package merkle

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	tmrand "github.com/tendermint/tendermint/libs/rand"
)

func TestSimpleProofValidateBasic(t *testing.T) {
	testCases := []struct {
		testName      string
		malleateProof func(*SimpleProof)
		errStr        string
	}{
		{"Good", func(sp *SimpleProof) {}, ""},
		{"Negative Total", func(sp *SimpleProof) { sp.Total = -1 }, "negative Total"},
		{"Negative Index", func(sp *SimpleProof) { sp.Index = -1 }, "negative Index"},
		{"Invalid LeafHash", func(sp *SimpleProof) { sp.LeafHash = make([]byte, 10) },
			"expected LeafHash size to be 32, got 10"},
		{"Too many Aunts", func(sp *SimpleProof) { sp.Aunts = make([][]byte, MaxAunts+1) },
			"expected no more than 100 aunts, got 101"},
		{"Invalid Aunt", func(sp *SimpleProof) { sp.Aunts[0] = make([]byte, 10) },
			"expected Aunts#0 size to be 32, got 10"},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.testName, func(t *testing.T) {
			_, proofs := SimpleProofsFromByteSlices([][]byte{
				[]byte("apple"),
				[]byte("watermelon"),
				[]byte("kiwi"),
			})
			tc.malleateProof(proofs[0])
			err := proofs[0].ValidateBasic()
			if tc.errStr != "" {
				assert.Contains(t, err.Error(), tc.errStr)
			}
		})
	}
}

func TestSimpleProofProtoBuf(t *testing.T) {
	testCases := []struct {
		testName string
		ps1      *SimpleProof
		expPass  bool
	}{
		{"failure empty", &SimpleProof{}, false},
		{"failure nil", nil, false},
		{"success",
			&SimpleProof{
				Total:    1,
				Index:    1,
				LeafHash: tmrand.Bytes(32),
				Aunts:    [][]byte{tmrand.Bytes(32), tmrand.Bytes(32), tmrand.Bytes(32)},
			}, true},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.testName, func(t *testing.T) {
			proto := tc.ps1.ToProto()
			p, err := SimpleProofFromProto(proto)
			if tc.expPass {
				require.NoError(t, err)
				require.Equal(t, tc.ps1, p, tc.testName)
			} else {
				require.Error(t, err, tc.testName)
			}
		})
	}
}
