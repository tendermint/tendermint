package merkle

type Tree interface {
	Size() (size int)
	Height() (height int8)
	Has(key []byte) (has bool)
	Get(key []byte) (index int, value []byte, exists bool)
	GetByIndex(index int) (key []byte, value []byte)
	Set(key []byte, value []byte) (updated bool)
	Remove(key []byte) (value []byte, removed bool)
	HashWithCount() (hash []byte, count int)
	Hash() (hash []byte)
	Save() (hash []byte)
	Load(hash []byte)
	Copy() Tree
	Iterate(func(key []byte, value []byte) (stop bool)) (stopped bool)
}

type Hashable interface {
	Hash() []byte
}
