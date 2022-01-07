package selectpeers

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tendermint/tendermint/dash/quorum/mock"
	"github.com/tendermint/tendermint/libs/bytes"
	"github.com/tendermint/tendermint/types"
)

func TestDIP6(t *testing.T) {
	tests := []struct {
		name            string
		validators      []*types.Validator
		me              *types.Validator
		quorumHash      bytes.HexBytes
		wantProTxHashes []bytes.HexBytes
		wantLen         int
		wantErr         bool
	}{
		{
			name:    "No validators",
			me:      mock.NewValidator(0),
			wantErr: true,
		},
		{
			name:       "Only me",
			validators: mock.NewValidators(1),
			me:         mock.NewValidator(0),
			quorumHash: mock.NewQuorumHash(0),
			wantErr:    true,
		},
		{
			name:            "4 validators",
			validators:      mock.NewValidators(4),
			me:              mock.NewValidator(0),
			quorumHash:      mock.NewQuorumHash(0),
			wantLen:         3,
			wantProTxHashes: mock.NewProTxHashes(0x01, 0x02, 0x03),
		},
		{
			name:            "5 validators",
			validators:      mock.NewValidators(5),
			me:              mock.NewValidator(0),
			quorumHash:      mock.NewQuorumHash(0),
			wantProTxHashes: mock.NewProTxHashes(0x01, 0x02),
			wantLen:         2,
		},
		{
			name:       "5 validators, not me",
			validators: mock.NewValidators(5),
			me:         mock.NewValidator(1000),
			quorumHash: mock.NewQuorumHash(0),
			wantErr:    true,
		},
		{
			name:            "5 validators, different quorum hash",
			validators:      mock.NewValidators(5),
			me:              mock.NewValidator(0),
			quorumHash:      mock.NewQuorumHash(1),
			wantProTxHashes: mock.NewProTxHashes(0x01, 0x03),
			wantLen:         2,
		},
		{
			name:            "8 validators",
			validators:      mock.NewValidators(6),
			me:              mock.NewValidator(0),
			quorumHash:      mock.NewQuorumHash(0),
			wantProTxHashes: mock.NewProTxHashes(0x01, 0x02),
			wantLen:         2,
		},
		{
			name:            "9 validators",
			validators:      mock.NewValidators(9),
			me:              mock.NewValidator(0),
			quorumHash:      mock.NewQuorumHash(0),
			wantProTxHashes: mock.NewProTxHashes(0x06, 0x01, 0x02),
			wantLen:         3,
		},
		{
			name:            "37 validators",
			validators:      mock.NewValidators(37),
			me:              mock.NewValidator(0),
			quorumHash:      mock.NewQuorumHash(0),
			wantProTxHashes: mock.NewProTxHashes(0xc, 0xa, 0x15, 0x19, 0x1d),
			wantLen:         5,
		},
		{
			name:            "37 validators, I am last",
			validators:      mock.NewValidators(37),
			me:              mock.NewValidator(36),
			quorumHash:      mock.NewQuorumHash(0),
			wantProTxHashes: mock.NewProTxHashes(0x08, 0x1b, 0x22, 0x23, 0x16),
			wantLen:         5,
		},
		{
			name:       "37 validators, not me",
			validators: mock.NewValidators(37),
			me:         mock.NewValidator(37),
			quorumHash: mock.NewQuorumHash(0),
			wantErr:    true,
		},
	}

	// nolint:scopelint
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewDIP6ValidatorSelector(tt.quorumHash).SelectValidators(tt.validators, tt.me)
			if (err == nil) == tt.wantErr {
				assert.FailNow(t, "unexpected error: %s", err)
			}
			if tt.wantProTxHashes != nil {
				assert.EqualValues(t, tt.wantProTxHashes, mock.ValidatorsProTxHashes(got))
			}
			if tt.wantLen > 0 {
				assert.Len(t, got, tt.wantLen)
			}
		})
	}
}
