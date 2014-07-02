package merkle

import (
	"crypto/sha256"
	"fmt"

	. "github.com/tendermint/tendermint/binary"
)

/*
Compute a deterministic merkle hash from a list of byteslices.
*/
func HashFromBinarySlice(items []Binary) ByteSlice {
	switch len(items) {
	case 0:
		panic("Cannot compute hash of empty slice")
	case 1:
		hasher := sha256.New()
		_, err := items[0].WriteTo(hasher)
		if err != nil {
			panic(err)
		}
		return ByteSlice(hasher.Sum(nil))
	default:
		hasher := sha256.New()
		_, err := HashFromBinarySlice(items[0 : len(items)/2]).WriteTo(hasher)
		if err != nil {
			panic(err)
		}
		_, err = HashFromBinarySlice(items[len(items)/2:]).WriteTo(hasher)
		if err != nil {
			panic(err)
		}
		return ByteSlice(hasher.Sum(nil))
	}
}

func PrintIAVLNode(node *IAVLNode) {
	fmt.Println("==== NODE")
	if node != nil {
		printIAVLNode(node, 0)
	}
	fmt.Println("==== END")
}

func printIAVLNode(node *IAVLNode, indent int) {
	indentPrefix := ""
	for i := 0; i < indent; i++ {
		indentPrefix += "    "
	}

	if node.right != nil {
		printIAVLNode(node.rightFilled(nil), indent+1)
	}

	fmt.Printf("%s%v:%v\n", indentPrefix, node.key, node.height)

	if node.left != nil {
		printIAVLNode(node.leftFilled(nil), indent+1)
	}

}

func maxUint8(a, b uint8) uint8 {
	if a > b {
		return a
	}
	return b
}
