package main

import (
	"bufio"
	"errors"
	"fmt"
	"net"
	"reflect"
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

	// Make a bunch of requests
	counter := 0
	for i := 0; ; i++ {
		req := types.RequestEcho{"foobar"}
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

func makeRequest(conn net.Conn, req types.Request) (types.Response, error) {
	var bufWriter = bufio.NewWriter(conn)
	var n int
	var err error

	// Write desired request
	wire.WriteBinaryLengthPrefixed(struct{ types.Request }{req}, bufWriter, &n, &err)
	if err != nil {
		return nil, err
	}
	bufWriter.Write([]byte{0x01, 0x01, types.RequestTypeFlush}) // Write flush msg
	err = bufWriter.Flush()
	if err != nil {
		return nil, err
	}

	// Read desired response
	var res types.Response
	wire.ReadBinaryPtrLengthPrefixed(&res, conn, 0, &n, &err)
	if err != nil {
		return nil, err
	}
	var resFlush types.Response // Read flush msg
	wire.ReadBinaryPtrLengthPrefixed(&resFlush, conn, 0, &n, &err)
	if err != nil {
		return nil, err
	}
	if _, ok := resFlush.(types.ResponseFlush); !ok {
		return nil, errors.New(Fmt("Expected flush response but got something else", reflect.TypeOf(resFlush)))
	}

	return res, nil
}
