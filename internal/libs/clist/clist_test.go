package clist

import (
	"fmt"
	mrand "math/rand"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPanicOnMaxLength(t *testing.T) {
	maxLength := 1000

	l := newWithMax(maxLength)
	for i := 0; i < maxLength; i++ {
		l.PushBack(1)
	}
	assert.Panics(t, func() {
		l.PushBack(1)
	})
}

func TestSmall(t *testing.T) {
	l := New()
	el1 := l.PushBack(1)
	el2 := l.PushBack(2)
	el3 := l.PushBack(3)
	if l.Len() != 3 {
		t.Error("Expected len 3, got ", l.Len())
	}

	// fmt.Printf("%p %v\n", el1, el1)
	// fmt.Printf("%p %v\n", el2, el2)
	// fmt.Printf("%p %v\n", el3, el3)

	r1 := l.Remove(el1)

	// fmt.Printf("%p %v\n", el1, el1)
	// fmt.Printf("%p %v\n", el2, el2)
	// fmt.Printf("%p %v\n", el3, el3)

	r2 := l.Remove(el2)

	// fmt.Printf("%p %v\n", el1, el1)
	// fmt.Printf("%p %v\n", el2, el2)
	// fmt.Printf("%p %v\n", el3, el3)

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

func TestGCFifo(t *testing.T) {

	const numElements = 10000
	l := New()
	gcCount := 0

	// SetFinalizer doesn't work well with circular structures,
	// so we construct a trivial non-circular structure to
	// track.
	type value struct {
		Int int
	}

	gcCh := make(chan struct{})
	for i := 0; i < numElements; i++ {
		v := new(value)
		v.Int = i
		l.PushBack(v)
		runtime.SetFinalizer(v, func(v *value) {
			gcCh <- struct{}{}
		})
	}

	for el := l.Front(); el != nil; {
		l.Remove(el)
		// oldEl := el
		el = el.Next()
		// oldEl.DetachPrev()
		// oldEl.DetachNext()
	}

	tickerQuitCh := make(chan struct{})
	tickerDoneCh := make(chan struct{})
	go func() {
		defer close(tickerDoneCh)
		ticker := time.NewTicker(250 * time.Millisecond)
		for {
			select {
			case <-ticker.C:
				runtime.GC()
			case <-tickerQuitCh:
				return
			}
		}
	}()

	for i := 0; i < numElements; i++ {
		<-gcCh
		gcCount++
	}

	close(tickerQuitCh)
	<-tickerDoneCh

	if gcCount != numElements {
		t.Errorf("expected gcCount to be %v, got %v", numElements,
			gcCount)
	}
}

func TestGCRandom(t *testing.T) {

	const numElements = 10000
	l := New()
	gcCount := 0

	// SetFinalizer doesn't work well with circular structures,
	// so we construct a trivial non-circular structure to
	// track.
	type value struct {
		Int int
	}

	gcCh := make(chan struct{})
	for i := 0; i < numElements; i++ {
		v := new(value)
		v.Int = i
		l.PushBack(v)
		runtime.SetFinalizer(v, func(v *value) {
			gcCh <- struct{}{}
		})
	}

	els := make([]*CElement, 0, numElements)
	for el := l.Front(); el != nil; el = el.Next() {
		els = append(els, el)
	}

	for _, i := range mrand.Perm(numElements) {
		el := els[i]
		l.Remove(el)
		_ = el.Next()
	}

	tickerQuitCh := make(chan struct{})
	tickerDoneCh := make(chan struct{})
	go func() {
		defer close(tickerDoneCh)
		ticker := time.NewTicker(250 * time.Millisecond)
		for {
			select {
			case <-ticker.C:
				runtime.GC()
			case <-tickerQuitCh:
				return
			}
		}
	}()

	for i := 0; i < numElements; i++ {
		<-gcCh
		gcCount++
	}

	close(tickerQuitCh)
	<-tickerDoneCh

	if gcCount != numElements {
		t.Errorf("expected gcCount to be %v, got %v", numElements,
			gcCount)
	}
}

func TestScanRightDeleteRandom(t *testing.T) {

	const numElements = 1000
	const numTimes = 100
	const numScanners = 10

	l := New()
	stop := make(chan struct{})

	els := make([]*CElement, numElements)
	for i := 0; i < numElements; i++ {
		el := l.PushBack(i)
		els[i] = el
	}

	// Launch scanner routines that will rapidly iterate over elements.
	for i := 0; i < numScanners; i++ {
		go func(scannerID int) {
			var el *CElement
			restartCounter := 0
			counter := 0
		FOR_LOOP:
			for {
				select {
				case <-stop:
					fmt.Println("stopped")
					break FOR_LOOP
				default:
				}
				if el == nil {
					el = l.frontWait()
					restartCounter++
				}
				el = el.Next()
				counter++
			}
			fmt.Printf("Scanner %v restartCounter: %v counter: %v\n", scannerID, restartCounter, counter)
		}(i)
	}

	// Remove an element, push back an element.
	for i := 0; i < numTimes; i++ {
		// Pick an element to remove
		rmElIdx := mrand.Intn(len(els))
		rmEl := els[rmElIdx]

		// Remove it
		l.Remove(rmEl)
		// fmt.Print(".")

		// Insert a new element
		newEl := l.PushBack(-1*i - 1)
		els[rmElIdx] = newEl

		if i%100000 == 0 {
			fmt.Printf("Pushed %vK elements so far...\n", i/1000)
		}

	}

	// Stop scanners
	close(stop)
	// time.Sleep(time.Second * 1)

	// And remove all the elements.
	for el := l.Front(); el != nil; el = el.Next() {
		l.Remove(el)
	}
	if l.Len() != 0 {
		t.Fatal("Failed to remove all elements from CList")
	}
}

func TestWaitChan(t *testing.T) {
	l := New()
	ch := l.WaitChan()

	// 1) add one element to an empty list
	go l.PushBack(1)
	<-ch

	// 2) and remove it
	el := l.Front()
	v := l.Remove(el)
	if v != 1 {
		t.Fatal("where is 1 coming from?")
	}

	// 3) test iterating forward and waiting for Next (NextWaitChan and Next)
	el = l.PushBack(0)

	done := make(chan struct{})
	pushed := 0
	go func() {
		defer close(done)
		for i := 1; i < 100; i++ {
			l.PushBack(i)
			pushed++
			time.Sleep(time.Duration(mrand.Intn(20)) * time.Millisecond)
		}
		// apply a deterministic pause so the counter has time to catch up
		time.Sleep(20 * time.Millisecond)
	}()

	next := el
	seen := 0
FOR_LOOP:
	for {
		select {
		case <-next.NextWaitChan():
			next = next.Next()
			seen++
			if next == nil {
				t.Fatal("Next should not be nil when waiting on NextWaitChan")
			}
		case <-done:
			break FOR_LOOP
		case <-time.After(2 * time.Second):
			t.Fatal("max execution time")
		}
	}

	if pushed != seen {
		t.Fatalf("number of pushed items (%d) not equal to number of seen items (%d)", pushed, seen)
	}

}

func TestRemoved(t *testing.T) {
	l := New()
	el1 := l.PushBack(1)
	el2 := l.PushBack(2)
	l.Remove(el1)
	require.True(t, el1.Removed())
	require.False(t, el2.Removed())
}

func TestNextWaitChan(t *testing.T) {
	l := New()
	el1 := l.PushBack(1)
	t.Run("tail element should not have a closed nextWaitChan", func(t *testing.T) {
		select {
		case <-el1.NextWaitChan():
			t.Fatal("nextWaitChan should not have been closed")
		default:
		}
	})

	el2 := l.PushBack(2)
	t.Run("adding element should close tail nextWaitChan", func(t *testing.T) {
		select {
		case <-el1.NextWaitChan():
			require.NotNil(t, el1.Next())
		default:
			t.Fatal("nextWaitChan should have been closed")
		}

		select {
		case <-el2.NextWaitChan():
			t.Fatal("nextWaitChan should not have been closed")
		default:
		}
	})

	t.Run("removing element should close its nextWaitChan", func(t *testing.T) {
		l.Remove(el2)
		select {
		case <-el2.NextWaitChan():
			require.Nil(t, el2.Next())
		default:
			t.Fatal("nextWaitChan should have been closed")
		}
	})
}
