package merkle

import (
	"fmt"
)

type DB interface {
	Get([]byte) []byte
	Set([]byte, []byte)
}

type Tree interface {
	Size() uint64
	Height() uint8
	Has(key []byte) bool
	Get(key []byte) []byte
	Set(key []byte, value []byte) bool
	Remove(key []byte) ([]byte, error)
	HashWithCount() ([]byte, uint64)
	Hash() []byte
	Save() []byte
	Copy() Tree
}

func NotFound(key []byte) error {
	return fmt.Errorf("Key was not found.")
}

type Hashable interface {
	Hash() []byte
}
