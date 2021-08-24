package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"reflect"

	tmnet "github.com/tendermint/tendermint/libs/net"
	"github.com/tendermint/tendermint/pkg/abci"
)

func main() {

	conn, err := tmnet.Connect("unix://test.sock")
	if err != nil {
		log.Fatal(err.Error())
	}

	// Make a bunch of requests
	counter := 0
	for i := 0; ; i++ {
		req := abci.ToRequestEcho("foobar")
		_, err := makeRequest(conn, req)
		if err != nil {
			log.Fatal(err.Error())
		}
		counter++
		if counter%1000 == 0 {
			fmt.Println(counter)
		}
	}
}

func makeRequest(conn io.ReadWriter, req *abci.Request) (*abci.Response, error) {
	var bufWriter = bufio.NewWriter(conn)

	// Write desired request
	err := abci.WriteMessage(req, bufWriter)
	if err != nil {
		return nil, err
	}
	err = abci.WriteMessage(abci.ToRequestFlush(), bufWriter)
	if err != nil {
		return nil, err
	}
	err = bufWriter.Flush()
	if err != nil {
		return nil, err
	}

	// Read desired response
	var res = &abci.Response{}
	err = abci.ReadMessage(conn, res)
	if err != nil {
		return nil, err
	}
	var resFlush = &abci.Response{}
	err = abci.ReadMessage(conn, resFlush)
	if err != nil {
		return nil, err
	}
	if _, ok := resFlush.Value.(*abci.Response_Flush); !ok {
		return nil, fmt.Errorf("expected flush response but got something else: %v", reflect.TypeOf(resFlush))
	}

	return res, nil
}
