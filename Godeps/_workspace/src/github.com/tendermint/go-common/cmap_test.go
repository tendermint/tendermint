package common

import (
	"fmt"
	"math/rand"
	"testing"
)

func TestMap(t *testing.T) {

	const numElements = 10000
	const numTimes = 10000000
	const numScanners = 10

	els := make(map[int]struct{})
	for i := 0; i < numElements; i++ {
		els[i] = struct{}{}
	}

	// Remove an element, push back an element.
	for i := 0; i < numTimes; i++ {
		// Pick an element to remove
		rmElIdx := rand.Intn(len(els))
		delete(els, rmElIdx)

		// Insert a new element
		els[rmElIdx] = struct{}{}

		if i%100000 == 0 {
			fmt.Printf("Pushed %vK elements so far...\n", i/1000)
		}

	}
}

func TestCMap(t *testing.T) {

	const numElements = 10000
	const numTimes = 10000000
	const numScanners = 10

	els := NewCMapInt()
	for i := 0; i < numElements; i++ {
		els.Set(i, struct{}{})
	}

	// Launch scanner routines that will rapidly iterate over elements.
	for i := 0; i < numScanners; i++ {
		go func(scannerID int) {
			counter := 0
			for {
				els.Get(counter)
				counter = (counter + 1) % numElements
			}
		}(i)
	}

	// Remove an element, push back an element.
	for i := 0; i < numTimes; i++ {
		// Pick an element to remove
		rmElIdx := rand.Intn(els.Size())
		els.Delete(rmElIdx)

		// Insert a new element
		els.Set(rmElIdx, struct{}{})

		if i%100000 == 0 {
			fmt.Printf("Pushed %vK elements so far...\n", i/1000)
		}
	}
}
