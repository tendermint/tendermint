package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/crypto/ed25519"
	"github.com/tendermint/tendermint/crypto/secp256k1"
)

func TestHeartbeatCopy(t *testing.T) {
	hb := &Heartbeat{ValidatorIndex: 1, Height: 10, Round: 1}
	hbCopy := hb.Copy()
	require.Equal(t, hbCopy, hb, "heartbeat copy should be the same")
	hbCopy.Round = hb.Round + 10
	require.NotEqual(t, hbCopy, hb, "heartbeat copy mutation should not change original")

	var nilHb *Heartbeat
	nilHbCopy := nilHb.Copy()
	require.Nil(t, nilHbCopy, "copy of nil should also return nil")
}

func TestHeartbeatString(t *testing.T) {
	var nilHb *Heartbeat
	require.Contains(t, nilHb.String(), "nil", "expecting a string and no panic")

	hb := &Heartbeat{ValidatorIndex: 1, Height: 11, Round: 2}
	require.Equal(t, "Heartbeat{1:000000000000 11/02 (0) /000000000000.../}", hb.String())

	var key ed25519.PrivKeyEd25519
	sig, err := key.Sign([]byte("Tendermint"))
	require.NoError(t, err)
	hb.Signature = sig
	require.Equal(t, "Heartbeat{1:000000000000 11/02 (0) /FF41E371B9BF.../}", hb.String())
}

func TestHeartbeatWriteSignBytes(t *testing.T) {
	chainID := "test_chain_id"

	{
		testHeartbeat := &Heartbeat{ValidatorIndex: 1, Height: 10, Round: 1}
		signBytes := testHeartbeat.SignBytes(chainID)
		expected, err := cdc.MarshalBinaryLengthPrefixed(CanonicalizeHeartbeat(chainID, testHeartbeat))
		require.NoError(t, err)
		require.Equal(t, expected, signBytes, "Got unexpected sign bytes for Heartbeat")
	}

	{
		testHeartbeat := &Heartbeat{}
		signBytes := testHeartbeat.SignBytes(chainID)
		expected, err := cdc.MarshalBinaryLengthPrefixed(CanonicalizeHeartbeat(chainID, testHeartbeat))
		require.NoError(t, err)
		require.Equal(t, expected, signBytes, "Got unexpected sign bytes for Heartbeat")
	}

	require.Panics(t, func() {
		var nilHb *Heartbeat
		signBytes := nilHb.SignBytes(chainID)
		require.Equal(t, string(signBytes), "null")
	})
}

func TestHeartbeatValidateBasic(t *testing.T) {
	testCases := []struct {
		testName          string
		malleateHeartBeat func(*Heartbeat)
		expectErr         bool
	}{
		{"Good HeartBeat", func(hb *Heartbeat) {}, false},
		{"Invalid address size", func(hb *Heartbeat) {
			hb.ValidatorAddress = nil
		}, true},
		{"Negative validator index", func(hb *Heartbeat) {
			hb.ValidatorIndex = -1
		}, true},
		{"Negative height", func(hb *Heartbeat) {
			hb.Height = -1
		}, true},
		{"Negative round", func(hb *Heartbeat) {
			hb.Round = -1
		}, true},
		{"Negative sequence", func(hb *Heartbeat) {
			hb.Sequence = -1
		}, true},
		{"Missing signature", func(hb *Heartbeat) {
			hb.Signature = nil
		}, true},
		{"Signature too big", func(hb *Heartbeat) {
			hb.Signature = make([]byte, MaxSignatureSize+1)
		}, true},
	}
	for _, tc := range testCases {
		t.Run(tc.testName, func(t *testing.T) {
			hb := &Heartbeat{
				ValidatorAddress: secp256k1.GenPrivKey().PubKey().Address(),
				Signature:        make([]byte, 4),
				ValidatorIndex:   1, Height: 10, Round: 1}

			tc.malleateHeartBeat(hb)
			assert.Equal(t, tc.expectErr, hb.ValidateBasic() != nil, "Validate Basic had an unexpected result")
		})
	}
}
