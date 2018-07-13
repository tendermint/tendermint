package merkle

import (
	"math/rand"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestKeyPath(t *testing.T) {
	var path KeyPath
	keys := make([][]byte, 10)

	for d := 0; d < 20; d++ {
		for i := range keys {
			keys[i] = make([]byte, rand.Uint32()%20)
			rand.Read(keys[i])
			enc := keyEncoding(rand.Intn(int(KeyEncodingMax)))
			path = path.AppendKey(keys[i], enc)
		}

		res, err := KeyPathToKeys(path.String())
		require.Nil(t, err)

		for i, key := range keys {
			require.Equal(t, key, res[i])
		}
	}
}
