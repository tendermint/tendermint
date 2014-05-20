package merkle

import (
    "fmt"
)

type Sortable interface {
    Equals(b Sortable) bool
    Less(b Sortable) bool
}

type Tree interface {
    Root() Node

    Size() int
    Has(key Sortable) bool
    Get(key Sortable) (value interface{}, err error)

    Put(key Sortable, value interface{}) (err error)
    Remove(key Sortable) (value interface{}, err error)
}

type Node interface {
    Key() Sortable
    Value() interface{}
    Left() Node
    Right() Node

    Size() int
    Has(key Sortable) bool
    Get(key Sortable) (value interface{}, err error)

    Put(key Sortable, value interface{}) (_ *IAVLNode, updated bool)
    Remove(key Sortable) (_ *IAVLNode, value interface{}, err error)
}

type NodeIterator func() (node Node, next NodeIterator)

func NotFound(key Sortable) error {
    return fmt.Errorf("Key was not found.")
}
