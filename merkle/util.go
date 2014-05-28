package merkle

import (
    "math/big"
    "crypto/rand"
    "fmt"
)

func PrintIAVLNode(node *IAVLNode) {
    fmt.Println("==== NODE")
    if node != nil {
        printIAVLNode(node, 0)
    }
    fmt.Println("==== END")
}

func printIAVLNode(node *IAVLNode, indent int) {
    indentPrefix := ""
    for i:=0; i<indent; i++ {
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

const allRandChars = "0123456789"

func RandStr(numChars int) String {
    var res string
    for i:=0; i<numChars; i++ {
        v, err := rand.Int(rand.Reader, big.NewInt(int64(10)))
        if err != nil { panic(err) }
        randIndex := int(v.Int64())
        res = res + allRandChars[randIndex:randIndex+1]
    }
    return String(res)
}

func maxUint8(a, b uint8) uint8 {
    if a > b {
        return a
    }
    return b
}

