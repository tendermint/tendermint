package merkle

import (
    "fmt"
)

func Iterator(node Node) NodeIterator {
    stack := make([]Node, 0, 10)
    var cur Node = node
    var tn_iterator NodeIterator
    tn_iterator = func()(tn Node) {
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
    return tn_iterator
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
        printIAVLNode(node.left_filled(nil), indent+1)
        fmt.Printf("%s%v:%v\n", indentPrefix, node.key, node.height)
        printIAVLNode(node.right_filled(nil), indent+1)
    }
}
