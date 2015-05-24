package binary

import (
	"bytes"
	"fmt"
	"testing"
)

func TestVarint(t *testing.T) {

	check := func(i int, s string) {
		buf := new(bytes.Buffer)
		n, err := new(int64), new(error)
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

	// 123457 is some prime.
	for i := -(2 << 33); i < (2 << 33); i += 123457 {
		check(i, "")
	}

	// Near zero
	check(-1, "F101")
	check(0, "00")
	check(1, "0101")
	// Positives
	check(1<<32-1, "04FFFFFFFF")
	check(1<<32+0, "050100000000")
	check(1<<32+1, "050100000001")
	check(1<<53-1, "071FFFFFFFFFFFFF")
	// Negatives
	check(-1<<32+1, "F4FFFFFFFF")
	check(-1<<32-0, "F50100000000")
	check(-1<<32-1, "F50100000001")
	check(-1<<53+1, "F71FFFFFFFFFFFFF")
}

func TestUvarint(t *testing.T) {

	check := func(i uint, s string) {
		buf := new(bytes.Buffer)
		n, err := new(int64), new(error)
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

	// 123457 is some prime.
	for i := 0; i < (2 << 33); i += 123457 {
		check(uint(i), "")
	}

	check(1, "0101")
	check(1<<32-1, "04FFFFFFFF")
	check(1<<32+0, "050100000000")
	check(1<<32+1, "050100000001")
	check(1<<53-1, "071FFFFFFFFFFFFF")

}
