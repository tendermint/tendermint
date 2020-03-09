package remotedb

import (
	"fmt"

	db "github.com/tendermint/tm-db"
	protodb "github.com/tendermint/tm-db/remotedb/proto"
)

func makeIterator(dic protodb.DB_IteratorClient) db.Iterator {
	return &iterator{dic: dic}
}

func makeReverseIterator(dric protodb.DB_ReverseIteratorClient) db.Iterator {
	return &reverseIterator{dric: dric}
}

type reverseIterator struct {
	dric protodb.DB_ReverseIteratorClient
	cur  *protodb.Iterator
}

var _ db.Iterator = (*iterator)(nil)

// Valid implements Iterator.
func (rItr *reverseIterator) Valid() bool {
	return rItr.cur != nil && rItr.cur.Valid
}

// Domain implements Iterator.
func (rItr *reverseIterator) Domain() (start, end []byte) {
	if rItr.cur == nil || rItr.cur.Domain == nil {
		return nil, nil
	}
	return rItr.cur.Domain.Start, rItr.cur.Domain.End
}

// Next implements Iterator.
func (rItr *reverseIterator) Next() {
	var err error
	rItr.cur, err = rItr.dric.Recv()
	if err != nil {
		panic(fmt.Sprintf("RemoteDB.ReverseIterator.Next error: %v", err))
	}
}

// Key implements Iterator.
func (rItr *reverseIterator) Key() []byte {
	if rItr.cur == nil {
		panic("key does not exist")
	}
	return rItr.cur.Key
}

// Value implements Iterator.
func (rItr *reverseIterator) Value() []byte {
	if rItr.cur == nil {
		panic("key does not exist")
	}
	return rItr.cur.Value
}

// Error implements Iterator.
func (rItr *reverseIterator) Error() error {
	return nil
}

// Close implements Iterator.
func (rItr *reverseIterator) Close() {}

// iterator implements the db.Iterator by retrieving
// streamed iterators from the remote backend as
// needed. It is NOT safe for concurrent usage,
// matching the behavior of other iterators.
type iterator struct {
	dic protodb.DB_IteratorClient
	cur *protodb.Iterator
}

var _ db.Iterator = (*iterator)(nil)

// Valid implements Iterator.
func (itr *iterator) Valid() bool {
	return itr.cur != nil && itr.cur.Valid
}

// Domain implements Iterator.
func (itr *iterator) Domain() (start, end []byte) {
	if itr.cur == nil || itr.cur.Domain == nil {
		return nil, nil
	}
	return itr.cur.Domain.Start, itr.cur.Domain.End
}

// Next implements Iterator.
func (itr *iterator) Next() {
	var err error
	itr.cur, err = itr.dic.Recv()
	if err != nil {
		panic(fmt.Sprintf("remoteDB.Iterator.Next error: %v", err))
	}
}

// Key implements Iterator.
func (itr *iterator) Key() []byte {
	if itr.cur == nil {
		return nil
	}
	return itr.cur.Key
}

// Value implements Iterator.
func (itr *iterator) Value() []byte {
	if itr.cur == nil {
		panic("current poisition is not valid")
	}
	return itr.cur.Value
}

// Error implements Iterator.
func (itr *iterator) Error() error {
	return nil
}

// Close implements Iterator.
func (itr *iterator) Close() {
	err := itr.dic.CloseSend()
	if err != nil {
		panic(fmt.Sprintf("Error closing iterator: %v", err))
	}
}
