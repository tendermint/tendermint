package vm

import (
	"fmt"
)

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

func (st *Stack) Push(d Word) {
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
	st.Push(BytesToWord(bz))
}

func (st *Stack) Push64(i uint64) {
	st.Push(Uint64ToWord(i))
}

func (st *Stack) Pop() Word {
	st.useGas(GasStackOp)
	if st.ptr == 0 {
		st.setErr(ErrDataStackUnderflow)
		return Zero
	}
	st.ptr--
	return st.data[st.ptr]
}

func (st *Stack) PopBytes() []byte {
	return st.Pop().Bytes()
}

func (st *Stack) Pop64() uint64 {
	return GetUint64(st.Pop())
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
func (st *Stack) Peek() Word {
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
