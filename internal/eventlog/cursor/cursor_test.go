package cursor_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/tendermint/tendermint/internal/eventlog/cursor"
)

func mustParse(t *testing.T, s string) cursor.Cursor {
	t.Helper()
	var c cursor.Cursor
	if err := c.UnmarshalText([]byte(s)); err != nil {
		t.Fatalf("Unmarshal %q: unexpected error: %v", s, err)
	}
	return c
}

func TestSource_counter(t *testing.T) {
	src := &cursor.Source{
		TimeIndex: func() int64 { return 255 },
	}
	for i := 1; i <= 5; i++ {
		want := fmt.Sprintf("00000000000000ff-%04x", i)
		got := src.Cursor().String()
		if got != want {
			t.Errorf("Cursor %d: got %q, want %q", i, got, want)
		}
	}
}

func TestSource_timeIndex(t *testing.T) {
	times := []int64{0, 1, 100, 65535, 0x76543210fecdba98}
	src := &cursor.Source{
		TimeIndex: func() int64 {
			out := times[0]
			times = append(times[1:], out)
			return out
		},
		Counter: 160,
	}
	results := []string{
		"0000000000000000-00a1",
		"0000000000000001-00a2",
		"0000000000000064-00a3",
		"000000000000ffff-00a4",
		"76543210fecdba98-00a5",
	}
	for i, want := range results {
		if got := src.Cursor().String(); got != want {
			t.Errorf("Cursor %d: got %q, want %q", i+1, got, want)
		}
	}
}

func TestCursor_roundTrip(t *testing.T) {
	const text = `0123456789abcdef-fce9`

	c := mustParse(t, text)
	if got := c.String(); got != text {
		t.Errorf("Wrong string format: got %q, want %q", got, text)
	}
	cmp, err := c.MarshalText()
	if err != nil {
		t.Fatalf("Marshal %+v failed: %v", c, err)
	}
	if got := string(cmp); got != text {
		t.Errorf("Wrong text format: got %q, want %q", got, text)
	}
}

func TestCursor_ordering(t *testing.T) {
	// Condition: text1 precedes text2 in time order.
	// Condition: text2 has an earlier sequence than text1.
	const zero = ""
	const text1 = "0000000012345678-0005"
	const text2 = "00000000fecdeba9-0002"

	zc := mustParse(t, zero)
	c1 := mustParse(t, text1)
	c2 := mustParse(t, text2)

	// Confirm for all pairs that string order respects time order.
	pairs := []struct {
		t1, t2 string
		c1, c2 cursor.Cursor
	}{
		{zero, zero, zc, zc},
		{zero, text1, zc, c1},
		{zero, text2, zc, c2},
		{text1, zero, c1, zc},
		{text1, text1, c1, c1},
		{text1, text2, c1, c2},
		{text2, zero, c2, zc},
		{text2, text1, c2, c1},
		{text2, text2, c2, c2},
	}
	for _, pair := range pairs {
		want := pair.t1 < pair.t2
		if got := pair.c1.Before(pair.c2); got != want {
			t.Errorf("(%s).Before(%s): got %v, want %v", pair.t1, pair.t2, got, want)
		}
	}
}

func TestCursor_IsZero(t *testing.T) {
	tests := []struct {
		text string
		want bool
	}{
		{"", true},
		{"0000000000000000-0000", true},
		{"0000000000000001-0000", false},
		{"0000000000000000-0001", false},
		{"0000000000000001-0001", false},
	}
	for _, test := range tests {
		c := mustParse(t, test.text)
		if got := c.IsZero(); got != test.want {
			t.Errorf("IsZero(%q): got %v, want %v", test.text, got, test.want)
		}
	}
}

func TestCursor_Diff(t *testing.T) {
	const time1 = 0x1ac0193001
	const time2 = 0x0ac0193001

	text1 := fmt.Sprintf("%016x-0001", time1)
	text2 := fmt.Sprintf("%016x-0005", time2)
	want := time.Duration(time1 - time2)

	c1 := mustParse(t, text1)
	c2 := mustParse(t, text2)

	got := c1.Diff(c2)
	if got != want {
		t.Fatalf("Diff %q - %q: got %v, want %v", text1, text2, got, want)
	}
}
