package main

import (
	"bufio"
	"fmt"
	//"encoding/hex"

	. "github.com/tendermint/go-common"
	"github.com/tendermint/abci/types"
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
			var res = &types.Response{}
			err := types.ReadMessage(conn, res)
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
		var req = types.ToRequestEcho("foobar")

		err := types.WriteMessage(req, bufWriter)
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
