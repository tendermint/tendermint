package merkle

type Tree interface {
	Size() (size uint64)
	Height() (height uint8)
	Has(key interface{}) (has bool)
	Get(key interface{}) (index uint64, value interface{})
	GetByIndex(index uint64) (key interface{}, value interface{})
	Set(key interface{}, value interface{}) (updated bool)
	Remove(key interface{}) (value interface{}, removed error)
	HashWithCount() (hash []byte, count uint64)
	Hash() (hash []byte)
	Save() (hash []byte)
	Load(hash []byte)
	Copy() Tree
	Iterate(func(key interface{}, value interface{}) (stop bool)) (stopped bool)
}

type Hashable interface {
	Hash() []byte
}
