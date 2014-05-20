package merkle

func Iterator(node Node) NodeIterator {
    stack := make([]Node, 0, 10)
    var cur Node = node
    var tn_iterator NodeIterator
    tn_iterator = func()(tn Node, next NodeIterator) {
        if len(stack) > 0 || cur != nil {
            for cur != nil {
                stack = append(stack, cur)
                cur = cur.Left()
            }
            stack, cur = pop(stack)
            tn = cur
            cur = cur.Right()
            return tn, tn_iterator
        } else {
            return nil, nil
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
