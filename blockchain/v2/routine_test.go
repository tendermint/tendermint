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

var done = fmt.Errorf("done")

func simpleHandler(event Event) (Events, error) {
	switch event.(type) {
	case eventA:
		return Events{eventB{}}, nil
	case eventB:
		return Events{routineFinished{}}, done
	}
	return Events{}, nil
}

func TestRoutine(t *testing.T) {
	routine := newRoutine("simpleRoutine", simpleHandler)

	assert.False(t, routine.isRunning(),
		"expected an initialized routine to not be running")
	go routine.run()
	go routine.feedback()
	for {
		if routine.isRunning() {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	routine.send(eventA{})

	routine.stop()
}

func TesRoutineSend(t *testing.T) {
	routine := newRoutine("simpleRoutine", simpleHandler)

	assert.False(t, routine.send(eventA{}),
		"expected sending to an unstarted routine to fail")

	go routine.run()

	go routine.feedback()
	for {
		if routine.isRunning() {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	assert.True(t, routine.send(eventA{}),
		"expected sending to a running routine to succeed")

	routine.stop()

	assert.False(t, routine.send(eventA{}),
		"expected sending to a stopped routine to fail")
}

func genStatefulHandler(maxCount int) handleFunc {
	counter := 0
	return func(event Event) (Events, error) {
		switch event.(type) {
		case eventA:
			counter += 1
			if counter >= maxCount {
				return Events{}, done
			}

			return Events{eventA{}}, nil
		}
		return Events{}, nil
	}
}

func TestStatefulRoutine(t *testing.T) {
	handler := genStatefulHandler(10)
	routine := newRoutine("statefulRoutine", handler)
	routine.setLogger(log.TestingLogger())

	go routine.run()
	go routine.feedback()

	for {
		if routine.isRunning() {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	routine.send(eventA{})

	routine.stop()
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
	go routine.run()
	go func() {
		for {
			routine.send(eventA{})
			time.Sleep(10 * time.Millisecond)
		}
	}()

	for {
		if routine.isRunning() {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	assert.True(t, routine.send(errEvent{}),
		"expected send to succeed even when saturated")

	routine.wait()
}
