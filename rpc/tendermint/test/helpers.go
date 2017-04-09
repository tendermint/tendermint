package rpctest

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
	wire "github.com/tendermint/go-wire"
	logger "github.com/tendermint/tmlibs/logger"

	abci "github.com/tendermint/abci/types"
	cfg "github.com/tendermint/go-config"
	"github.com/tendermint/tendermint/config/tendermint_test"
	nm "github.com/tendermint/tendermint/node"
	"github.com/tendermint/tendermint/proxy"
	client "github.com/tendermint/tendermint/rpc/client"
	ctypes "github.com/tendermint/tendermint/rpc/tendermint/core/types"
	core_grpc "github.com/tendermint/tendermint/rpc/tendermint/grpc"
	"github.com/tendermint/tendermint/types"
)

var (
	config *viper.Viper
)

const tmLogLevel = "error"

// f**ing long, but unique for each test
func makePathname() string {
	// get path
	p, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	fmt.Println(p)
	sep := string(filepath.Separator)
	return strings.Replace(p, sep, "_", -1)
}

func randPort() int {
	// returns between base and base + spread
	base, spread := 20000, 20000
	return base + rand.Intn(spread)
}

func makeAddrs() (string, string, string) {
	start := randPort()
	return fmt.Sprintf("tcp://0.0.0.0:%d", start),
		fmt.Sprintf("tcp://0.0.0.0:%d", start+1),
		fmt.Sprintf("tcp://0.0.0.0:%d", start+2)
}

// GetConfig returns a config for the test cases as a singleton
func GetConfig() *viper.Viper {
	if config == nil {
		pathname := makePathname()
		config = tendermint_test.ResetConfig(pathname)
		// Shut up the logging
		logger.SetLogLevel(tmLogLevel)
		// and we use random ports to run in parallel
		tm, rpc, grpc := makeAddrs()
		config.Set("node_laddr", tm)
		config.Set("rpc_laddr", rpc)
		config.Set("grpc_laddr", grpc)
	}
	return config
}

// GetURIClient gets a uri client pointing to the test tendermint rpc
func GetURIClient() *client.URIClient {
	rpcAddr := GetConfig().GetString("rpc_laddr")
	return client.NewURIClient(rpcAddr)
}

// GetJSONClient gets a http/json client pointing to the test tendermint rpc
func GetJSONClient() *client.JSONRPCClient {
	rpcAddr := GetConfig().GetString("rpc_laddr")
	return client.NewJSONRPCClient(rpcAddr)
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
func StartTendermint(app abci.Application) *nm.Node {
	node := NewTendermint(app)
	node.Start()
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
