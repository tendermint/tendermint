package merkle

import (
	"fmt"
)

type Db interface {
	Get([]byte) []byte
	Set([]byte, []byte)
}

type Tree interface {
	Size() uint64
	Height() uint8
	Has(key []byte) bool
	Get(key []byte) []byte
	HashWithCount() ([]byte, uint64)
	Hash() []byte
	Save()
	SaveKey(string)
	Set(key []byte, vlaue []byte) bool
	Remove(key []byte) ([]byte, error)
	Copy() Tree
}

func NotFound(key []byte) error {
	return fmt.Errorf("Key was not found.")
}
