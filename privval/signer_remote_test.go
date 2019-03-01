package privval

import (
	"github.com/stretchr/testify/assert"
	"github.com/tendermint/tendermint/libs/common"
	"github.com/tendermint/tendermint/types"
	"testing"
)

func TestSignerGetPubKey(t *testing.T) {
	for _, tc := range getTestCases(t) {
		func() {
			chainID := common.RandStr(12)
			mockPV := types.NewMockPV()

			validatorEndpoint, serviceEndpoint := getMockEndpoints(t, chainID, mockPV, tc.addr, tc.dialer)

			sr, err := NewSignerRemote(*validatorEndpoint)
			assert.NoError(t, err)

			defer validatorEndpoint.Stop()
			defer serviceEndpoint.Stop()

			clientKey := sr.GetPubKey()
			expectedPubKey := mockPV.GetPubKey()

			assert.Equal(t, expectedPubKey, clientKey)
		}()
	}
}
