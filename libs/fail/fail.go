package fail

import (
	"fmt"
	"math/rand"
	"os"
	"strconv"
)

var callIndexToFail int

func init() {
	callIndexToFailS := os.Getenv("FAIL_TEST_INDEX")

	if callIndexToFailS == "" {
		callIndexToFail = -1
	} else {
		var err error
		callIndexToFail, err = strconv.Atoi(callIndexToFailS)
		if err != nil {
			callIndexToFail = -1
		}
	}
}

// Fail when FAIL_TEST_INDEX == callIndex
var (
	callIndex int //indexes Fail calls

	callRandIndex       int  // indexes a run of FailRand calls
	callRandIndexToFail = -1 // the callRandIndex to fail on in FailRand
)

func Fail() {
	if callIndexToFail < 0 {
		return
	}

	if callIndex == callIndexToFail {
		Exit()
	}

	callIndex += 1
}

// FailRand should be called n successive times.
// It will fail on a random one of those calls
// n must be greater than 0
func FailRand(n int) {
	if callIndexToFail < 0 {
		return
	}

	if callRandIndexToFail < 0 {
		// first call in the loop, pick a random index to fail at
		callRandIndexToFail = rand.Intn(n)
		callRandIndex = 0
	}

	if callIndex == callIndexToFail {
		if callRandIndex == callRandIndexToFail {
			Exit()
		}
	}

	callRandIndex += 1

	if callRandIndex == n {
		callIndex += 1
	}
}

func Exit() {
	fmt.Printf("*** fail-test %d ***\n", callIndex)
	os.Exit(1)
	//	proc, _ := os.FindProcess(os.Getpid())
	//	proc.Signal(os.Interrupt)
	//	panic(fmt.Sprintf("*** fail-test %d ***", callIndex))
}
