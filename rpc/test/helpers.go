package rpctest

import (
	"fmt"

	logger "github.com/tendermint/go-logger"

	abci "github.com/tendermint/abci/types"
	cfg "github.com/tendermint/go-config"
	client "github.com/tendermint/go-rpc/client"
	"github.com/tendermint/tendermint/config/tendermint_test"
	nm "github.com/tendermint/tendermint/node"
	"github.com/tendermint/tendermint/proxy"
	rpcclient "github.com/tendermint/tendermint/rpc/client"
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

// GetClient gets a rpc client pointing to the test tendermint rpc
func GetClient() *rpcclient.HTTPClient {
	rpcAddr := GetConfig().GetString("rpc_laddr")
	return rpcclient.New(rpcAddr, "/websocket")
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
