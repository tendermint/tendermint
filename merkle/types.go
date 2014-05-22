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

type Db interface {
    Get([]byte) ([]byte, error)
    Put([]byte, []byte) error
}

type Node interface {
    Key()           Key
    Value()         Value
    Left(Db)        Node
    Right(Db)       Node

    Size()          uint64
    Height()        uint8
    Has(Db, Key)    bool
    Get(Db, Key)    (Value, error)
    Hash()          ([]byte, uint64)
    Bytes()         []byte

    Put(Db, Key, Value) (*IAVLNode, bool)
    Remove(Db, Key) (*IAVLNode, Value, error)
}

type NodeIterator func() Node

func NotFound(key Key) error {
    return fmt.Errorf("Key was not found.")
}
