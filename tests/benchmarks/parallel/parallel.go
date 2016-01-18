package main

import (
	"bufio"
	"fmt"
	//"encoding/hex"

	. "github.com/tendermint/go-common"
	"github.com/tendermint/go-wire"
	"github.com/tendermint/tmsp/types"
)

func main() {

	conn, err := Connect("unix://test.sock")
	if err != nil {
		Exit(err.Error())
	}

	// Read a bunch of responses
	go func() {
		counter := 0
		for {
			var res types.Response
			var n int
			var err error
			wire.ReadBinaryPtrLengthPrefixed(&res, conn, 0, &n, &err)
			if err != nil {
				Exit(err.Error())
			}
			counter += 1
			if counter%1000 == 0 {
				fmt.Println("Read", counter)
			}
		}
	}()

	// Write a bunch of requests
	counter := 0
	for i := 0; ; i++ {
		var bufWriter = bufio.NewWriter(conn)
		var req types.Request = types.RequestEcho{"foobar"}
		var n int
		var err error
		wire.WriteBinaryLengthPrefixed(struct{ types.Request }{req}, bufWriter, &n, &err)
		if err != nil {
			Exit(err.Error())
		}
		err = bufWriter.Flush()
		if err != nil {
			Exit(err.Error())
		}
		counter += 1
		if counter%1000 == 0 {
			fmt.Println("Write", counter)
		}
	}
}
