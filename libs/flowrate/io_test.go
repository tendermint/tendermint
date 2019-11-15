//
// Written by Maxim Khitrov (November 2012)
//

package flowrate

import (
	"bytes"
	"testing"
	"time"
)

const (
	_50ms  = 50 * time.Millisecond
	_100ms = 100 * time.Millisecond
	_200ms = 200 * time.Millisecond
	_300ms = 300 * time.Millisecond
	_400ms = 400 * time.Millisecond
	_500ms = 500 * time.Millisecond
)

func nextStatus(m *Monitor) Status {
	samples := m.samples
	for i := 0; i < 30; i++ {
		if s := m.Status(); s.Samples != samples {
			return s
		}
		time.Sleep(5 * time.Millisecond)
	}
	return m.Status()
}

func TestReader(t *testing.T) {
	in := make([]byte, 100)
	for i := range in {
		in[i] = byte(i)
	}
	b := make([]byte, 100)
	r := NewReader(bytes.NewReader(in), 100)
	start := time.Now()

	// Make sure r implements Limiter
	_ = Limiter(r)

	// 1st read of 10 bytes is performed immediately
	if n, err := r.Read(b); n != 10 || err != nil {
		t.Fatalf("r.Read(b) expected 10 (<nil>); got %v (%v)", n, err)
	} else if rt := time.Since(start); rt > _50ms {
		t.Fatalf("r.Read(b) took too long (%v)", rt)
	}

	// No new Reads allowed in the current sample
	r.SetBlocking(false)
	if n, err := r.Read(b); n != 0 || err != nil {
		t.Fatalf("r.Read(b) expected 0 (<nil>); got %v (%v)", n, err)
	} else if rt := time.Since(start); rt > _50ms {
		t.Fatalf("r.Read(b) took too long (%v)", rt)
	}

	status := [6]Status{0: r.Status()} // No samples in the first status

	// 2nd read of 10 bytes blocks until the next sample
	r.SetBlocking(true)
	if n, err := r.Read(b[10:]); n != 10 || err != nil {
		t.Fatalf("r.Read(b[10:]) expected 10 (<nil>); got %v (%v)", n, err)
	} else if rt := time.Since(start); rt < _100ms {
		t.Fatalf("r.Read(b[10:]) returned ahead of time (%v)", rt)
	}

	status[1] = r.Status()            // 1st sample
	status[2] = nextStatus(r.Monitor) // 2nd sample
	status[3] = nextStatus(r.Monitor) // No activity for the 3rd sample

	if n := r.Done(); n != 20 {
		t.Fatalf("r.Done() expected 20; got %v", n)
	}

	status[4] = r.Status()
	status[5] = nextStatus(r.Monitor) // Timeout
	start = status[0].Start

	// Active, Bytes, Samples, InstRate, CurRate, AvgRate, PeakRate, BytesRem, Start, Duration, Idle, TimeRem, Progress
	want := []Status{
		{start, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, true},
		{start, 10, 1, 100, 100, 100, 100, 0, _100ms, 0, 0, 0, true},
		{start, 20, 2, 100, 100, 100, 100, 0, _200ms, _100ms, 0, 0, true},
		{start, 20, 3, 0, 90, 67, 100, 0, _300ms, _200ms, 0, 0, true},
		{start, 20, 3, 0, 0, 67, 100, 0, _300ms, 0, 0, 0, false},
		{start, 20, 3, 0, 0, 67, 100, 0, _300ms, 0, 0, 0, false},
	}
	for i, s := range status {
		s := s
		if !statusesAreEqual(&s, &want[i]) {
			t.Errorf("r.Status(%v)\nexpected: %v\ngot     : %v", i, want[i], s)
		}
	}
	if !bytes.Equal(b[:20], in[:20]) {
		t.Errorf("r.Read() input doesn't match output")
	}
}

func TestWriter(t *testing.T) {
	b := make([]byte, 100)
	for i := range b {
		b[i] = byte(i)
	}
	w := NewWriter(&bytes.Buffer{}, 200)
	start := time.Now()

	// Make sure w implements Limiter
	_ = Limiter(w)

	// Non-blocking 20-byte write for the first sample returns ErrLimit
	w.SetBlocking(false)
	if n, err := w.Write(b); n != 20 || err != ErrLimit {
		t.Fatalf("w.Write(b) expected 20 (ErrLimit); got %v (%v)", n, err)
	} else if rt := time.Since(start); rt > _50ms {
		t.Fatalf("w.Write(b) took too long (%v)", rt)
	}

	// Blocking 80-byte write
	w.SetBlocking(true)
	if n, err := w.Write(b[20:]); n != 80 || err != nil {
		t.Fatalf("w.Write(b[20:]) expected 80 (<nil>); got %v (%v)", n, err)
	} else if rt := time.Since(start); rt < _300ms {
		// Explanation for `rt < _300ms` (as opposed to `< _400ms`)
		//
		//                 |<-- start        |        |
		// epochs: -----0ms|---100ms|---200ms|---300ms|---400ms
		// sends:        20|20      |20      |20      |20#
		//
		// NOTE: The '#' symbol can thus happen before 400ms is up.
		// Thus, we can only panic if rt < _300ms.
		t.Fatalf("w.Write(b[20:]) returned ahead of time (%v)", rt)
	}

	w.SetTransferSize(100)
	status := []Status{w.Status(), nextStatus(w.Monitor)}
	start = status[0].Start

	// Active, Bytes, Samples, InstRate, CurRate, AvgRate, PeakRate, BytesRem, Start, Duration, Idle, TimeRem, Progress
	want := []Status{
		{start, 80, 4, 200, 200, 200, 200, 20, _400ms, 0, _100ms, 80000, true},
		{start, 100, 5, 200, 200, 200, 200, 0, _500ms, _100ms, 0, 100000, true},
	}

	for i, s := range status {
		s := s
		if !statusesAreEqual(&s, &want[i]) {
			t.Errorf("w.Status(%v)\nexpected: %v\ngot     : %v\n", i, want[i], s)
		}
	}
	if !bytes.Equal(b, w.Writer.(*bytes.Buffer).Bytes()) {
		t.Errorf("w.Write() input doesn't match output")
	}
}

const maxDeviationForDuration = 50 * time.Millisecond
const maxDeviationForRate int64 = 50

// statusesAreEqual returns true if s1 is equal to s2. Equality here means
// general equality of fields except for the duration and rates, which can
// drift due to unpredictable delays (e.g. thread wakes up 25ms after
// `time.Sleep` has ended).
func statusesAreEqual(s1 *Status, s2 *Status) bool {
	if s1.Active == s2.Active &&
		s1.Start == s2.Start &&
		durationsAreEqual(s1.Duration, s2.Duration, maxDeviationForDuration) &&
		s1.Idle == s2.Idle &&
		s1.Bytes == s2.Bytes &&
		s1.Samples == s2.Samples &&
		ratesAreEqual(s1.InstRate, s2.InstRate, maxDeviationForRate) &&
		ratesAreEqual(s1.CurRate, s2.CurRate, maxDeviationForRate) &&
		ratesAreEqual(s1.AvgRate, s2.AvgRate, maxDeviationForRate) &&
		ratesAreEqual(s1.PeakRate, s2.PeakRate, maxDeviationForRate) &&
		s1.BytesRem == s2.BytesRem &&
		durationsAreEqual(s1.TimeRem, s2.TimeRem, maxDeviationForDuration) &&
		s1.Progress == s2.Progress {
		return true
	}
	return false
}

func durationsAreEqual(d1 time.Duration, d2 time.Duration, maxDeviation time.Duration) bool {
	return d2-d1 <= maxDeviation
}

func ratesAreEqual(r1 int64, r2 int64, maxDeviation int64) bool {
	sub := r1 - r2
	if sub < 0 {
		sub = -sub
	}
	if sub <= maxDeviation {
		return true
	}
	return false
}
