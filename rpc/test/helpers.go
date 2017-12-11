package rpctest

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"

	"github.com/tendermint/tmlibs/log"

	abci "github.com/tendermint/abci/types"
	cfg "github.com/tendermint/tendermint/config"
	nm "github.com/tendermint/tendermint/node"
	"github.com/tendermint/tendermint/proxy"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
	core_grpc "github.com/tendermint/tendermint/rpc/grpc"
	rpcclient "github.com/tendermint/tendermint/rpc/lib/client"
	"github.com/tendermint/tendermint/types"
)

var globalConfig *cfg.Config

func waitForRPC() {
	laddr := GetConfig().RPC.ListenAddress
	client := rpcclient.NewJSONRPCClient(laddr)
	result := new(ctypes.ResultStatus)
	for {
		_, err := client.Call("status", map[string]interface{}{}, result)
		if err == nil {
			return
		}
	}
}

func waitForGRPC() {
	client := GetGRPCClient()
	for {
		_, err := client.Ping(context.Background(), &core_grpc.RequestPing{})
		if err == nil {
			return
		}
	}
}

// f**ing long, but unique for each test
func makePathname() string {
	// get path
	p, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	// fmt.Println(p)
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
func GetConfig() *cfg.Config {
	if globalConfig == nil {
		pathname := makePathname()
		globalConfig = cfg.ResetTestRoot(pathname)

		// and we use random ports to run in parallel
		tm, rpc, grpc := makeAddrs()
		globalConfig.P2P.ListenAddress = tm
		globalConfig.RPC.ListenAddress = rpc
		globalConfig.RPC.GRPCListenAddress = grpc
		globalConfig.TxIndex.IndexTags = "app.creator" // see dummy application
	}
	return globalConfig
}

func GetGRPCClient() core_grpc.BroadcastAPIClient {
	grpcAddr := globalConfig.RPC.GRPCListenAddress
	return core_grpc.StartGRPCClient(grpcAddr)
}

// StartTendermint starts a test tendermint server in a go routine and returns when it is initialized
func StartTendermint(app abci.Application) *nm.Node {
	node := NewTendermint(app)
	err := node.Start()
	if err != nil {
		panic(err)
	}

	// wait for rpc
	waitForRPC()
	waitForGRPC()

	fmt.Println("Tendermint running!")

	return node
}

// NewTendermint creates a new tendermint server and sleeps forever
func NewTendermint(app abci.Application) *nm.Node {
	// Create & start node
	config := GetConfig()
	logger := log.NewTMLogger(log.NewSyncWriter(os.Stdout))
	logger = log.NewFilter(logger, log.AllowError())
	privValidatorFile := config.PrivValidatorFile()
	privValidator := types.LoadOrGenPrivValidatorFS(privValidatorFile)
	papp := proxy.NewLocalClientCreator(app)
	node, err := nm.NewNode(config, privValidator, papp,
		nm.DefaultGenesisDocProviderFunc(config),
		nm.DefaultDBProvider, logger)
	if err != nil {
		panic(err)
	}
	return node
}
