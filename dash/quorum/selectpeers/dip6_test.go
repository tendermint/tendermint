package selectpeers

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tendermint/tendermint/dash/quorum/mock"
	"github.com/tendermint/tendermint/libs/bytes"
	"github.com/tendermint/tendermint/types"
)

const (
	mySeed = math.MaxUint16 - 1
)

func TestDIP6(t *testing.T) {
	me := mock.NewValidator(mySeed)
	quorumHash := mock.NewQuorumHash(0xaa)

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
			me:      me,
			wantErr: true,
		},
		{
			name:       "Only me",
			validators: append([]*types.Validator{}, me),
			me:         me,
			quorumHash: quorumHash,
			wantErr:    true,
		},
		{
			name:            "4 validators",
			validators:      append(mock.NewValidators(3), me),
			me:              me,
			quorumHash:      quorumHash,
			wantLen:         3,
			wantProTxHashes: mock.NewProTxHashes(0x01, 0x03, 0x02),
		},
		{
			name:            "5 validators",
			validators:      append(mock.NewValidators(4), me),
			me:              me,
			quorumHash:      quorumHash,
			wantProTxHashes: mock.NewProTxHashes(0x03, 0x04),
			wantLen:         2,
		},
		{
			name:       "5 validators, not me",
			validators: mock.NewValidators(5),
			me:         mock.NewValidator(1000),
			quorumHash: quorumHash,
			wantErr:    true,
		},
		{
			name:            "5 validators, different quorum hash",
			validators:      append(mock.NewValidators(4), me),
			me:              me,
			quorumHash:      mock.NewQuorumHash(1),
			wantProTxHashes: mock.NewProTxHashes(0x01, 0x03),
			wantLen:         2,
		},
		{
			name:            "6 validators",
			validators:      append(mock.NewValidators(5), me),
			me:              me,
			quorumHash:      quorumHash,
			wantProTxHashes: mock.NewProTxHashes(0x03, 0x04),
			wantLen:         2,
		},
		{
			name:            "8 validators",
			validators:      append(mock.NewValidators(7), me),
			me:              me,
			quorumHash:      quorumHash,
			wantProTxHashes: mock.NewProTxHashes(0x03, 0x04),
			wantLen:         2,
		},
		{
			name:            "9 validators",
			validators:      append(mock.NewValidators(8), me),
			me:              me,
			quorumHash:      quorumHash,
			wantProTxHashes: mock.NewProTxHashes(0x03, 0x08, 0x02),
			wantLen:         3,
		},
		{
			name:            "37 validators",
			validators:      append(mock.NewValidators(36), me),
			me:              me,
			quorumHash:      quorumHash,
			wantProTxHashes: mock.NewProTxHashes(0x03, 0x0b, 0x08, 0x22, 0x1d),
			wantLen:         5,
		},
		{
			name:            "37 validators, I am last",
			validators:      mock.NewValidators(37),
			me:              mock.NewValidator(36),
			quorumHash:      quorumHash,
			wantProTxHashes: mock.NewProTxHashes(0x0a, 0x10, 0x06, 0x03, 0x02),
			wantLen:         5,
		},
		{
			name:       "37 validators, not me",
			validators: mock.NewValidators(37),
			me:         me,
			quorumHash: quorumHash,
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
