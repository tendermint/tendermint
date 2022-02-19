package consensus

import (
	"context"
	"io"
	"time"
)

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

type LiveIterator struct {
	peerMsgQueue     chan msgInfo
	internalMsgQueue chan msgInfo
	timeoutTicker    TimeoutTicker
	txNotifier       txNotifier

	err error
	e   Event
}

func (l *LiveIterator) Next(ctx context.Context) bool {
	select {
	case m := <-l.peerMsgQueue:
		l.e = Event{m, time.Now()}
	case m := <-l.internalMsgQueue:
		l.e = Event{m, time.Now()}
	case t := <-l.timeoutTicker.Chan():
		l.e = Event{Message: t, Time: time.Now()}
	case <-l.txNotifier.TxsAvailable():
		l.e = Event{txAvailable{}, time.Now()}
	case <-ctx.Done():
		l.err = ctx.Err()
		return false
	}
	return true
}

func (l *LiveIterator) Event() Event {
	return l.e
}

func (l *LiveIterator) Error() error {
	return l.err
}
