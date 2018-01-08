package wire

import (
	"bytes"
	"fmt"
	"testing"
)

const arch64 = uint64(^uint(0)) == ^uint64(0)

func TestInt16(t *testing.T) {
	buf := new(bytes.Buffer)
	n, err := new(int), new(error)
	WriteUint16(11, buf, n, err)
	if *n != 2 {
		t.Fatalf("Encoded uint16 expected 2 bytes got %d", *n)
	}
}

// Returns true of overflow or underflow
func checkIntSpill(i int64) bool {
	return i == (int64)((int)(i))
}

// Returns true of overflow or underflow
func checkUintSpill(i uint64) bool {
	return i == (uint64)((uint)(i))
}

func TestVarint(t *testing.T) {

	check := func(i64 int64, s string) {
		i := int(i64)
		// Test with WriteVarint
		{
			buf := new(bytes.Buffer)
			n, err := new(int), new(error)
			WriteVarint(i, buf, n, err)
			bufBytes := buf.Bytes() // Read before consuming below.
			i_ := ReadVarint(buf, n, err)
			if i != i_ {
				fmt.Println(bufBytes)
				t.Fatalf("Encoded %v and got %v", i, i_)
			}
			if s != "" {
				if bufHex := fmt.Sprintf("%X", bufBytes); bufHex != s {
					t.Fatalf("Encoded %v, expected %v", bufHex, s)
				}
			}
		}
		// Test with PutVarint
		{
			buf := make([]byte, 100, 100)
			n1, err := PutVarint(buf, i)
			if err != nil {
				t.Errorf("Unexpected error in PutVarint %v (encoding %v)", err.Error(), i)
			}
			if s != "" {
				if n1 != len(s)/2 {
					t.Errorf("Unexpected PutVarint length %v, expected %v (encoding %v)", n1, len(s)/2, i)
				}
				if bufHex := fmt.Sprintf("%X", buf[:n1]); bufHex != s {
					t.Errorf("Got %v, expected %v (encoding %v)", bufHex, s, i)
				}
			}
			i2, n2, err := GetVarint(buf)
			if err != nil {
				t.Errorf("Unexpected error in GetVarint %v (encoding %v)", err.Error(), i)
			}
			if s != "" {
				if n2 != len(s)/2 {
					t.Errorf("Unexpected GetVarint length %v, expected %v (encoding %v)", n2, len(s)/2, i)
				}
			} else {
				if n1 != n2 {
					t.Errorf("Unmatched Put/Get lengths. %v vs %v (encoding %v)", n1, n2, i)
				}
			}
			if i != i2 {
				t.Errorf("Put/Get expected %v, got %v (encoding %v)", i, i2, i)
			}
		}
	}

	// 123457 is some prime.
	var i int64
	for i = -(2 << 33); i < (2 << 33); i += 123457 {
		if !arch64 && checkIntSpill(i) {
			continue
		}
		check(i, "")
	}

	// Near zero
	check(-1, "F101")
	check(0, "00")
	check(1, "0101")
	// Positives
	check(1<<31-1, "047FFFFFFF")
	if arch64 {
		check(1<<32-1, "04FFFFFFFF")
		check(1<<32+0, "050100000000")
		check(1<<32+1, "050100000001")
		check(1<<53-1, "071FFFFFFFFFFFFF")
		check(1<<63-1, "087FFFFFFFFFFFFFFF")
	}
	// Negatives
	check(-1<<31+1, "F47FFFFFFF")
	if arch64 {
		check(-1<<32+1, "F4FFFFFFFF")
		check(-1<<32-0, "F50100000000")
		check(-1<<32-1, "F50100000001")
		check(-1<<53+1, "F71FFFFFFFFFFFFF")
		check(-1<<63, "F88000000000000000")
		check(-1<<63+1, "F87FFFFFFFFFFFFFFF")
	}
}

func TestUvarint(t *testing.T) {

	check := func(i64 uint64, s string) {
		i := uint(i64)
		// Test with WriteUvarint
		{
			buf := new(bytes.Buffer)
			n, err := new(int), new(error)
			WriteUvarint(i, buf, n, err)
			bufBytes := buf.Bytes()
			i_ := ReadUvarint(buf, n, err)
			if i != i_ {
				fmt.Println(buf.Bytes())
				t.Fatalf("Encoded %v and got %v", i, i_)
			}
			if s != "" {
				if bufHex := fmt.Sprintf("%X", bufBytes); bufHex != s {
					t.Fatalf("Encoded %v, expected %v", bufHex, s)
				}
			}
		}
		// Test with PutUvarint
		{
			buf := make([]byte, 100, 100)
			n1, err := PutUvarint(buf, i)
			if err != nil {
				t.Errorf("Unexpected error in PutUvarint %v (encoding %v)", err.Error(), i)
			}
			if s != "" {
				if n1 != len(s)/2 {
					t.Errorf("Unexpected PutUvarint length %v, expected %v (encoding %v)", n1, len(s)/2, i)
				}
				if bufHex := fmt.Sprintf("%X", buf[:n1]); bufHex != s {
					t.Errorf("Got %v, expected %v (encoding %v)", bufHex, s, i)
				}
			}
			i2, n2, err := GetUvarint(buf)
			if err != nil {
				t.Errorf("Unexpected error in GetUvarint %v (encoding %v)", err.Error(), i)
			}
			if s != "" {
				if n2 != len(s)/2 {
					t.Errorf("Unexpected GetUvarint length %v, expected %v (encoding %v)", n2, len(s)/2, i)
				}
			} else {
				if n1 != n2 {
					t.Errorf("Unmatched Put/Get lengths. %v vs %v (encoding %v)", n1, n2, i)
				}
			}
			if i != i2 {
				t.Errorf("Put/Get expected %v, got %v (encoding %v)", i, i2, i)
			}
		}
	}

	// 123457 is some prime.
	var i uint64
	for i = 0; i < (2 << 33); i += 123457 {
		if !arch64 && checkUintSpill(i) {
			continue
		}
		check(i, "")
	}

	check(1, "0101")
	check(1<<31-1, "047FFFFFFF")
	check(1<<32-1, "04FFFFFFFF")
	if arch64 {
		check(1<<32-1, "04FFFFFFFF")
		check(1<<32+0, "050100000000")
		check(1<<32+1, "050100000001")
		check(1<<53-1, "071FFFFFFFFFFFFF")
	}

}
