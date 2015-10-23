package vm

import (
	"fmt"
	. "github.com/tendermint/go-common"
)

// Not goroutine safe
type Stack struct {
	data []Word256
	ptr  int

	gas *int64
	err *error
}

func NewStack(capacity int, gas *int64, err *error) *Stack {
	return &Stack{
		data: make([]Word256, capacity),
		ptr:  0,
		gas:  gas,
		err:  err,
	}
}

func (st *Stack) useGas(gasToUse int64) {
	if *st.gas > gasToUse {
		*st.gas -= gasToUse
	} else {
		st.setErr(ErrInsufficientGas)
	}
}

func (st *Stack) setErr(err error) {
	if *st.err == nil {
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

// currently only called after Sha3
func (st *Stack) PushBytes(bz []byte) {
	if len(bz) != 32 {
		PanicSanity("Invalid bytes size: expected 32")
	}
	st.Push(LeftPadWord256(bz))
}

func (st *Stack) Push64(i int64) {
	st.Push(Int64ToWord256(i))
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

func (st *Stack) Pop64() int64 {
	d := st.Pop()
	return Int64FromWord256(d)
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
	if st.ptr == 0 {
		st.setErr(ErrDataStackUnderflow)
		return Zero256
	}
	return st.data[st.ptr-1]
}

func (st *Stack) Print(n int) {
	fmt.Println("### stack ###")
	if st.ptr > 0 {
		nn := MinInt(n, st.ptr)
		for j, i := 0, st.ptr-1; i > st.ptr-1-nn; i-- {
			fmt.Printf("%-3d  %X\n", j, st.data[i])
			j += 1
		}
	} else {
		fmt.Println("-- empty --")
	}
	fmt.Println("#############")
}
