package v2

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type eventA struct {
	priorityNormal
}

var done = fmt.Errorf("done")

func simpleHandler(event Event) (Event, error) {
	if _, ok := event.(eventA); ok {
		return noOp, done
	}
	return noOp, nil
}

func TestRoutineFinal(t *testing.T) {
	var (
		bufferSize = 10
		routine    = newRoutine("simpleRoutine", simpleHandler, bufferSize)
	)

	assert.False(t, routine.isRunning(),
		"expected an initialized routine to not be running")
	go routine.start()
	<-routine.ready()
	assert.True(t, routine.isRunning(),
		"expected an started routine")

	assert.True(t, routine.send(eventA{}),
		"expected sending to a ready routine to succeed")

	assert.Equal(t, done, <-routine.final(),
		"expected the final event to be done")

	assert.False(t, routine.isRunning(),
		"expected an completed routine to no longer be running")
}

func TestRoutineStop(t *testing.T) {
	var (
		bufferSize = 10
		routine    = newRoutine("simpleRoutine", simpleHandler, bufferSize)
	)

	assert.False(t, routine.send(eventA{}),
		"expected sending to an unstarted routine to fail")

	go routine.start()
	<-routine.ready()

	assert.True(t, routine.send(eventA{}),
		"expected sending to a running routine to succeed")

	routine.stop()

	assert.False(t, routine.send(eventA{}),
		"expected sending to a stopped routine to fail")
}

type finalCount struct {
	count int
}

func (f finalCount) Error() string {
	return "end"
}

func genStatefulHandler(maxCount int) handleFunc {
	counter := 0
	return func(event Event) (Event, error) {
		if _, ok := event.(eventA); ok {
			counter += 1
			if counter >= maxCount {
				return noOp, finalCount{counter}
			}

			return eventA{}, nil
		}
		return noOp, nil
	}
}

func feedback(r *Routine) {
	for event := range r.next() {
		r.send(event)
	}
}

func TestStatefulRoutine(t *testing.T) {
	var (
		count      = 10
		handler    = genStatefulHandler(count)
		bufferSize = 20
		routine    = newRoutine("statefulRoutine", handler, bufferSize)
	)

	go routine.start()
	go feedback(routine)
	<-routine.ready()

	assert.True(t, routine.send(eventA{}),
		"expected sending to a started routine to succeed")

	final := <-routine.final()
	if fnl, ok := final.(finalCount); ok {
		assert.Equal(t, count, fnl.count,
			"expected the routine to count to 10")
	} else {
		t.Fail()
	}
}

type lowPriorityEvent struct {
	priorityLow
}

type highPriorityEvent struct {
	priorityHigh
}

func handleWithPriority(event Event) (Event, error) {
	switch event.(type) {
	case lowPriorityEvent:
		return noOp, nil
	case highPriorityEvent:
		return noOp, done
	}
	return noOp, nil
}

func TestPriority(t *testing.T) {
	var (
		bufferSize = 20
		routine    = newRoutine("priorityRoutine", handleWithPriority, bufferSize)
	)

	go routine.start()
	<-routine.ready()
	go func() {
		for {
			routine.send(lowPriorityEvent{})
			time.Sleep(1 * time.Millisecond)
		}
	}()
	time.Sleep(10 * time.Millisecond)

	assert.True(t, routine.isRunning(),
		"expected an started routine")
	assert.True(t, routine.send(highPriorityEvent{}),
		"expected send to succeed even when saturated")

	assert.Equal(t, done, <-routine.final())
	assert.False(t, routine.isRunning(),
		"expected an started routine")
}
