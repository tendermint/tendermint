package events

import (
	"context"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/fortytw2/leaktest"
	"github.com/stretchr/testify/require"
)

// TestAddListenerForEventFireOnce sets up an EventSwitch, subscribes a single
// listener to an event, and sends a string "data".
func TestAddListenerForEventFireOnce(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	evsw := NewEventSwitch()

	messages := make(chan EventData)
	require.NoError(t, evsw.AddListenerForEvent("listener", "event",
		func(data EventData) error {
			select {
			case messages <- data:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		}))
	go evsw.FireEvent("event", "data")
	received := <-messages
	if received != "data" {
		t.Errorf("message received does not match: %v", received)
	}
}

// TestAddListenerForEventFireMany sets up an EventSwitch, subscribes a single
// listener to an event, and sends a thousand integers.
func TestAddListenerForEventFireMany(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	evsw := NewEventSwitch()

	doneSum := make(chan uint64)
	doneSending := make(chan uint64)
	numbers := make(chan uint64, 4)
	// subscribe one listener for one event
	require.NoError(t, evsw.AddListenerForEvent("listener", "event",
		func(data EventData) error {
			select {
			case numbers <- data.(uint64):
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		}))
	// collect received events
	go sumReceivedNumbers(numbers, doneSum)
	// go fire events
	go fireEvents(ctx, evsw, "event", doneSending, uint64(1))
	checkSum := <-doneSending
	close(numbers)
	eventSum := <-doneSum
	if checkSum != eventSum {
		t.Errorf("not all messages sent were received.\n")
	}
}

// TestAddListenerForDifferentEvents sets up an EventSwitch, subscribes a single
// listener to three different events and sends a thousand integers for each
// of the three events.
func TestAddListenerForDifferentEvents(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	t.Cleanup(leaktest.Check(t))

	evsw := NewEventSwitch()

	doneSum := make(chan uint64)
	doneSending1 := make(chan uint64)
	doneSending2 := make(chan uint64)
	doneSending3 := make(chan uint64)
	numbers := make(chan uint64, 4)
	// subscribe one listener to three events
	require.NoError(t, evsw.AddListenerForEvent("listener", "event1",
		func(data EventData) error {
			select {
			case numbers <- data.(uint64):
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		}))
	require.NoError(t, evsw.AddListenerForEvent("listener", "event2",
		func(data EventData) error {
			select {
			case numbers <- data.(uint64):
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		}))
	require.NoError(t, evsw.AddListenerForEvent("listener", "event3",
		func(data EventData) error {
			select {
			case numbers <- data.(uint64):
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		}))
	// collect received events
	go sumReceivedNumbers(numbers, doneSum)
	// go fire events
	go fireEvents(ctx, evsw, "event1", doneSending1, uint64(1))
	go fireEvents(ctx, evsw, "event2", doneSending2, uint64(1))
	go fireEvents(ctx, evsw, "event3", doneSending3, uint64(1))
	var checkSum uint64
	checkSum += <-doneSending1
	checkSum += <-doneSending2
	checkSum += <-doneSending3
	close(numbers)
	eventSum := <-doneSum
	if checkSum != eventSum {
		t.Errorf("not all messages sent were received.\n")
	}
}

// TestAddDifferentListenerForDifferentEvents sets up an EventSwitch,
// subscribes a first listener to three events, and subscribes a second
// listener to two of those three events, and then sends a thousand integers
// for each of the three events.
func TestAddDifferentListenerForDifferentEvents(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	t.Cleanup(leaktest.Check(t))

	evsw := NewEventSwitch()

	doneSum1 := make(chan uint64)
	doneSum2 := make(chan uint64)
	doneSending1 := make(chan uint64)
	doneSending2 := make(chan uint64)
	doneSending3 := make(chan uint64)
	numbers1 := make(chan uint64, 4)
	numbers2 := make(chan uint64, 4)
	// subscribe two listener to three events
	require.NoError(t, evsw.AddListenerForEvent("listener1", "event1",
		func(data EventData) error {
			select {
			case numbers1 <- data.(uint64):
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		}))
	require.NoError(t, evsw.AddListenerForEvent("listener1", "event2",
		func(data EventData) error {
			select {
			case numbers1 <- data.(uint64):
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		}))
	require.NoError(t, evsw.AddListenerForEvent("listener1", "event3",
		func(data EventData) error {
			select {
			case numbers1 <- data.(uint64):
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		}))
	require.NoError(t, evsw.AddListenerForEvent("listener2", "event2",
		func(data EventData) error {
			select {
			case numbers2 <- data.(uint64):
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		}))
	require.NoError(t, evsw.AddListenerForEvent("listener2", "event3",
		func(data EventData) error {
			select {
			case numbers2 <- data.(uint64):
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		}))
	// collect received events for listener1
	go sumReceivedNumbers(numbers1, doneSum1)
	// collect received events for listener2
	go sumReceivedNumbers(numbers2, doneSum2)
	// go fire events
	go fireEvents(ctx, evsw, "event1", doneSending1, uint64(1))
	go fireEvents(ctx, evsw, "event2", doneSending2, uint64(1001))
	go fireEvents(ctx, evsw, "event3", doneSending3, uint64(2001))
	checkSumEvent1 := <-doneSending1
	checkSumEvent2 := <-doneSending2
	checkSumEvent3 := <-doneSending3
	checkSum1 := checkSumEvent1 + checkSumEvent2 + checkSumEvent3
	checkSum2 := checkSumEvent2 + checkSumEvent3
	close(numbers1)
	close(numbers2)
	eventSum1 := <-doneSum1
	eventSum2 := <-doneSum2
	if checkSum1 != eventSum1 ||
		checkSum2 != eventSum2 {
		t.Errorf("not all messages sent were received for different listeners to different events.\n")
	}
}

// TestManagerLiistenersAsync sets up an EventSwitch, subscribes two
// listeners to three events, and fires a thousand integers for each event.
// These two listeners serve as the baseline validation while other listeners
// are randomly subscribed and unsubscribed.
// More precisely it randomly subscribes new listeners (different from the first
// two listeners) to one of these three events. At the same time it starts
// randomly unsubscribing these additional listeners from all events they are
// at that point subscribed to.
// NOTE: it is important to run this test with race conditions tracking on,
// `go test -race`, to examine for possible race conditions.
func TestManageListenersAsync(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	evsw := NewEventSwitch()

	doneSum1 := make(chan uint64)
	doneSum2 := make(chan uint64)
	doneSending1 := make(chan uint64)
	doneSending2 := make(chan uint64)
	doneSending3 := make(chan uint64)
	numbers1 := make(chan uint64, 4)
	numbers2 := make(chan uint64, 4)
	// subscribe two listener to three events
	require.NoError(t, evsw.AddListenerForEvent("listener1", "event1",
		func(data EventData) error {
			select {
			case numbers1 <- data.(uint64):
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		}))
	require.NoError(t, evsw.AddListenerForEvent("listener1", "event2",
		func(data EventData) error {
			select {
			case numbers1 <- data.(uint64):
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		}))
	require.NoError(t, evsw.AddListenerForEvent("listener1", "event3",
		func(data EventData) error {
			select {
			case numbers1 <- data.(uint64):
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		}))
	require.NoError(t, evsw.AddListenerForEvent("listener2", "event1",
		func(data EventData) error {
			select {
			case numbers2 <- data.(uint64):
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		}))
	require.NoError(t, evsw.AddListenerForEvent("listener2", "event2",
		func(data EventData) error {
			select {
			case numbers2 <- data.(uint64):
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		}))
	require.NoError(t, evsw.AddListenerForEvent("listener2", "event3",
		func(data EventData) error {
			select {
			case numbers2 <- data.(uint64):
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		}))
	// collect received events for event1
	go sumReceivedNumbers(numbers1, doneSum1)
	// collect received events for event2
	go sumReceivedNumbers(numbers2, doneSum2)
	addListenersStress := func() {
		r1 := rand.New(rand.NewSource(time.Now().Unix()))
		r1.Seed(time.Now().UnixNano())
		for k := uint16(0); k < 400; k++ {
			listenerNumber := r1.Intn(100) + 3
			eventNumber := r1.Intn(3) + 1
			go evsw.AddListenerForEvent(fmt.Sprintf("listener%v", listenerNumber), //nolint:errcheck // ignore for tests
				fmt.Sprintf("event%v", eventNumber),
				func(EventData) error { return nil })
		}
	}
	addListenersStress()
	// go fire events
	go fireEvents(ctx, evsw, "event1", doneSending1, uint64(1))
	go fireEvents(ctx, evsw, "event2", doneSending2, uint64(1001))
	go fireEvents(ctx, evsw, "event3", doneSending3, uint64(2001))
	checkSumEvent1 := <-doneSending1
	checkSumEvent2 := <-doneSending2
	checkSumEvent3 := <-doneSending3
	checkSum := checkSumEvent1 + checkSumEvent2 + checkSumEvent3
	close(numbers1)
	close(numbers2)
	eventSum1 := <-doneSum1
	eventSum2 := <-doneSum2
	if checkSum != eventSum1 ||
		checkSum != eventSum2 {
		t.Errorf("not all messages sent were received.\n")
	}
}

//------------------------------------------------------------------------------
// Helper functions

// sumReceivedNumbers takes two channels and adds all numbers received
// until the receiving channel `numbers` is closed; it then sends the sum
// on `doneSum` and closes that channel.  Expected to be run in a go-routine.
func sumReceivedNumbers(numbers, doneSum chan uint64) {
	var sum uint64
	for {
		j, more := <-numbers
		sum += j
		if !more {
			doneSum <- sum
			close(doneSum)
			return
		}
	}
}

// fireEvents takes an EventSwitch and fires a thousand integers under
// a given `event` with the integers mootonically increasing from `offset`
// to `offset` + 999.  It additionally returns the addition of all integers
// sent on `doneChan` for assertion that all events have been sent, and enabling
// the test to assert all events have also been received.
func fireEvents(ctx context.Context, evsw Fireable, event string, doneChan chan uint64, offset uint64) {
	defer close(doneChan)

	var sentSum uint64
	for i := offset; i <= offset+uint64(999); i++ {
		if ctx.Err() != nil {
			break
		}

		evsw.FireEvent(event, i)
		sentSum += i
	}

	select {
	case <-ctx.Done():
	case doneChan <- sentSum:
	}
}
