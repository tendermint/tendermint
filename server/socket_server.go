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

type SocketServer struct {
	QuitService

	proto    string
	addr     string
	listener net.Listener

	appMtx sync.Mutex
	app    types.Application
}

func NewSocketServer(protoAddr string, app types.Application) (Service, error) {
	parts := strings.SplitN(protoAddr, "://", 2)
	proto, addr := parts[0], parts[1]
	s := &SocketServer{
		proto:    proto,
		addr:     addr,
		listener: nil,
		app:      app,
	}
	s.QuitService = *NewQuitService(nil, "TMSPServer", s)
	_, err := s.Start() // Just start it
	return s, err
}

func (s *SocketServer) OnStart() error {
	s.QuitService.OnStart()
	ln, err := net.Listen(s.proto, s.addr)
	if err != nil {
		return err
	}
	s.listener = ln
	go s.acceptConnectionsRoutine()
	return nil
}

func (s *SocketServer) OnStop() {
	s.QuitService.OnStop()
	s.listener.Close()
}

func (s *SocketServer) acceptConnectionsRoutine() {
	// semaphore := make(chan struct{}, maxNumberConnections)

	for {
		// semaphore <- struct{}{}

		// Accept a connection
		fmt.Println("Waiting for new connection...")
		conn, err := s.listener.Accept()
		if err != nil {
			if !s.IsRunning() {
				return // Ignore error from listener closing.
			}
			Exit("Failed to accept connection: " + err.Error())
		} else {
			fmt.Println("Accepted a new connection")
		}

		closeConn := make(chan error, 2)              // Push to signal connection closed
		responses := make(chan *types.Response, 1000) // A channel to buffer responses

		// Read requests from conn and deal with them
		go s.handleRequests(closeConn, conn, responses)
		// Pull responses from 'responses' and write them to conn.
		go s.handleResponses(closeConn, responses, conn)

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
}

// Read requests from conn and deal with them
func (s *SocketServer) handleRequests(closeConn chan error, conn net.Conn, responses chan<- *types.Response) {
	var count int
	var bufReader = bufio.NewReader(conn)
	for {

		var req = &types.Request{}
		err := types.ReadMessage(bufReader, req)
		if err != nil {
			if err == io.EOF {
				closeConn <- fmt.Errorf("Connection closed by client")
			} else {
				closeConn <- fmt.Errorf("Error in handleValue: %v", err.Error())
			}
			return
		}
		s.appMtx.Lock()
		count++
		s.handleRequest(req, responses)
		s.appMtx.Unlock()
	}
}

func (s *SocketServer) handleRequest(req *types.Request, responses chan<- *types.Response) {
	switch r := req.Value.(type) {
	case *types.Request_Echo:
		responses <- types.ToResponseEcho(r.Echo.Message)
	case *types.Request_Flush:
		responses <- types.ToResponseFlush()
	case *types.Request_Info:
		data := s.app.Info()
		responses <- types.ToResponseInfo(data)
	case *types.Request_SetOption:
		so := r.SetOption
		logStr := s.app.SetOption(so.Key, so.Value)
		responses <- types.ToResponseSetOption(logStr)
	case *types.Request_AppendTx:
		res := s.app.AppendTx(r.AppendTx.Tx)
		responses <- types.ToResponseAppendTx(res.Code, res.Data, res.Log)
	case *types.Request_CheckTx:
		res := s.app.CheckTx(r.CheckTx.Tx)
		responses <- types.ToResponseCheckTx(res.Code, res.Data, res.Log)
	case *types.Request_Commit:
		res := s.app.Commit()
		responses <- types.ToResponseCommit(res.Code, res.Data, res.Log)
	case *types.Request_Query:
		res := s.app.Query(r.Query.Query)
		responses <- types.ToResponseQuery(res.Code, res.Data, res.Log)
	case *types.Request_InitChain:
		if app, ok := s.app.(types.BlockchainAware); ok {
			app.InitChain(r.InitChain.Validators)
			responses <- types.ToResponseInitChain()
		} else {
			responses <- types.ToResponseInitChain()
		}
	case *types.Request_EndBlock:
		if app, ok := s.app.(types.BlockchainAware); ok {
			validators := app.EndBlock(r.EndBlock.Height)
			responses <- types.ToResponseEndBlock(validators)
		} else {
			responses <- types.ToResponseEndBlock(nil)
		}
	default:
		responses <- types.ToResponseException("Unknown request")
	}
}

// Pull responses from 'responses' and write them to conn.
func (s *SocketServer) handleResponses(closeConn chan error, responses <-chan *types.Response, conn net.Conn) {
	var count int
	var bufWriter = bufio.NewWriter(conn)
	for {
		var res = <-responses
		err := types.WriteMessage(res, bufWriter)
		if err != nil {
			closeConn <- fmt.Errorf("Error in handleValue: %v", err.Error())
			return
		}
		if _, ok := res.Value.(*types.Response_Flush); ok {
			err = bufWriter.Flush()
			if err != nil {
				closeConn <- fmt.Errorf("Error in handleValue: %v", err.Error())
				return
			}
		}
		count++
	}
}
