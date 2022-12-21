// Package cursor implements time-ordered item cursors for an event log.
package cursor

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// A Source produces cursors based on a time index generator and a sequence
// counter. A zero-valued Source is ready for use with defaults as described.
type Source struct {
	// This function is called to produce the current time index.
	// If nil, it defaults to time.Now().UnixNano().
	TimeIndex func() int64

	// The current counter value used for sequence number generation.  It is
	// incremented in-place each time a cursor is generated.
	Counter int64
}

func (s *Source) timeIndex() int64 {
	if s.TimeIndex == nil {
		return time.Now().UnixNano()
	}
	return s.TimeIndex()
}

func (s *Source) nextCounter() int64 {
	s.Counter++
	return s.Counter
}

// Cursor produces a fresh cursor from s at the current time index and counter.
func (s *Source) Cursor() Cursor {
	return Cursor{
		timestamp: uint64(s.timeIndex()),
		sequence:  uint16(s.nextCounter() & 0xffff),
	}
}

// A Cursor is a unique identifier for an item in a time-ordered event log.
// It is safe to copy and compare cursors by value.
type Cursor struct {
	timestamp uint64 // ns since Unix epoch
	sequence  uint16 // sequence number
}

// Before reports whether c is prior to o in time ordering. This comparison
// ignores sequence numbers.
func (c Cursor) Before(o Cursor) bool { return c.timestamp < o.timestamp }

// Diff returns the time duration between c and o. The duration is negative if
// c is before o in time order.
func (c Cursor) Diff(o Cursor) time.Duration {
	return time.Duration(c.timestamp) - time.Duration(o.timestamp)
}

// IsZero reports whether c is the zero cursor.
func (c Cursor) IsZero() bool { return c == Cursor{} }

// MarshalText implements the encoding.TextMarshaler interface.
// A zero cursor marshals as "", otherwise the format used by the String method.
func (c Cursor) MarshalText() ([]byte, error) {
	if c.IsZero() {
		return nil, nil
	}
	return []byte(c.String()), nil
}

// UnmarshalText implements the encoding.TextUnmarshaler interface.
// An empty text unmarshals without error to a zero cursor.
func (c *Cursor) UnmarshalText(data []byte) error {
	if len(data) == 0 {
		*c = Cursor{} // set zero
		return nil
	}
	ps := strings.SplitN(string(data), "-", 2)
	if len(ps) != 2 {
		return errors.New("invalid cursor format")
	}
	ts, err := strconv.ParseUint(ps[0], 16, 64)
	if err != nil {
		return fmt.Errorf("invalid timestamp: %w", err)
	}
	sn, err := strconv.ParseUint(ps[1], 16, 16)
	if err != nil {
		return fmt.Errorf("invalid sequence: %w", err)
	}
	c.timestamp = ts
	c.sequence = uint16(sn)
	return nil
}

// String returns a printable text representation of a cursor.
func (c Cursor) String() string {
	return fmt.Sprintf("%016x-%04x", c.timestamp, c.sequence)
}
