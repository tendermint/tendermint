package types

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/crypto/ed25519"
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
		expected, err := cdc.MarshalBinary(CanonicalizeHeartbeat(chainID, testHeartbeat))
		require.NoError(t, err)
		require.Equal(t, expected, signBytes, "Got unexpected sign bytes for Heartbeat")
	}

	{
		testHeartbeat := &Heartbeat{}
		signBytes := testHeartbeat.SignBytes(chainID)
		expected, err := cdc.MarshalBinary(CanonicalizeHeartbeat(chainID, testHeartbeat))
		require.NoError(t, err)
		require.Equal(t, expected, signBytes, "Got unexpected sign bytes for Heartbeat")
	}

	require.Panics(t, func() {
		var nilHb *Heartbeat
		signBytes := nilHb.SignBytes(chainID)
		require.Equal(t, string(signBytes), "null")
	})
}
