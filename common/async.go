package common

// val: the value returned after task execution.
// err: the error returned during task completion.
// abort: tells Parallel to return, whether or not all tasks have completed.
type Task func(i int) (val interface{}, err error, abort bool)

type TaskResult struct {
	Value interface{}
	Error error
	Panic interface{}
}

type TaskResultCh <-chan TaskResult

// Run tasks in parallel, with ability to abort early.
// NOTE: Do not implement quit features here.  Instead, provide convenient
// concurrent quit-like primitives, passed implicitly via Task closures. (e.g.
// it's not Parallel's concern how you quit/abort your tasks).
func Parallel(tasks ...Task) []TaskResultCh {
	var taskResultChz = make([]TaskResultCh, len(tasks)) // To return.
	var taskDoneCh = make(chan bool, len(tasks))         // A "wait group" channel, early abort if any true received.

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
					taskResultCh <- TaskResult{nil, nil, pnk}
					taskDoneCh <- false
				}
			}()
			// Run the task.
			var val, err, abort = task(i)
			// Send val/err to taskResultCh.
			// NOTE: Below this line, nothing must panic/
			taskResultCh <- TaskResult{val, err, nil}
			// Decrement waitgroup.
			taskDoneCh <- abort
		}(i, task, taskResultCh)
	}

	// Wait until all tasks are done, or until abort.
	for i := 0; i < len(tasks); i++ {
		abort := <-taskDoneCh
		if abort {
			break
		}
	}

	// Caller can use this however they want.
	// TODO: implement convenience functions to
	// make sense of this structure safely.
	return taskResultChz
}
