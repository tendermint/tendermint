package v2

import (
	"fmt"
	"testing"
	"time"
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
	events := make(chan Event, 10)
	routine := newRoutine("simpleRoutine", events, simpleHandler)

	go routine.run()
	go routine.feedback()

	routine.send(eventA{})

	routine.wait()
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
	events := make(chan Event, 10)
	handler := genStatefulHandler(10)
	routine := newRoutine("statefulRoutine", events, handler)

	go routine.run()
	go routine.feedback()

	go routine.send(eventA{})

	routine.wait()
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
	events := make(chan Event, 10)
	routine := newRoutine("errorRoutine", events, handleWithErrors)

	go routine.run()
	go func() {
		for {
			routine.send(eventA{})
			time.Sleep(10 * time.Millisecond)
		}
	}()
	routine.send(errEvent{})

	routine.wait()
}
