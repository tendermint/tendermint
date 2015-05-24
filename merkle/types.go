package merkle

type Tree interface {
	Size() (size uint)
	Height() (height uint8)
	Has(key interface{}) (has bool)
	Get(key interface{}) (index uint, value interface{})
	GetByIndex(index uint) (key interface{}, value interface{})
	Set(key interface{}, value interface{}) (updated bool)
	Remove(key interface{}) (value interface{}, removed bool)
	HashWithCount() (hash []byte, count uint)
	Hash() (hash []byte)
	Save() (hash []byte)
	Load(hash []byte)
	Copy() Tree
	Iterate(func(key interface{}, value interface{}) (stop bool)) (stopped bool)
}

type Hashable interface {
	Hash() []byte
}
