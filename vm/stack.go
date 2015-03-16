package vm

import (
	"encoding/binary"
	"errors"
	"fmt"
)

var (
	ErrDataStackOverflow  = errors.New("DataStackOverflow")
	ErrDataStackUnderflow = errors.New("DataStackUnderflow")
)

type Word [4]uint64

func Bytes2Uint64(bz []byte) uint64 {
	return binary.LittleEndian.Uint64(bz)
}

func Uint642Bytes(dest []byte, i uint64) {
	binary.LittleEndian.PutUint64(dest, i)
}

// Not goroutine safe
type Stack struct {
	data []Word
	ptr  int

	gas *uint64
	err *error
}

func NewStack(capacity int, gas *uint64, err *error) *Stack {
	return &Stack{
		data: make([]Word, capacity),
		ptr:  0,
		gas:  gas,
		err:  err,
	}
}

func (st *Stack) Push(d Word) error {
	if st.ptr == cap(st.data) {
		return ErrDataStackOverflow
	}
	st.data[st.ptr] = d
	st.ptr++
}

func (st *Stack) Push64(i uint64) error {
	if st.ptr == cap(st.data) {
		return ErrDataStackOverflow
	}
	st.data[st.ptr] = [4]uint64{i, 0, 0, 0}
	st.ptr++
}

func (st *Stack) PushBytes(bz []byte) error {
	if len(bz) != 32 {
		panic("Invalid bytes size: expected 32")
	}
	if st.ptr == cap(st.data) {
		return ErrDataStackOverflow
	}
	st.data[st.ptr] = [4]uint64{
		Bytes2Uint64(bz[0:8]),
		Bytes2Uint64(bz[8:16]),
		Bytes2Uint64(bz[16:24]),
		Bytes2Uint64(bz[24:32]),
	}
	st.ptr++
}

func (st *Stack) Pop() (Word, error) {
	if st.ptr == 0 {
		return Zero, ErrDataStackUnderflow
	}
	st.ptr--
	return st.data[st.ptr], nil
}

func (st *Stack) Pop64() (uint64, error) {
	if st.ptr == 0 {
		return Zero, ErrDataStackUnderflow
	}
	st.ptr--
	return st.data[st.ptr][0], nil
}

func (st *Stack) PopBytes() ([]byte, error) {
	if st.ptr == 0 {
		return Zero, ErrDataStackUnderflow
	}
	st.ptr--
	res := make([]byte, 32)
	copy(res[0:8], Uint642Bytes(st.data[st.ptr][0]))
	copy(res[8:16], Uint642Bytes(st.data[st.ptr][1]))
	copy(res[16:24], Uint642Bytes(st.data[st.ptr][2]))
	copy(res[24:32], Uint642Bytes(st.data[st.ptr][3]))
	return res, nil
}

func (st *Stack) Len() int {
	return st.ptr
}

func (st *Stack) Swap(n int) error {
	if st.ptr < n {
		return ErrDataStackUnderflow
	}
	st.data[st.ptr-n], st.data[st.ptr-1] = st.data[st.ptr-1], st.data[st.ptr-n]
	return nil
}

func (st *Stack) Dup(n int) {
	st.Push(st.data[st.ptr-n])
}

func (st *Stack) Peek() Word {
	return st.data[st.ptr-1]
}

func (st *Stack) Require(n int) error {
	if st.ptr < n {
		return ErrDataStackUnderflow
	}
}

func (st *Stack) Print() {
	fmt.Println("### stack ###")
	if st.ptr > 0 {
		for i, val := range st.data {
			fmt.Printf("%-3d  %v\n", i, val)
		}
	} else {
		fmt.Println("-- empty --")
	}
	fmt.Println("#############")
}
