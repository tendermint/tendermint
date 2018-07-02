package main

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"reflect"

	"github.com/tendermint/tendermint/abci/types"
	cmn "github.com/tendermint/tendermint/libs/common"
)

func main() {

	conn, err := cmn.Connect("unix://test.sock")
	if err != nil {
		log.Fatal(err.Error())
	}

	// Make a bunch of requests
	counter := 0
	for i := 0; ; i++ {
		req := types.ToRequestEcho("foobar")
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

func makeRequest(conn net.Conn, req *types.Request) (*types.Response, error) {
	var bufWriter = bufio.NewWriter(conn)

	// Write desired request
	err := types.WriteMessage(req, bufWriter)
	if err != nil {
		return nil, err
	}
	err = types.WriteMessage(types.ToRequestFlush(), bufWriter)
	if err != nil {
		return nil, err
	}
	err = bufWriter.Flush()
	if err != nil {
		return nil, err
	}

	// Read desired response
	var res = &types.Response{}
	err = types.ReadMessage(conn, res)
	if err != nil {
		return nil, err
	}
	var resFlush = &types.Response{}
	err = types.ReadMessage(conn, resFlush)
	if err != nil {
		return nil, err
	}
	if _, ok := resFlush.Value.(*types.Response_Flush); !ok {
		return nil, fmt.Errorf("Expected flush response but got something else: %v", reflect.TypeOf(resFlush))
	}

	return res, nil
}
