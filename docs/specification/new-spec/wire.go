package main

import (
	"fmt"
	"time"

	wire "github.com/tendermint/go-wire"
)

func main() {

	encode(uint8(6))
	encode(uint32(6))
	encode(int8(-6))
	encode(int32(-6))
	Break()
	encode(uint(6))
	encode(uint(70000))
	encode(int(0))
	encode(int(-6))
	encode(int(-70000))
	Break()
	encode("")
	encode("a")
	encode("hello")
	encode("¥")
	Break()
	encode([4]int8{1, 2, 3, 4})
	encode([4]int16{1, 2, 3, 4})
	encode([4]int{1, 2, 3, 4})
	encode([2]string{"abc", "efg"})
	Break()
	encode([]int8{})
	encode([]int8{1, 2, 3, 4})
	encode([]int16{1, 2, 3, 4})
	encode([]int{1, 2, 3, 4})
	encode([]string{"abc", "efg"})
	Break()

	timeFmt := "Mon Jan 2 15:04:05 -0700 MST 2006"
	t1, _ := time.Parse(timeFmt, timeFmt)
	n := (t1.UnixNano() / 1000000.) * 1000000
	encode(n)
	encode(t1)

	t2, _ := time.Parse(timeFmt, "Thu Jan 1 00:00:00 -0000 UTC 1970")
	encode(t2)

	t2, _ = time.Parse(timeFmt, "Thu Jan 1 00:00:01 -0000 UTC 1970")
	fmt.Println("N", t2.UnixNano())
	encode(t2)
	Break()
	encode(struct {
		A int
		B string
		C time.Time
	}{
		4,
		"hello",
		t1,
	})
}

func encode(i interface{}) {
	Println(wire.BinaryBytes(i))

}

func Println(b []byte) {
	s := "["
	for _, x := range b {
		s += fmt.Sprintf("0x%.2X, ", x)
	}
	s = s[:len(s)-2] + "]"
	fmt.Println(s)
}

func Break() {
	fmt.Println("------")
}
