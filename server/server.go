package server

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"

	. "github.com/tendermint/go-common"
	"github.com/tendermint/go-wire"
	"github.com/tendermint/tmsp/types"
)

// var maxNumberConnections = 2

func StartListener(protoAddr string, app types.Application) (net.Listener, error) {
	var mtx sync.Mutex // global mutex
	parts := strings.SplitN(protoAddr, "://", 2)
	proto, addr := parts[0], parts[1]
	ln, err := net.Listen(proto, addr)
	if err != nil {
		return nil, err
	}

	// A goroutine to accept a connection.
	go func() {
		// semaphore := make(chan struct{}, maxNumberConnections)

		for {
			// semaphore <- struct{}{}

			// Accept a connection
			fmt.Println("Waiting for new connection...")
			conn, err := ln.Accept()
			if err != nil {
				Exit("Failed to accept connection")
			} else {
				fmt.Println("Accepted a new connection")
			}

			closeConn := make(chan error, 2)             // Push to signal connection closed
			responses := make(chan types.Response, 1000) // A channel to buffer responses

			// Read requests from conn and deal with them
			go handleRequests(&mtx, app, closeConn, conn, responses)
			// Pull responses from 'responses' and write them to conn.
			go handleResponses(closeConn, responses, conn)

			go func() {
				// Wait until signal to close connection
				errClose := <-closeConn
				if errClose != nil {
					fmt.Printf("Connection error: %v\n", errClose)
				} else {
					fmt.Println("Connection was closed.")
				}

				// Close the connection
				err := conn.Close()
				if err != nil {
					fmt.Printf("Error in closing connection: %v\n", err)
				}

				// <-semaphore
			}()
		}

	}()

	return ln, nil
}

// Read requests from conn and deal with them
func handleRequests(mtx *sync.Mutex, app types.Application, closeConn chan error, conn net.Conn, responses chan<- types.Response) {
	var count int
	var bufReader = bufio.NewReader(conn)
	for {
		var n int
		var err error
		var req types.Request
		wire.ReadBinaryPtrLengthPrefixed(&req, bufReader, 0, &n, &err)
		if err != nil {
			if err == io.EOF {
				closeConn <- fmt.Errorf("Connection closed by client")
			} else {
				closeConn <- fmt.Errorf("Error in handleRequests: %v", err.Error())
			}
			return
		}
		mtx.Lock()
		count++
		handleRequest(app, req, responses)
		mtx.Unlock()
	}
}

func handleRequest(app types.Application, req types.Request, responses chan<- types.Response) {
	switch req := req.(type) {
	case types.RequestEcho:
		responses <- types.ResponseEcho{req.Message}
	case types.RequestFlush:
		responses <- types.ResponseFlush{}
	case types.RequestInfo:
		data := app.Info()
		responses <- types.ResponseInfo{data}
	case types.RequestSetOption:
		logStr := app.SetOption(req.Key, req.Value)
		responses <- types.ResponseSetOption{logStr}
	case types.RequestAppendTx:
		code, result, logStr := app.AppendTx(req.TxBytes)
		responses <- types.ResponseAppendTx{code, result, logStr}
	case types.RequestCheckTx:
		code, result, logStr := app.CheckTx(req.TxBytes)
		responses <- types.ResponseCheckTx{code, result, logStr}
	case types.RequestGetHash:
		hash, logStr := app.GetHash()
		responses <- types.ResponseGetHash{hash, logStr}
	case types.RequestQuery:
		result, logStr := app.Query(req.QueryBytes)
		responses <- types.ResponseQuery{result, logStr}
	default:
		responses <- types.ResponseException{"Unknown request"}
	}
}

// Pull responses from 'responses' and write them to conn.
func handleResponses(closeConn chan error, responses <-chan types.Response, conn net.Conn) {
	var count int
	var bufWriter = bufio.NewWriter(conn)
	for {
		var res = <-responses
		var n int
		var err error
		wire.WriteBinaryLengthPrefixed(struct{ types.Response }{res}, bufWriter, &n, &err)
		if err != nil {
			closeConn <- fmt.Errorf("Error in handleResponses: %v", err.Error())
			return
		}
		if _, ok := res.(types.ResponseFlush); ok {
			err = bufWriter.Flush()
			if err != nil {
				closeConn <- fmt.Errorf("Error in handleResponses: %v", err.Error())
				return
			}
		}
		count++
	}
}
