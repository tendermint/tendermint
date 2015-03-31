package vm

import (
	"fmt"
	. "github.com/tendermint/tendermint2/common"
)

// Not goroutine safe
type Stack struct {
	data []Word256
	ptr  int

	gas *uint64
	err *error
}

func NewStack(capacity int, gas *uint64, err *error) *Stack {
	return &Stack{
		data: make([]Word256, capacity),
		ptr:  0,
		gas:  gas,
		err:  err,
	}
}

func (st *Stack) useGas(gasToUse uint64) {
	if *st.gas > gasToUse {
		*st.gas -= gasToUse
	} else {
		st.setErr(ErrInsufficientGas)
	}
}

func (st *Stack) setErr(err error) {
	if *st.err != nil {
		*st.err = err
	}
}

func (st *Stack) Push(d Word256) {
	st.useGas(GasStackOp)
	if st.ptr == cap(st.data) {
		st.setErr(ErrDataStackOverflow)
		return
	}
	st.data[st.ptr] = d
	st.ptr++
}

func (st *Stack) PushBytes(bz []byte) {
	if len(bz) != 32 {
		panic("Invalid bytes size: expected 32")
	}
	st.Push(RightPadWord256(bz))
}

func (st *Stack) Push64(i uint64) {
	st.Push(Uint64ToWord256(i))
}

func (st *Stack) Pop() Word256 {
	st.useGas(GasStackOp)
	if st.ptr == 0 {
		st.setErr(ErrDataStackUnderflow)
		return Zero256
	}
	st.ptr--
	return st.data[st.ptr]
}

func (st *Stack) PopBytes() []byte {
	return st.Pop().Bytes()
}

func (st *Stack) Pop64() uint64 {
	return GetUint64(st.Pop().Bytes())
}

func (st *Stack) Len() int {
	return st.ptr
}

func (st *Stack) Swap(n int) {
	st.useGas(GasStackOp)
	if st.ptr < n {
		st.setErr(ErrDataStackUnderflow)
		return
	}
	st.data[st.ptr-n], st.data[st.ptr-1] = st.data[st.ptr-1], st.data[st.ptr-n]
	return
}

func (st *Stack) Dup(n int) {
	st.useGas(GasStackOp)
	if st.ptr < n {
		st.setErr(ErrDataStackUnderflow)
		return
	}
	st.Push(st.data[st.ptr-n])
	return
}

// Not an opcode, costs no gas.
func (st *Stack) Peek() Word256 {
	return st.data[st.ptr-1]
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
