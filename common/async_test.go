package common

import (
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestParallel(t *testing.T) {

	// Create tasks.
	var counter = new(int32)
	var tasks = make([]Task, 100*1000)
	for i := 0; i < len(tasks); i++ {
		tasks[i] = func(i int) (res interface{}, err error, abort bool) {
			atomic.AddInt32(counter, 1)
			return -1 * i, nil, false
		}
	}

	// Run in parallel.
	var taskResultChz = Parallel(tasks...)

	// Verify.
	assert.Equal(t, int(*counter), len(tasks), "Each task should have incremented the counter already")
	var failedTasks int
	for i := 0; i < len(tasks); i++ {
		select {
		case taskResult := <-taskResultChz[i]:
			if taskResult.Error != nil {
				assert.Fail(t, "Task should not have errored but got %v", taskResult.Error)
				failedTasks += 1
			} else if !assert.Equal(t, -1*i, taskResult.Value.(int)) {
				failedTasks += 1
			} else {
				// Good!
			}
		default:
			failedTasks += 1
		}
	}
	assert.Equal(t, failedTasks, 0, "No task should have failed")

}

func TestParallelAbort(t *testing.T) {

	var flow1 = make(chan struct{}, 1)
	var flow2 = make(chan struct{}, 1)
	var flow3 = make(chan struct{}, 1) // Cap must be > 0 to prevent blocking.
	var flow4 = make(chan struct{}, 1)

	// Create tasks.
	var tasks = []Task{
		func(i int) (res interface{}, err error, abort bool) {
			assert.Equal(t, i, 0)
			flow1 <- struct{}{}
			return 0, nil, false
		},
		func(i int) (res interface{}, err error, abort bool) {
			assert.Equal(t, i, 1)
			flow2 <- <-flow1
			return 1, errors.New("some error"), false
		},
		func(i int) (res interface{}, err error, abort bool) {
			assert.Equal(t, i, 2)
			flow3 <- <-flow2
			return 2, nil, true
		},
		func(i int) (res interface{}, err error, abort bool) {
			assert.Equal(t, i, 3)
			<-flow4
			return 3, nil, false
		},
	}

	// Run in parallel.
	var taskResultChz = Parallel(tasks...)

	// Verify task #3.
	// Initially taskResultCh[3] sends nothing since flow4 didn't send.
	waitTimeout(t, taskResultChz[3], "Task #3")

	// Now let the last task (#3) complete after abort.
	flow4 <- <-flow3

	// Verify task #0, #1, #2.
	waitFor(t, taskResultChz[0], "Task #0", 0, nil, nil)
	waitFor(t, taskResultChz[1], "Task #1", 1, errors.New("some error"), nil)
	waitFor(t, taskResultChz[2], "Task #2", 2, nil, nil)
}

func TestParallelRecover(t *testing.T) {

	// Create tasks.
	var tasks = []Task{
		func(i int) (res interface{}, err error, abort bool) {
			return 0, nil, false
		},
		func(i int) (res interface{}, err error, abort bool) {
			return 1, errors.New("some error"), false
		},
		func(i int) (res interface{}, err error, abort bool) {
			panic(2)
		},
	}

	// Run in parallel.
	var taskResultChz = Parallel(tasks...)

	// Verify task #0, #1, #2.
	waitFor(t, taskResultChz[0], "Task #0", 0, nil, nil)
	waitFor(t, taskResultChz[1], "Task #1", 1, errors.New("some error"), nil)
	waitFor(t, taskResultChz[2], "Task #2", nil, nil, 2)
}

// Wait for result
func waitFor(t *testing.T, taskResultCh TaskResultCh, taskName string, val interface{}, err error, pnk interface{}) {
	select {
	case taskResult, ok := <-taskResultCh:
		assert.True(t, ok, "TaskResultCh unexpectedly closed for %v", taskName)
		assert.Equal(t, val, taskResult.Value, taskName)
		assert.Equal(t, err, taskResult.Error, taskName)
		assert.Equal(t, pnk, taskResult.Panic, taskName)
	default:
		assert.Fail(t, "Failed to receive result for %v", taskName)
	}
}

// Wait for timeout (no result)
func waitTimeout(t *testing.T, taskResultCh TaskResultCh, taskName string) {
	select {
	case _, ok := <-taskResultCh:
		if !ok {
			assert.Fail(t, "TaskResultCh unexpectedly closed (%v)", taskName)
		} else {
			assert.Fail(t, "TaskResultCh unexpectedly returned for %v", taskName)
		}
	case <-time.After(1 * time.Second): // TODO use deterministic time?
		// Good!
	}
}
