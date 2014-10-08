package merkle

import (
	. "github.com/tendermint/tendermint/binary"
	. "github.com/tendermint/tendermint/common"
)

// TODO: make TypedTree work with the underlying tree to cache the decoded value.
type TypedTree struct {
	Tree       Tree
	keyCodec   Codec
	valueCodec Codec
}

func NewTypedTree(tree Tree, keyCodec, valueCodec Codec) *TypedTree {
	return &TypedTree{
		Tree:       tree,
		keyCodec:   keyCodec,
		valueCodec: valueCodec,
	}
}

func (t *TypedTree) Has(key interface{}) bool {
	bytes, err := t.keyCodec.Write(key)
	if err != nil {
		Panicf("Error from keyCodec: %v", err)
	}
	return t.Tree.Has(bytes)
}

func (t *TypedTree) Get(key interface{}) interface{} {
	keyBytes, err := t.keyCodec.Write(key)
	if err != nil {
		Panicf("Error from keyCodec: %v", err)
	}
	valueBytes := t.Tree.Get(keyBytes)
	if valueBytes == nil {
		return nil
	}
	value, err := t.valueCodec.Read(valueBytes)
	if err != nil {
		Panicf("Error from valueCodec: %v", err)
	}
	return value
}

func (t *TypedTree) Set(key interface{}, value interface{}) bool {
	keyBytes, err := t.keyCodec.Write(key)
	if err != nil {
		Panicf("Error from keyCodec: %v", err)
	}
	valueBytes, err := t.valueCodec.Write(value)
	if err != nil {
		Panicf("Error from valueCodec: %v", err)
	}
	return t.Tree.Set(keyBytes, valueBytes)
}

func (t *TypedTree) Remove(key interface{}) (interface{}, error) {
	keyBytes, err := t.keyCodec.Write(key)
	if err != nil {
		Panicf("Error from keyCodec: %v", err)
	}
	valueBytes, err := t.Tree.Remove(keyBytes)
	if valueBytes == nil {
		return nil, err
	}
	value, err_ := t.valueCodec.Read(valueBytes)
	if err_ != nil {
		Panicf("Error from valueCodec: %v", err)
	}
	return value, err
}

func (t *TypedTree) Copy() *TypedTree {
	return &TypedTree{
		Tree:       t.Tree.Copy(),
		keyCodec:   t.keyCodec,
		valueCodec: t.valueCodec,
	}
}
