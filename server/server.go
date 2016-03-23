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

type Server struct {
	QuitService

	proto    string
	addr     string
	listener net.Listener

	appMtx sync.Mutex
	app    types.Application
}

func NewServer(protoAddr string, app types.Application) (*Server, error) {
	parts := strings.SplitN(protoAddr, "://", 2)
	proto, addr := parts[0], parts[1]
	s := &Server{
		proto:    proto,
		addr:     addr,
		listener: nil,
		app:      app,
	}
	s.QuitService = *NewQuitService(nil, "TMSPServer", s)
	_, err := s.Start() // Just start it
	return s, err
}

func (s *Server) OnStart() error {
	s.QuitService.OnStart()
	ln, err := net.Listen(s.proto, s.addr)
	if err != nil {
		return err
	}
	s.listener = ln
	go s.acceptConnectionsRoutine()
	return nil
}

func (s *Server) OnStop() {
	s.QuitService.OnStop()
	s.listener.Close()
}

func (s *Server) acceptConnectionsRoutine() {
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
func (s *Server) handleRequests(closeConn chan error, conn net.Conn, responses chan<- *types.Response) {
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
		s.appMtx.Lock()
		count++
		s.handleRequest(req, responses)
		s.appMtx.Unlock()
	}
}

func (s *Server) handleRequest(req *types.Request, responses chan<- *types.Response) {
	switch req.Type {
	case types.MessageType_Echo:
		responses <- types.ResponseEcho(string(req.Data))
	case types.MessageType_Flush:
		responses <- types.ResponseFlush()
	case types.MessageType_Info:
		data := s.app.Info()
		responses <- types.ResponseInfo(data)
	case types.MessageType_SetOption:
		logStr := s.app.SetOption(req.Key, req.Value)
		responses <- types.ResponseSetOption(logStr)
	case types.MessageType_AppendTx:
		res := s.app.AppendTx(req.Data)
		responses <- types.ResponseAppendTx(res.Code, res.Data, res.Log)
	case types.MessageType_CheckTx:
		res := s.app.CheckTx(req.Data)
		responses <- types.ResponseCheckTx(res.Code, res.Data, res.Log)
	case types.MessageType_Commit:
		res := s.app.Commit()
		responses <- types.ResponseCommit(res.Code, res.Data, res.Log)
	case types.MessageType_Query:
		res := s.app.Query(req.Data)
		responses <- types.ResponseQuery(res.Code, res.Data, res.Log)
	case types.MessageType_InitChain:
		if app, ok := s.app.(types.BlockchainAware); ok {
			app.InitChain(req.Validators)
			responses <- types.ResponseInitChain()
		} else {
			responses <- types.ResponseInitChain()
		}
	case types.MessageType_EndBlock:
		if app, ok := s.app.(types.BlockchainAware); ok {
			validators := app.EndBlock(req.Height)
			responses <- types.ResponseEndBlock(validators)
		} else {
			responses <- types.ResponseEndBlock(nil)
		}
	default:
		responses <- types.ResponseException("Unknown request")
	}
}

// Pull responses from 'responses' and write them to conn.
func (s *Server) handleResponses(closeConn chan error, responses <-chan *types.Response, conn net.Conn) {
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
