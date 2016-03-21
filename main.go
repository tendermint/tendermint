package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"

	"github.com/tendermint/netmon/handlers"
	"github.com/tendermint/netmon/types"

	"github.com/codegangsta/cli"
	. "github.com/tendermint/go-common"
	cfg "github.com/tendermint/go-config"
	pcm "github.com/tendermint/go-process"
	"github.com/tendermint/go-rpc/server"
	"github.com/tendermint/go-wire"
	tmcfg "github.com/tendermint/tendermint/config/tendermint"
)

func init() {

	config := tmcfg.GetConfig("")
	config.Set("log_level", "debug")
	cfg.ApplyConfig(config) // Notify modules of new config
}

func main() {
	app := cli.NewApp()
	app.Name = "netmon"
	app.Usage = "netmon [command] [args...]"
	app.Commands = []cli.Command{
		{
			Name:      "config",
			Usage:     "Create a config from a mintnet testnet",
			ArgsUsage: "[chainID] [prefix] [N]",
			Action: func(c *cli.Context) {
				cmdConfig(c)
			},
		},
		{
			Name:      "chains-and-vals",
			Usage:     "Add a chain or validator set to the main config file",
			ArgsUsage: "",
			Action: func(c *cli.Context) {
				cmdChainsAndVals(c)
			},
			Subcommands: []cli.Command{
				{
					Name:      "chain",
					Usage:     "Add a chain to the main config file",
					ArgsUsage: "[configFile] [chainBaseDir]",
					Action: func(c *cli.Context) {
						cmdAddChain(c)
					},
				},
				{
					Name:      "val",
					Usage:     "Add a validator set to the main config file",
					ArgsUsage: "[configFile] [valsetBaseDir]",
					Action: func(c *cli.Context) {
						cmdAddValSet(c)
					},
				},
			},
		},
		{
			Name:      "monitor",
			Usage:     "Monitor a chain",
			ArgsUsage: "[config file]",
			Action: func(c *cli.Context) {
				cmdMonitor(c)
			},
		},
		{
			Name:      "bench",
			Usage:     "Benchmark a chain's tx throughput and latency",
			ArgsUsage: "[config file] [results dir] [n txs] -- [command to fire n txs]",
			Action: func(c *cli.Context) {
				cmdBench(c)
			},
			Flags: []cli.Flag{
				cli.IntFlag{
					Name:  "n_txs",
					Value: 0,
					Usage: "run benchmark until this many txs have been committed",
				},
				cli.IntFlag{
					Name:  "n_blocks",
					Value: 0,
					Usage: "run benchmark until this many blocks have been committed",
				},
			},
		},
	}
	app.Run(os.Args)
}

func cmdChainsAndVals(c *cli.Context) {
	cli.ShowAppHelp(c)
}

func cmdAddChain(c *cli.Context) {
	args := c.Args()
	if len(args) != 2 {
		Exit("add chain expectes 2 arg")
	}
	cfgFile, chainDir := args[0], args[1]

	// load major config
	chainsAndVals := new(ChainsAndValidators)
	if err := ReadJSONFile(chainsAndVals, cfgFile); err != nil {
		Exit(err.Error())
	}

	// load new chain
	chainCfg_ := new(BlockchainConfig)
	if err := ReadJSONFile(chainCfg_, path.Join(chainDir, "chain_config.json")); err != nil {
		Exit(err.Error())
	}
	chainCfg := convertMintnetBlockchain(chainCfg_)

	// append new chain
	chainsAndVals.Blockchains = append(chainsAndVals.Blockchains, chainCfg)

	// write major config
	b := wire.JSONBytes(chainsAndVals)
	if err := ioutil.WriteFile(cfgFile, b, 0600); err != nil {
		Exit(err.Error())
	}
}

func ReadJSONFile(o interface{}, filename string) error {
	b, err := ioutil.ReadFile(filename)
	if err != nil {
		return err
	}
	wire.ReadJSON(o, b, &err)
	if err != nil {
		return err
	}
	return nil
}

func cmdAddValSet(c *cli.Context) {
	args := c.Args()
	if len(args) != 2 {
		Exit("add chain expectes 2 arg")
	}
	cfgFile, valSetDir := args[0], args[1]

	// load major config
	chainsAndVals := new(ChainsAndValidators)
	if err := ReadJSONFile(chainsAndVals, cfgFile); err != nil {
		Exit(err.Error())
	}

	// load new validator set
	valSet := new(types.ValidatorSet)
	if err := ReadJSONFile(valSet, path.Join(valSetDir, "validator_set.json")); err != nil {
		Exit(err.Error())
	}

	// append new validator set
	chainsAndVals.ValidatorSets = append(chainsAndVals.ValidatorSets, valSet)

	// write major config to file
	b := wire.JSONBytes(chainsAndVals)
	if err := ioutil.WriteFile(cfgFile, b, 0600); err != nil {
		Exit(err.Error())
	}

}

func cmdMonitor(c *cli.Context) {
	args := c.Args()
	if len(args) != 1 {
		Exit("monitor expectes 1 arg")
	}
	chainsAndValsFile := args[0]
	chainsAndVals, err := LoadChainsAndValsFromFile(chainsAndValsFile)
	if err != nil {
		Exit(err.Error())
	}

	network := registerNetwork(chainsAndVals)
	startRPC(network)

	TrapSignal(func() {
		network.Stop()
	})
}

func cmdBench(c *cli.Context) {
	args := c.Args()
	if len(args) < 2 {
		Exit("bench expects at least 2 args")
	}
	chainsAndValsFile := args[0]
	resultsDir := args[1]
	// extra args are a program to run locally
	if len(args) > 2 {
		args = args[2:]
	} else {
		args = args[:0]
	}

	chainsAndVals, err := LoadChainsAndValsFromFile(chainsAndValsFile)
	if err != nil {
		Exit(err.Error())
	}

	network := registerNetwork(chainsAndVals)
	startRPC(network)

	// benchmark txs
	done := make(chan *types.BenchmarkResults)

	// we should only have one chain for a benchmark run
	chAndValIDs, _ := network.Status()
	chain, _ := network.GetChain(chAndValIDs.ChainIDs[0])

	// setup benchresults struct and fire txs
	if nTxs := c.Int("n_txs"); nTxs != 0 {
		chain.Status.BenchmarkTxs(done, nTxs, args)
	} else if nBlocks := c.Int("n_blocks"); nBlocks != 0 {
		chain.Status.BenchmarkBlocks(done, nBlocks, args)
	} else {
		Exit("Must specify one of n_txs or n_blocks")
	}
	results := <-done

	b, err := json.Marshal(results)
	if err != nil {
		Exit(err.Error())
	}
	fmt.Println(string(b))
	if err := ioutil.WriteFile(path.Join(resultsDir, "netmon.log"), b, 0600); err != nil {
		Exit(err.Error())
	}
	finalResults := fmt.Sprintf("%f,%f\n", results.MeanLatency, results.MeanThroughput)
	if err := ioutil.WriteFile(path.Join(resultsDir, "final_results"), []byte(finalResults), 0600); err != nil {
		Exit(err.Error())
	}
}

func registerNetwork(chainsAndVals *ChainsAndValidators) *handlers.TendermintNetwork {
	// the main object that watches for changes and serves the rpc requests
	network := handlers.NewTendermintNetwork()

	for _, valSetCfg := range chainsAndVals.ValidatorSets {
		// Register validator set
		_, err := network.RegisterValidatorSet(valSetCfg)
		if err != nil {
			Exit("Register validator set error: " + err.Error())
		}
	}

	for _, chainCfg := range chainsAndVals.Blockchains {
		// Register blockchain
		_, err := network.RegisterChain(chainCfg)
		if err != nil {
			Exit(Fmt("Register chain error for chain %s: %v", chainCfg.ID, err))
		}
	}

	return network
}

func startRPC(network *handlers.TendermintNetwork) {
	// the routes are functions on the network object
	routes := handlers.Routes(network)

	// serve http and ws
	mux := http.NewServeMux()
	wm := rpcserver.NewWebsocketManager(routes, nil) // TODO: evsw
	mux.HandleFunc("/websocket", wm.WebsocketHandler)
	rpcserver.RegisterRPCFuncs(mux, routes)
	if _, err := rpcserver.StartHTTPServer("0.0.0.0:46670", mux); err != nil {
		Exit(err.Error())
	}

}

func cmdConfig(c *cli.Context) {
	args := c.Args()
	if len(args) != 3 {
		Exit("config expects 3 args")
	}
	id, prefix := args[0], args[1]
	n, err := strconv.Atoi(args[2])
	if err != nil {
		Exit(err.Error())
	}
	chain, err := ConfigFromMachines(id, prefix, n)
	if err != nil {
		Exit(err.Error())
	}

	b, err := json.Marshal(chain)
	if err != nil {
		Exit(err.Error())
	}
	fmt.Println(string(b))
}

func ConfigFromMachines(chainID, prefix string, N int) (*types.BlockchainConfig, error) {

	chain := &types.BlockchainConfig{
		ID:         chainID,
		Validators: make([]*types.ValidatorState, N),
	}
	for i := 0; i < N; i++ {
		id := fmt.Sprintf("%s%d", prefix, i+1)
		ip, success := runProcessGetResult(id+"-ip", "docker-machine", []string{"ip", id})
		if !success {
			return nil, fmt.Errorf(ip)
		}

		val := &types.Validator{
			ID: id,
			// TODO: pubkey
		}
		chainVal := &types.ValidatorState{
			Config: &types.ValidatorConfig{
				Validator: val,
				RPCAddr:   fmt.Sprintf("%s:%d", strings.Trim(ip, "\n"), 46657),
				Index:     i,
			},
		}
		chain.Validators[i] = chainVal
	}
	return chain, nil
}

func runProcessGetResult(label string, command string, args []string) (string, bool) {
	outFile := NewBufferCloser(nil)
	fmt.Println(Green(command), Green(args))
	proc, err := pcm.StartProcess(label, command, args, nil, outFile)
	if err != nil {
		return "", false
	}

	<-proc.WaitCh
	if proc.ExitState.Success() {
		fmt.Println(Blue(string(outFile.Bytes())))
		return string(outFile.Bytes()), true
	} else {
		// Error!
		fmt.Println(Red(string(outFile.Bytes())))
		return string(outFile.Bytes()), false
	}
}

//----------------------------------------------------------------------

type ChainsAndValidators struct {
	ValidatorSets []*types.ValidatorSet     `json:"validator_sets"`
	Blockchains   []*types.BlockchainConfig `json:"blockchains"`
}

func LoadChainsAndValsFromFile(configFile string) (*ChainsAndValidators, error) {

	b, err := ioutil.ReadFile(configFile)
	if err != nil {
		return nil, err
	}

	chainsAndVals_ := new(ChainsAndValidators)
	wire.ReadJSON(chainsAndVals_, b, &err)
	if err != nil {
		return nil, err
	}

	return chainsAndVals_, nil
}

// because types are duplicated in mintnet
type BlockchainConfig struct {
	ID         string                   `json:"id"`
	ValSetID   string                   `json:"val_set_id"`
	Validators []*types.ValidatorConfig `json:"validators"`
}

func convertMintnetBlockchain(b *BlockchainConfig) *types.BlockchainConfig {
	vals := make([]*types.ValidatorState, len(b.Validators))
	for j, v := range b.Validators {
		vals[j] = new(types.ValidatorState)
		vals[j].Config = v
	}
	return &types.BlockchainConfig{
		ID:         b.ID,
		ValSetID:   b.ValSetID,
		Validators: vals,
	}

}
