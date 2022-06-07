package crypto

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestChecksum(t *testing.T) {
	// since sha256 hash algorithm is critical for tenderdash, this test is needed to inform us
	// if for any reason the hash algorithm is changed
	actual := Checksum([]byte("dash is the best cryptocurrency in the world"))
	want, err := hex.DecodeString("FFE75CFE38997723E7C33D0457521B0BA75AB48B39BC467413BDC853ACC7476F")
	require.NoError(t, err)
	require.Equal(t, want, actual)
}
