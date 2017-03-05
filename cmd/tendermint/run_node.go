package main

import (
	"io/ioutil"
	"time"

	. "github.com/tendermint/go-common"
	cfg "github.com/tendermint/go-config"
	"github.com/tendermint/tendermint/node"
	"github.com/tendermint/tendermint/types"
)

// Users wishing to:
//	* Use an external signer for their validators
//	* Supply an in-proc abci app
// should import github.com/tendermint/tendermint/node and implement
// their own run_node to call node.NewNode (instead of node.NewNodeDefault)
// with their custom priv validator and/or custom proxy.ClientCreator
func run_node(config cfg.Config) {

	// Wait until the genesis doc becomes available
	// This is for Mintnet compatibility.
	// TODO: If Mintnet gets deprecated or genesis_file is
	// always available, remove.
	genDocFile := config.GetString("genesis_file")
	if !FileExists(genDocFile) {
		log.Notice(Fmt("Waiting for genesis file %v...", genDocFile))
		for {
			time.Sleep(time.Second)
			if !FileExists(genDocFile) {
				continue
			}
			jsonBlob, err := ioutil.ReadFile(genDocFile)
			if err != nil {
				Exit(Fmt("Couldn't read GenesisDoc file: %v", err))
			}
			genDoc, err := types.GenesisDocFromJSON(jsonBlob)
			if err != nil {
				Exit(Fmt("Error reading GenesisDoc: %v", err))
			}
			if genDoc.ChainID == "" {
				Exit(Fmt("Genesis doc %v must include non-empty chain_id", genDocFile))
			}
			config.Set("chain_id", genDoc.ChainID)
		}
	}

	// Create & start node
	n := node.NewNodeDefault(config)
	if _, err := n.Start(); err != nil {
		Exit(Fmt("Failed to start node: %v", err))
	} else {
		log.Notice("Started node", "nodeInfo", n.Switch().NodeInfo())
	}

	// Trap signal, run forever.
	n.RunForever()

}
