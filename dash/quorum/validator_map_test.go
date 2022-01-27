package quorum

import (
	"hash/crc64"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tendermint/tendermint/dash/quorum/mock"
	"github.com/tendermint/tendermint/types"
)

func Test_validatorMap_String(t *testing.T) {
	vals := mock.NewValidators(5)

	tests := []struct {
		vm      validatorMap
		wantCrc uint64
	}{
		{
			vm:      newValidatorMap([]*types.Validator{}),
			wantCrc: 0x0,
		},
		{
			vm:      newValidatorMap(vals[:2]),
			wantCrc: 0x1f78f540f6d32856,
		},
		{
			vm:      newValidatorMap(vals),
			wantCrc: 0xc7ca01eea9504bdf,
		},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			got := tt.vm.String()
			crcTable := crc64.MakeTable(crc64.ISO)
			assert.EqualValues(t, tt.wantCrc, crc64.Checksum([]byte(got), crcTable), got)
		})
	}
}
