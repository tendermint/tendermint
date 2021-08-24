package main

import (
	"bufio"
	"fmt"
	"log"

	tmnet "github.com/tendermint/tendermint/libs/net"
	"github.com/tendermint/tendermint/pkg/abci"
)

func main() {

	conn, err := tmnet.Connect("unix://test.sock")
	if err != nil {
		log.Fatal(err.Error())
	}

	// Read a bunch of responses
	go func() {
		counter := 0
		for {
			var res = &abci.Response{}
			err := abci.ReadMessage(conn, res)
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
		var req = abci.ToRequestEcho("foobar")

		err := abci.WriteMessage(req, bufWriter)
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
