package rpctest

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	logger "github.com/tendermint/go-logger"
	wire "github.com/tendermint/go-wire"

	abci "github.com/tendermint/abci/types"
	cfg "github.com/tendermint/go-config"
	client "github.com/tendermint/go-rpc/client"
	"github.com/tendermint/tendermint/config/tendermint_test"
	nm "github.com/tendermint/tendermint/node"
	"github.com/tendermint/tendermint/proxy"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
	core_grpc "github.com/tendermint/tendermint/rpc/grpc"
	"github.com/tendermint/tendermint/types"
)

var (
	config cfg.Config
	node   *nm.Node
)

const tmLogLevel = "error"

// GetConfig returns a config for the test cases as a singleton
func GetConfig() cfg.Config {
	if config == nil {
		config = tendermint_test.ResetConfig("rpc_test_client_test")
		// Shut up the logging
		logger.SetLogLevel(tmLogLevel)
	}
	return config
}

func GetNode() *nm.Node {
	return node
}

// GetURIClient gets a uri client pointing to the test tendermint rpc
func GetURIClient() *client.ClientURI {
	rpcAddr := GetConfig().GetString("rpc_laddr")
	return client.NewClientURI(rpcAddr)
}

// GetJSONClient gets a http/json client pointing to the test tendermint rpc
func GetJSONClient() *client.ClientJSONRPC {
	rpcAddr := GetConfig().GetString("rpc_laddr")
	return client.NewClientJSONRPC(rpcAddr)
}

func GetGRPCClient() core_grpc.BroadcastAPIClient {
	grpcAddr := config.GetString("grpc_laddr")
	return core_grpc.StartGRPCClient(grpcAddr)
}

func GetWSClient() *client.WSClient {
	rpcAddr := GetConfig().GetString("rpc_laddr")
	wsc := client.NewWSClient(rpcAddr, "/websocket")
	if _, err := wsc.Start(); err != nil {
		panic(err)
	}
	return wsc
}

// StartTendermint starts a test tendermint server in a go routine and returns when it is initialized
// TODO: can one pass an Application in????
func StartTendermint(app abci.Application) *nm.Node {
	// start a node
	fmt.Println("Starting Tendermint...")

	node = NewTendermint(app)
	fmt.Println("Tendermint running!")
	return node
}

// NewTendermint creates a new tendermint server and sleeps forever
func NewTendermint(app abci.Application) *nm.Node {
	// Create & start node
	config := GetConfig()
	privValidatorFile := config.GetString("priv_validator_file")
	privValidator := types.LoadOrGenPrivValidator(privValidatorFile)
	papp := proxy.NewLocalClientCreator(app)
	node := nm.NewNode(config, privValidator, papp)

	// node.Start now does everything including the RPC server
	node.Start()
	return node
}

//--------------------------------------------------------------------------------
// Utilities for testing the websocket service

// wait for an event; do things that might trigger events, and check them when they are received
// the check function takes an event id and the byte slice read off the ws
func waitForEvent(t *testing.T, wsc *client.WSClient, eventid string, dieOnTimeout bool, f func(), check func(string, interface{}) error) {
	// go routine to wait for webscoket msg
	goodCh := make(chan interface{})
	errCh := make(chan error)

	// Read message
	go func() {
		var err error
	LOOP:
		for {
			select {
			case r := <-wsc.ResultsCh:
				result := new(ctypes.TMResult)
				wire.ReadJSONPtr(result, r, &err)
				if err != nil {
					errCh <- err
					break LOOP
				}
				event, ok := (*result).(*ctypes.ResultEvent)
				if ok && event.Name == eventid {
					goodCh <- event.Data
					break LOOP
				}
			case err := <-wsc.ErrorsCh:
				errCh <- err
				break LOOP
			case <-wsc.Quit:
				break LOOP
			}
		}
	}()

	// do stuff (transactions)
	f()

	// wait for an event or timeout
	timeout := time.NewTimer(10 * time.Second)
	select {
	case <-timeout.C:
		if dieOnTimeout {
			wsc.Stop()
			require.True(t, false, "%s event was not received in time", eventid)
		}
		// else that's great, we didn't hear the event
		// and we shouldn't have
	case eventData := <-goodCh:
		if dieOnTimeout {
			// message was received and expected
			// run the check
			require.Nil(t, check(eventid, eventData))
		} else {
			wsc.Stop()
			require.True(t, false, "%s event was not expected", eventid)
		}
	case err := <-errCh:
		panic(err) // Show the stack trace.
	}
}

//--------------------------------------------------------------------------------
