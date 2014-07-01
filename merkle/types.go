package merkle

import (
	"fmt"
	. "github.com/tendermint/tendermint/binary"
)

type Value interface {
	Binary
}

type Key interface {
	Binary
	Equals(Binary) bool
	Less(b Binary) bool
}

type Db interface {
	Get([]byte) []byte
	Put([]byte, []byte)
}

type Node interface {
	Binary
	Key() Key
	Value() Value
	Size() uint64
	Height() uint8
	Hash() (ByteSlice, uint64)
	Save(Db)
}

type Tree interface {
	Root() Node
	Size() uint64
	Height() uint8
	Has(key Key) bool
	Get(key Key) Value
	Hash() (ByteSlice, uint64)
	Save()
	Put(Key, Value) bool
	Remove(Key) (Value, error)
	Copy() Tree
	Traverse(func(Node) bool)
	Values() <-chan Value
}

func NotFound(key Key) error {
	return fmt.Errorf("Key was not found.")
}
