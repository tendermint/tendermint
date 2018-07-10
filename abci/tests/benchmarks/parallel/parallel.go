package main

import (
	"bufio"
	"fmt"
	"log"

	"github.com/tendermint/tendermint/abci/types"
	cmn "github.com/tendermint/tendermint/libs/common"
)

func main() {

	conn, err := cmn.Connect("unix://test.sock")
	if err != nil {
		log.Fatal(err.Error())
	}

	// Read a bunch of responses
	go func() {
		counter := 0
		for {
			var res = &types.Response{}
			err := types.ReadMessage(conn, res)
			if err != nil {
				log.Fatal(err.Error())
			}
			counter++
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
			log.Fatal(err.Error())
		}
		err = bufWriter.Flush()
		if err != nil {
			log.Fatal(err.Error())
		}

		counter++
		if counter%1000 == 0 {
			fmt.Println("Write", counter)
		}
	}
}
