package merkle

import (
    "fmt"
)

type Binary interface {
    ByteSize()      int
    WriteTo([]byte)  int
    Equals(Binary)  bool
}

type Value interface {
    Binary
}

type Key interface {
    Binary
    Less(b Key)     bool
}

type Db interface {
    Get([]byte) []byte
    Put([]byte, []byte)
}

type Node interface {
    Binary
    Key()           Key
    Value()         Value
    Size()          uint64
    Height()        uint8
    Hash()          (ByteSlice, uint64)
    Save(Db)
}

type Tree interface {
    Root()          Node
    Size()          uint64
    Height()        uint8
    Has(key Key)    bool
    Get(key Key)    Value
    Hash()          (ByteSlice, uint64)
    Save()
    Put(Key, Value) bool
    Remove(Key)     (Value, error)
    Traverse(func(Node)bool)
}

func NotFound(key Key) error {
    return fmt.Errorf("Key was not found.")
}
