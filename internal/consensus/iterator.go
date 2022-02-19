package consensus

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"
)

const (
	Timeout EventType = iota
	ReceiveMessage
)

var (
	ErrEndOfIterator = errors.New("end of iterator")
)

type EventType int64

type txAvailable struct{}

type Event struct {
	Message interface{}
	Time    time.Time
}

type Iterator interface {
	Next(context.Context) bool
	Event() Event
	Error() error
}

type WALReadingIterator struct {
	dec WALDecoder
	e   Event
	err error
}

func (w WALReadingIterator) Next(ctx context.Context) bool {
	fmt.Println("hello")
	wm, err := w.dec.Decode()
	if err != nil {
		if err != io.EOF {
			w.err = err
		}
		return false
	}
	w.e = Event{
		Time:    wm.Time,
		Message: wm.Msg,
	}
	return true
}

func (w WALReadingIterator) Event() Event {
	return w.e
}

func (w WALReadingIterator) Error() error {
	return w.err
}

type testIterator struct {
	i      int
	events []Event
	e      error
}

func (t testIterator) Next(ctx context.Context) bool {
	if t.i >= len(t.events) {
		return false
	}
	return true
}

func (t testIterator) Event() Event {
	return t.events[t.i]
}
func (t testIterator) Error() error {
	return t.e
}
