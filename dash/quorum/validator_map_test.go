package quorum

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tendermint/tendermint/dash/quorum/mock"
	"github.com/tendermint/tendermint/types"
)

func Test_validatorMap_String(t *testing.T) {
	vals := mock.NewValidators(5)

	tests := []struct {
		vm        validatorMap
		contains  []string
		wantEmpty bool
	}{
		{
			vm:        newValidatorMap([]*types.Validator{}),
			wantEmpty: true,
		},
		{
			vm: newValidatorMap(vals[:2]),
			contains: []string{
				"tcp://0100000000000000000000000000000000000000@127.0.0.1:1",
				"tcp://0200000000000000000000000000000000000000@127.0.0.1:2",
			},
		},
		{
			vm: newValidatorMap(vals),
			contains: []string{
				"<nil> VP:0 A:0 N:tcp://0100000000000000000000000000000000000000@127.0.0.1:1}",
				"<nil> VP:0 A:0 N:tcp://0200000000000000000000000000000000000000@127.0.0.1:2}",
				"<nil> VP:0 A:0 N:tcp://0300000000000000000000000000000000000000@127.0.0.1:3}",
				"<nil> VP:0 A:0 N:tcp://0400000000000000000000000000000000000000@127.0.0.1:4}",
				"<nil> VP:0 A:0 N:tcp://0500000000000000000000000000000000000000@127.0.0.1:5}",
			},
		},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			got := tt.vm.String()
			if tt.wantEmpty {
				assert.Empty(t, got)
			}
			if len(tt.contains) > 0 {
				for key, item := range tt.contains {
					assert.Contains(t, got, item, "Item %d", key)
				}
			}
		})
	}
}
