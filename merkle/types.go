package merkle

import (
    "fmt"
)

type Value interface {
    Bytes()         []byte
}

type Key interface {
    Equals(b Key)   bool
    Less(b Key)     bool
    Bytes()         []byte
}

type Tree interface {
    Root()          Node

    Size()          uint64
    Height()        uint8
    Has(key Key)    bool
    Get(key Key)    (Value, error)
    Hash()          ([]byte, uint64)

    Put(Key, Value) (err error)
    Remove(Key)     (Value, error)
}

type Node interface {
    Key()           Key
    Value()         Value
    Left()          Node
    Right()         Node

    Size()          uint64
    Height()        uint8
    Has(Key)        bool
    Get(Key)        (Value, error)
    Hash()          ([]byte, uint64)
    Bytes()         []byte

    Put(Key, Value) (*IAVLNode, bool)
    Remove(Key)     (*IAVLNode, Value, error)
}

type NodeIterator func() Node

func NotFound(key Key) error {
    return fmt.Errorf("Key was not found.")
}
