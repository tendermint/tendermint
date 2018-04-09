package common

import (
	"sync/atomic"
)

//----------------------------------------
// Task

// val: the value returned after task execution.
// err: the error returned during task completion.
// abort: tells Parallel to return, whether or not all tasks have completed.
type Task func(i int) (val interface{}, err error, abort bool)

type TaskResult struct {
	Value interface{}
	Error error
}

type TaskResultCh <-chan TaskResult

type taskResultOK struct {
	TaskResult
	OK bool
}

type TaskResultSet struct {
	chz     []TaskResultCh
	results []taskResultOK
}

func newTaskResultSet(chz []TaskResultCh) *TaskResultSet {
	return &TaskResultSet{
		chz:     chz,
		results: nil,
	}
}

func (trs *TaskResultSet) Channels() []TaskResultCh {
	return trs.chz
}

func (trs *TaskResultSet) LatestResult(index int) (TaskResult, bool) {
	if len(trs.results) <= index {
		return TaskResult{}, false
	}
	resultOK := trs.results[index]
	return resultOK.TaskResult, resultOK.OK
}

// NOTE: Not concurrency safe.
func (trs *TaskResultSet) Reap() *TaskResultSet {
	if trs.results == nil {
		trs.results = make([]taskResultOK, len(trs.chz))
	}
	for i := 0; i < len(trs.results); i++ {
		var trch = trs.chz[i]
		select {
		case result := <-trch:
			// Overwrite result.
			trs.results[i] = taskResultOK{
				TaskResult: result,
				OK:         true,
			}
		default:
			// Do nothing.
		}
	}
	return trs
}

// Returns the firstmost (by task index) error as
// discovered by all previous Reap() calls.
func (trs *TaskResultSet) FirstValue() interface{} {
	for _, result := range trs.results {
		if result.Value != nil {
			return result.Value
		}
	}
	return nil
}

// Returns the firstmost (by task index) error as
// discovered by all previous Reap() calls.
func (trs *TaskResultSet) FirstError() error {
	for _, result := range trs.results {
		if result.Error != nil {
			return result.Error
		}
	}
	return nil
}

//----------------------------------------
// Parallel

// Run tasks in parallel, with ability to abort early.
// Returns ok=false iff any of the tasks returned abort=true.
// NOTE: Do not implement quit features here.  Instead, provide convenient
// concurrent quit-like primitives, passed implicitly via Task closures. (e.g.
// it's not Parallel's concern how you quit/abort your tasks).
func Parallel(tasks ...Task) (trs *TaskResultSet, ok bool) {
	var taskResultChz = make([]TaskResultCh, len(tasks)) // To return.
	var taskDoneCh = make(chan bool, len(tasks))         // A "wait group" channel, early abort if any true received.
	var numPanics = new(int32)                           // Keep track of panics to set ok=false later.
	ok = true                                            // We will set it to false iff any tasks panic'd or returned abort.

	// Start all tasks in parallel in separate goroutines.
	// When the task is complete, it will appear in the
	// respective taskResultCh (associated by task index).
	for i, task := range tasks {
		var taskResultCh = make(chan TaskResult, 1) // Capacity for 1 result.
		taskResultChz[i] = taskResultCh
		go func(i int, task Task, taskResultCh chan TaskResult) {
			// Recovery
			defer func() {
				if pnk := recover(); pnk != nil {
					atomic.AddInt32(numPanics, 1)
					taskResultCh <- TaskResult{nil, ErrorWrap(pnk, "Panic in task")}
					taskDoneCh <- false
				}
			}()
			// Run the task.
			var val, err, abort = task(i)
			// Send val/err to taskResultCh.
			// NOTE: Below this line, nothing must panic/
			taskResultCh <- TaskResult{val, err}
			// Decrement waitgroup.
			taskDoneCh <- abort
		}(i, task, taskResultCh)
	}

	// Wait until all tasks are done, or until abort.
	// DONE_LOOP:
	for i := 0; i < len(tasks); i++ {
		abort := <-taskDoneCh
		if abort {
			ok = false
			break
		}
	}

	// Ok is also false if there were any panics.
	// We must do this check here (after DONE_LOOP).
	ok = ok && (atomic.LoadInt32(numPanics) == 0)

	return newTaskResultSet(taskResultChz).Reap(), ok
}
