package p2p

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	cmn "github.com/tendermint/tmlibs/common"
)

func TestLoadOrGenNodeKey(t *testing.T) {
	filePath := filepath.Join(os.TempDir(), cmn.RandStr(12)+"_peer_id.json")

	target := MakePoWTarget(2)
	nodeKey, err := LoadOrGenNodeKey(filePath, target)
	assert.Nil(t, err)

	nodeKey2, err := LoadOrGenNodeKey(filePath, target)
	assert.Nil(t, err)

	assert.Equal(t, nodeKey, nodeKey2)
}

func repeatBytes(val byte, n int) []byte {
	return bytes.Repeat([]byte{val}, n)
}

func TestPoWTarget(t *testing.T) {

	cases := []struct {
		difficulty uint8
		target     []byte
	}{
		{0, bytes.Repeat([]byte{255}, 20)},
		{1, append([]byte{128}, repeatBytes(255, 19)...)},
		{8, append([]byte{0}, repeatBytes(255, 19)...)},
		{9, append([]byte{0, 128}, repeatBytes(255, 18)...)},
		{10, append([]byte{0, 64}, repeatBytes(255, 18)...)},
		{16, append([]byte{0, 0}, repeatBytes(255, 18)...)},
		{17, append([]byte{0, 0, 128}, repeatBytes(255, 17)...)},
	}

	for _, c := range cases {
		assert.Equal(t, MakePoWTarget(c.difficulty), c.target)
	}

}
