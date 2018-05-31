package fuzz_test

import (
	"fmt"

	"github.com/tendermint/tendermint/libs/pubsub/query"
)

func Fuzz(data []byte) int {
	sdata := string(data)
	q0, err := query.New(sdata)
	if err != nil {
		return 0
	}

	sdata1 := q0.String()
	q1, err := query.New(sdata1)
	if err != nil {
		panic(err)
	}

	sdata2 := q1.String()
	if sdata1 != sdata2 {
		fmt.Printf("q0: %q\n", sdata1)
		fmt.Printf("q1: %q\n", sdata2)
		panic("query changed")
	}

	return 1
}
