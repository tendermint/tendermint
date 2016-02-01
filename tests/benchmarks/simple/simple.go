package main

import (
	"bufio"
	"errors"
	"fmt"
	"net"
	//"encoding/hex"

	. "github.com/tendermint/go-common"
	"github.com/tendermint/tmsp/types"
)

func main() {

	conn, err := Connect("unix://test.sock")
	if err != nil {
		Exit(err.Error())
	}

	// Make a bunch of requests
	counter := 0
	for i := 0; ; i++ {
		req := types.RequestEcho("foobar")
		_, err := makeRequest(conn, req)
		if err != nil {
			Exit(err.Error())
		}
		counter += 1
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
	err = types.WriteMessage(types.RequestFlush(), bufWriter)
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
	if resFlush.Type != types.MessageType_Flush {
		return nil, errors.New(Fmt("Expected flush response but got something else: %v", resFlush.Type))
	}

	return res, nil
}
