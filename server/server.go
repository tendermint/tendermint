package server

import (
	"bufio"
	"fmt"
	"net"
	"reflect"
	"strings"

	. "github.com/tendermint/go-common"
	"github.com/tendermint/go-wire"
	"github.com/tendermint/tmsp/types"
)

func StartListener(protoAddr string, app types.Application) (net.Listener, error) {
	parts := strings.SplitN(protoAddr, "://", 2)
	proto, addr := parts[0], parts[1]
	ln, err := net.Listen(proto, addr)
	if err != nil {
		return nil, err
	}

	// A goroutine to accept a connection.
	go func() {

		for {
			// Accept a connection
			conn, err := ln.Accept()
			if err != nil {
				Exit("Failed to accept connection")
			} else {
				fmt.Println("Accepted a new connection")
			}
			connClosed := make(chan struct{}, 2)         // Push to signal connection closed
			responses := make(chan types.Response, 1000) // A channel to buffer responses

			// Read requests from conn and deal with them
			go handleRequests(app, connClosed, conn, responses)
			// Pull responses from 'responses' and write them to conn.
			go handleResponses(connClosed, responses, conn)

			// Wait until connection is closed
			<-connClosed
			fmt.Println("Connection was closed. Waiting for new connection...")
		}

	}()

	return ln, nil
}

// Read requests from conn and deal with them
func handleRequests(app types.Application, connClosed chan struct{}, conn net.Conn, responses chan<- types.Response) {
	var count int
	var bufReader = bufio.NewReader(conn)
	for {
		var n int64
		var err error
		var req types.Request
		wire.ReadBinaryPtr(&req, bufReader, &n, &err)

		if err != nil {
			fmt.Println(err.Error())
			connClosed <- struct{}{}
			return
		}

		count++
		if count%1000 == 0 {
			fmt.Println("Received request", reflect.TypeOf(req), req, n, err, count)
		}

		handleRequest(app, req, responses)
	}
}

func handleRequest(app types.Application, req types.Request, responses chan<- types.Response) {
	switch req := req.(type) {
	case types.RequestEcho:
		retCode, msg := app.Echo(req.Message)
		responses <- types.ResponseEcho{retCode, msg}
	case types.RequestFlush:
		responses <- types.ResponseFlush{}
	case types.RequestAppendTx:
		retCode := app.AppendTx(req.TxBytes)
		responses <- types.ResponseAppendTx{retCode}
		events := app.GetEvents()
		for _, event := range events {
			responses <- types.ResponseEvent{event}
		}
	case types.RequestGetHash:
		hash, retCode := app.GetHash()
		responses <- types.ResponseGetHash{retCode, hash}
	case types.RequestCommit:
		retCode := app.Commit()
		responses <- types.ResponseCommit{retCode}
	case types.RequestRollback:
		retCode := app.Rollback()
		responses <- types.ResponseRollback{retCode}
	case types.RequestSetEventsMode:
		retCode := app.SetEventsMode(req.EventsMode)
		responses <- types.ResponseSetEventsMode{retCode}
		if req.EventsMode == types.EventsModeOn {
			events := app.GetEvents()
			for _, event := range events {
				responses <- types.ResponseEvent{event}
			}
		}
	case types.RequestAddListener:
		retCode := app.AddListener(req.EventKey)
		responses <- types.ResponseAddListener{retCode}
	case types.RequestRemListener:
		retCode := app.RemListener(req.EventKey)
		responses <- types.ResponseRemListener{retCode}
	default:
		responses <- types.ResponseException{"Unknown request"}
	}
}

// Pull responses from 'responses' and write them to conn.
func handleResponses(connClosed chan struct{}, responses <-chan types.Response, conn net.Conn) {
	var count int
	var bufWriter = bufio.NewWriter(conn)
	for {
		var res = <-responses
		var n int64
		var err error
		wire.WriteBinary(res, bufWriter, &n, &err)
		if err != nil {
			fmt.Println(err.Error())
			connClosed <- struct{}{}
			return
		}

		if _, ok := res.(types.ResponseFlush); ok {
			err = bufWriter.Flush()
			if err != nil {
				fmt.Println(err.Error())
				connClosed <- struct{}{}
				return
			}
		}

		count++
		if count%1000 == 0 {
			fmt.Println("Sent response", reflect.TypeOf(res), res, n, err, count)
		}
	}
}
