package proto

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/crypto/ed25519"
)

func TestEncodingPrivEd25519(t *testing.T) {
	pKey := ed25519.GenPrivKey()
	bz := pKey.Bytes()

	bz1, err := MarshalPrivKey(pKey)
	require.NoError(t, err)

	p, err := UnmarshalPrivKey(bz1)
	require.NoError(t, err)

	bz2 := p.Bytes()
	require.Equal(t, bz, bz2)
	require.Equal(t, p.PubKey(), pKey.PubKey())
}
