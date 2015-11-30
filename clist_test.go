package clist

import (
	"runtime"
	"testing"
	"time"
)

func TestSmall(t *testing.T) {
	l := New()
	el1 := l.PushBack(1)
	el2 := l.PushBack(2)
	el3 := l.PushBack(3)
	if l.Len() != 3 {
		t.Error("Expected len 3, got ", l.Len())
	}

	//fmt.Printf("%p %v\n", el1, el1)
	//fmt.Printf("%p %v\n", el2, el2)
	//fmt.Printf("%p %v\n", el3, el3)

	r1 := l.Remove(el1)

	//fmt.Printf("%p %v\n", el1, el1)
	//fmt.Printf("%p %v\n", el2, el2)
	//fmt.Printf("%p %v\n", el3, el3)

	r2 := l.Remove(el2)

	//fmt.Printf("%p %v\n", el1, el1)
	//fmt.Printf("%p %v\n", el2, el2)
	//fmt.Printf("%p %v\n", el3, el3)

	r3 := l.Remove(el3)

	if r1 != 1 {
		t.Error("Expected 1, got ", r1)
	}
	if r2 != 2 {
		t.Error("Expected 2, got ", r2)
	}
	if r3 != 3 {
		t.Error("Expected 3, got ", r3)
	}
	if l.Len() != 0 {
		t.Error("Expected len 0, got ", l.Len())
	}

}

/*
This test is quite hacky because it relies on SetFinalizer
which isn't guaranteed to run at all.
*/
func TestGCFifo(t *testing.T) {

	const numElements = 1000000
	l := New()
	gcCount := 0

	// SetFinalizer doesn't work well with circular structures,
	// so we construct a trivial non-circular structure to
	// track.
	type value struct {
		Int int
	}

	for i := 0; i < numElements; i++ {
		v := new(value)
		v.Int = i
		l.PushBack(v)
		runtime.SetFinalizer(v, func(v *value) {
			gcCount += 1
		})
	}

	for el := l.Front(); el != nil; {
		l.Remove(el)
		//oldEl := el
		el = el.Next()
		//oldEl.DetachPrev()
		//oldEl.DetachNext()
	}

	runtime.GC()
	time.Sleep(time.Second * 3)

	if gcCount != numElements {
		t.Errorf("Expected gcCount to be %v, got %v", numElements,
			gcCount)
	}
}
