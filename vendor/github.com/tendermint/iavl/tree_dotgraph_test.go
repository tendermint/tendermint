package iavl

import (
	"io/ioutil"
	"testing"
)

func TestWriteDOTGraph(t *testing.T) {
	var tree *Tree = NewTree(0, nil)
	for _, ikey := range []byte{
		0x0a, 0x11, 0x2e, 0x32, 0x50, 0x72, 0x99, 0xa1, 0xe4, 0xf7,
	} {
		key := []byte{ikey}
		tree.Set(key, key)
	}
	WriteDOTGraph(ioutil.Discard, tree, []*PathToKey{})
}
