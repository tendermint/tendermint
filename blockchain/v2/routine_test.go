package v2

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/tendermint/tendermint/libs/log"
)

type eventA struct{}
type eventB struct{}
type errEvent struct{}

var done = fmt.Errorf("done")

func simpleHandler(event Event) (Events, error) {
	switch event.(type) {
	case eventA:
		return Events{eventB{}}, nil
	case eventB:
		return Events{}, done
	}
	return Events{}, nil
}

func TestRoutine(t *testing.T) {
	routine := newRoutine("simpleRoutine", simpleHandler)

	assert.False(t, routine.isRunning(),
		"expected an initialized routine to not be running")
	go routine.start()
	go routine.feedback()
	<-routine.ready()

	assert.True(t, routine.trySend(eventA{}),
		"expected sending to a ready routine to succeed")

	assert.Equal(t, done, <-routine.final(),
		"expected the final event to be done")
}

func TestRoutineSend(t *testing.T) {
	routine := newRoutine("simpleRoutine", simpleHandler)

	assert.False(t, routine.trySend(eventA{}),
		"expected sending to an unstarted routine to fail")

	go routine.start()

	go routine.feedback()
	<-routine.ready()

	assert.True(t, routine.trySend(eventA{}),
		"expected sending to a running routine to succeed")

	routine.stop()

	assert.False(t, routine.trySend(eventA{}),
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
	return func(event Event) (Events, error) {
		// golint fixme
		switch event.(type) {
		case eventA:
			counter += 1
			if counter >= maxCount {
				return Events{}, finalCount{counter}
			}

			return Events{eventA{}}, nil
		case eventB:
			return Events{}, nil
		}
		return Events{}, nil
	}
}

func TestStatefulRoutine(t *testing.T) {
	count := 10
	handler := genStatefulHandler(count)
	routine := newRoutine("statefulRoutine", handler)
	routine.setLogger(log.TestingLogger())

	go routine.start()
	go routine.feedback()

	<-routine.ready()

	assert.True(t, routine.trySend(eventA{}),
		"expected sending to a started routine to succeed")

	final := <-routine.final()
	fnl, ok := final.(finalCount)
	if ok {
		assert.Equal(t, count, fnl.count,
			"expected the routine to count to 10")
	} else {
		t.Fail()
	}
}

func handleWithErrors(event Event) (Events, error) {
	switch event.(type) {
	case eventA:
		return Events{}, nil
	case errEvent:
		return Events{}, done
	}
	return Events{}, nil
}

func TestErrorSaturation(t *testing.T) {
	routine := newRoutine("errorRoutine", handleWithErrors)
	go routine.start()
	<-routine.ready()
	go func() {
		for {
			routine.trySend(eventA{})
			time.Sleep(10 * time.Millisecond)
		}
	}()

	assert.True(t, routine.trySend(errEvent{}),
		"expected send to succeed even when saturated")

	assert.Equal(t, done, <-routine.final())
}
