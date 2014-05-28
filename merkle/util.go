package merkle

import (
    "os"
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

func randstr(length int) String {
    if urandom, err := os.Open("/dev/urandom"); err != nil {
        panic(err)
    } else {
        slice := make([]byte, length)
        if _, err := urandom.Read(slice); err != nil {
            panic(err)
        }
        urandom.Close()
        return String(slice)
    }
    panic("unreachable")
}

func maxUint8(a, b uint8) uint8 {
    if a > b {
        return a
    }
    return b
}

