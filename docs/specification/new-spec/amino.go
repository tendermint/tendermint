package main

import (
	"fmt"
	"time"

	amino "github.com/tendermint/go-amino"
)

func main() {
	cdc := amino.NewCodec()

	encode(cdc, uint8(6))
	encode(cdc, uint32(6))
	encode(cdc, int8(-6))
	encode(cdc, int32(-6))
	Break()
	encode(cdc, uint(6))
	encode(cdc, uint(70000))
	encode(cdc, int(0))
	encode(cdc, int(-6))
	encode(cdc, int(-70000))
	Break()
	encode(cdc, "")
	encode(cdc, "a")
	encode(cdc, "hello")
	encode(cdc, "¥")
	Break()
	encode(cdc, [4]int8{1, 2, 3, 4})
	encode(cdc, [4]int16{1, 2, 3, 4})
	encode(cdc, [4]int{1, 2, 3, 4})
	encode(cdc, [2]string{"abc", "efg"})
	Break()
	encode(cdc, []int8{})
	encode(cdc, []int8{1, 2, 3, 4})
	encode(cdc, []int16{1, 2, 3, 4})
	encode(cdc, []int{1, 2, 3, 4})
	encode(cdc, []string{"abc", "efg"})
	Break()

	timeFmt := "Mon Jan 2 15:04:05 -0700 MST 2006"
	t1, _ := time.Parse(timeFmt, timeFmt)
	n := (t1.UnixNano() / 1000000.) * 1000000
	encode(cdc, n)
	encode(cdc, t1)

	t2, _ := time.Parse(timeFmt, "Thu Jan 1 00:00:00 -0000 UTC 1970")
	encode(cdc, t2)

	t2, _ = time.Parse(timeFmt, "Thu Jan 1 00:00:01 -0000 UTC 1970")
	fmt.Println("N", t2.UnixNano())
	encode(cdc, t2)
	Break()
	encode(cdc, struct {
		A int
		B string
		C time.Time
	}{
		4,
		"hello",
		t1,
	})
}

func encode(cdc *amino.Codec, i interface{}) {
	bz, err := cdc.MarshalBinary(i)
	if err != nil {
		panic(err)
	}
	Println(bz)
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
