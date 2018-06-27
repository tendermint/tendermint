package types

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/tendermint/tendermint/crypto"
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
	require.Equal(t, hb.String(), "Heartbeat{1:000000000000 11/02 (0) <nil>}")

	var key crypto.PrivKeyEd25519
	sig, err := key.Sign([]byte("Tendermint"))
	require.NoError(t, err)
	hb.Signature = sig
	require.Equal(t, hb.String(), "Heartbeat{1:000000000000 11/02 (0) /FF41E371B9BF.../}")
}

func TestHeartbeatWriteSignBytes(t *testing.T) {

	hb := &Heartbeat{ValidatorIndex: 1, Height: 10, Round: 1}
	bz := hb.SignBytes("0xdeadbeef")
	// XXX HMMMMMMM
	require.Equal(t, string(bz), `{"@chain_id":"0xdeadbeef","@type":"heartbeat","height":"10","round":"1","sequence":"0","validator_address":"","validator_index":"1"}`)

	plainHb := &Heartbeat{}
	bz = plainHb.SignBytes("0xdeadbeef")
	require.Equal(t, string(bz), `{"@chain_id":"0xdeadbeef","@type":"heartbeat","height":"0","round":"0","sequence":"0","validator_address":"","validator_index":"0"}`)

	require.Panics(t, func() {
		var nilHb *Heartbeat
		bz := nilHb.SignBytes("0xdeadbeef")
		require.Equal(t, string(bz), "null")
	})
}
