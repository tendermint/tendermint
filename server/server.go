package server

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"strings"

	. "github.com/tendermint/go-common"
	"github.com/tendermint/go-wire"
	"github.com/tendermint/tmsp/types"
)

// var maxNumberConnections = 2

func StartListener(protoAddr string, app types.Application) (net.Listener, error) {
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

			appContext := app.Open()
			closeConn := make(chan error, 2)             // Push to signal connection closed
			responses := make(chan types.Response, 1000) // A channel to buffer responses

			// Read requests from conn and deal with them
			go handleRequests(appContext, closeConn, conn, responses)
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

				// Close the AppContext
				err = appContext.Close()
				if err != nil {
					fmt.Printf("Error in closing app context: %v\n", err)
				}

				// <-semaphore
			}()
		}

	}()

	return ln, nil
}

// Read requests from conn and deal with them
func handleRequests(appC types.AppContext, closeConn chan error, conn net.Conn, responses chan<- types.Response) {
	var count int
	var bufReader = bufio.NewReader(conn)
	for {
		var n int
		var err error
		var req types.Request
		wire.ReadBinaryPtr(&req, bufReader, 0, &n, &err)
		if err != nil {
			if err == io.EOF {
				closeConn <- fmt.Errorf("Connection closed by client")
			} else {
				closeConn <- fmt.Errorf("Error in handleRequests: %v", err.Error())
			}
			return
		}
		count++
		handleRequest(appC, req, responses)
	}
}

func handleRequest(appC types.AppContext, req types.Request, responses chan<- types.Response) {
	switch req := req.(type) {
	case types.RequestEcho:
		msg := appC.Echo(req.Message)
		responses <- types.ResponseEcho{msg}
	case types.RequestFlush:
		responses <- types.ResponseFlush{}
	case types.RequestInfo:
		data := appC.Info()
		responses <- types.ResponseInfo{data}
	case types.RequestSetOption:
		retCode := appC.SetOption(req.Key, req.Value)
		responses <- types.ResponseSetOption{retCode}
	case types.RequestAppendTx:
		events, retCode := appC.AppendTx(req.TxBytes)
		responses <- types.ResponseAppendTx{retCode}
		for _, event := range events {
			responses <- types.ResponseEvent{event}
		}
	case types.RequestGetHash:
		hash, retCode := appC.GetHash()
		responses <- types.ResponseGetHash{retCode, hash}
	case types.RequestCommit:
		retCode := appC.Commit()
		responses <- types.ResponseCommit{retCode}
	case types.RequestRollback:
		retCode := appC.Rollback()
		responses <- types.ResponseRollback{retCode}
	case types.RequestAddListener:
		retCode := appC.AddListener(req.EventKey)
		responses <- types.ResponseAddListener{retCode}
	case types.RequestRemListener:
		retCode := appC.RemListener(req.EventKey)
		responses <- types.ResponseRemListener{retCode}
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
		wire.WriteBinary(res, bufWriter, &n, &err)
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
