package merkle

type Tree interface {
	Size() (size uint64)
	Height() (height uint8)
	Has(key interface{}) (has bool)
	Get(key interface{}) (index uint64, value interface{})
	GetByIndex(index uint64) (key interface{}, value interface{})
	Set(key interface{}, value interface{}) (updated bool)
	Remove(key interface{}) (value interface{}, removed bool)
	HashWithCount() (hash []byte, count uint64)
	Hash() (hash []byte)
	Save() (hash []byte)
	Checkpoint() (checkpoint interface{})
	Restore(checkpoint interface{})
}

type Hashable interface {
	Hash() []byte
}
