package merkle

import (
    "fmt"
)

type Value interface {
    Bytes() []byte
}

type Key interface {
    Equals(b Key) bool
    Less(b Key) bool
    Bytes() []byte
}

type Tree interface {
    Root() Node

    Size() int
    Has(key Key) bool
    Get(key Key) (value Value, err error)
    Hash() ([]byte, int)

    Put(key Key, value Value) (err error)
    Remove(key Key) (value Value, err error)
}

type Node interface {
    Key()   Key
    Value() Value
    Left()  Node
    Right() Node

    Size() int
    Has(key Key) bool
    Get(key Key) (value Value, err error)
    Hash() ([]byte, int)

    Put(key Key, value Value) (_ *IAVLNode, updated bool)
    Remove(key Key) (_ *IAVLNode, value Value, err error)
}

type NodeIterator func() (node Node)

func NotFound(key Key) error {
    return fmt.Errorf("Key was not found.")
}
