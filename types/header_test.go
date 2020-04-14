package types

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/tmhash"
	tmrand "github.com/tendermint/tendermint/libs/rand"
	"github.com/tendermint/tendermint/version"
)

func makeRandHeader() Header {
	chainID := "test"
	t := time.Now()
	height := tmrand.Int63()
	randBytes := tmrand.Bytes(tmhash.Size)
	randAddress := tmrand.Bytes(crypto.AddressSize)

	h := Header{
		Version:            version.Consensus{Block: 1, App: 1},
		ChainID:            chainID,
		Height:             height,
		Time:               t,
		LastBlockID:        BlockID{},
		LastCommitHash:     randBytes,
		DataHash:           randBytes,
		ValidatorsHash:     randBytes,
		NextValidatorsHash: randBytes,
		ConsensusHash:      randBytes,
		AppHash:            randBytes,

		LastResultsHash: randBytes,

		EvidenceHash:    randBytes,
		ProposerAddress: randAddress,
	}

	return h
}

func TestHeaderProto(t *testing.T) {
	h1 := makeRandHeader()
	tc := []struct {
		msg     string
		h1      *Header
		h2      *Header
		expPass bool
	}{
		{"failure empty Header", &Header{}, &Header{}, false},
		{"success", &h1, &Header{}, true},
		{"success Header nil", nil, nil, true}}

	for _, tt := range tc {
		tt := tt
		t.Run(tt.msg, func(t *testing.T) {
			pb := tt.h1.ToProto()
			err := tt.h2.FromProto(pb)
			if tt.expPass {
				require.NoError(t, err, tt.msg)
				require.Equal(t, tt.h1, tt.h2, tt.msg)
			} else {
				require.Error(t, err, tt.msg)
			}

		})
	}
}
