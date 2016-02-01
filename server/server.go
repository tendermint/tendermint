package server

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"

	. "github.com/tendermint/go-common"
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

			closeConn := make(chan error, 2)              // Push to signal connection closed
			responses := make(chan *types.Response, 1000) // A channel to buffer responses

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
func handleRequests(mtx *sync.Mutex, app types.Application, closeConn chan error, conn net.Conn, responses chan<- *types.Response) {
	var count int
	var bufReader = bufio.NewReader(conn)
	for {

		var req = &types.Request{}
		err := types.ReadMessage(bufReader, req)
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

func handleRequest(app types.Application, req *types.Request, responses chan<- *types.Response) {
	switch req.Type {
	case types.MessageType_Echo:
		responses <- types.ResponseEcho(string(req.Data))
	case types.MessageType_Flush:
		responses <- types.ResponseFlush()
	case types.MessageType_Info:
		data := app.Info()
		responses <- types.ResponseInfo(data)
	case types.MessageType_SetOption:
		logStr := app.SetOption(req.Key, req.Value)
		responses <- types.ResponseSetOption(logStr)
	case types.MessageType_AppendTx:
		code, result, logStr := app.AppendTx(req.Data)
		responses <- types.ResponseAppendTx(code, result, logStr)
	case types.MessageType_CheckTx:
		code, result, logStr := app.CheckTx(req.Data)
		responses <- types.ResponseCheckTx(code, result, logStr)
	case types.MessageType_GetHash:
		hash, logStr := app.GetHash()
		responses <- types.ResponseGetHash(hash, logStr)
	case types.MessageType_Query:
		code, result, logStr := app.Query(req.Data)
		responses <- types.ResponseQuery(code, result, logStr)
	default:
		responses <- types.ResponseException("Unknown request")
	}
}

// Pull responses from 'responses' and write them to conn.
func handleResponses(closeConn chan error, responses <-chan *types.Response, conn net.Conn) {
	var count int
	var bufWriter = bufio.NewWriter(conn)
	for {
		var res = <-responses
		err := types.WriteMessage(res, bufWriter)
		if err != nil {
			closeConn <- fmt.Errorf("Error in handleResponses: %v", err.Error())
			return
		}
		if res.Type == types.MessageType_Flush {
			err = bufWriter.Flush()
			if err != nil {
				closeConn <- fmt.Errorf("Error in handleResponses: %v", err.Error())
				return
			}
		}
		count++
	}
}
