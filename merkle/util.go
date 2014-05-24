package merkle

import (
    "os"
    "fmt"
)

func Iterator(node Node) NodeIterator {
    stack := make([]Node, 0, 10)
    var cur Node = node
    var itr NodeIterator
    itr = func()(tn Node) {
        if len(stack) > 0 || cur != nil {
            for cur != nil {
                stack = append(stack, cur)
                cur = cur.Left(nil)
            }
            stack, cur = pop(stack)
            tn = cur
            cur = cur.Right(nil)
            return tn
        } else {
            return nil
        }
    }
    return itr
}

func pop(stack []Node) ([]Node, Node) {
    if len(stack) <= 0 {
        return stack, nil
    } else {
        return stack[0:len(stack)-1], stack[len(stack)-1]
    }
}

func PrintIAVLNode(node *IAVLNode) {
    fmt.Println("==== NODE")
    printIAVLNode(node, 0)
    fmt.Println("==== END")
}

func printIAVLNode(node *IAVLNode, indent int) {
    indentPrefix := ""
    for i:=0; i<indent; i++ {
        indentPrefix += "    "
    }
    if node == nil {
        fmt.Printf("%s--\n", indentPrefix)
    } else {
        printIAVLNode(node.leftFilled(nil), indent+1)
        fmt.Printf("%s%v:%v\n", indentPrefix, node.key, node.height)
        printIAVLNode(node.rightFilled(nil), indent+1)
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

