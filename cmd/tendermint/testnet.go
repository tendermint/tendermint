package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path"
	"strconv"
	"time"

	cmn "github.com/tendermint/go-common"
	"github.com/tendermint/go-crypto"
	"github.com/tendermint/go-wire"
	"github.com/tendermint/tendermint/types"
)

func testnet(nString, baseDir string) {

	N, err := strconv.Atoi(nString)
	if err != nil {
		fmt.Println("expected int, got", nString)
		cmn.Exit(err.Error())
	}

	genVals := make([]types.GenesisValidator, N)

	// Initialize core dir and priv_validator.json's
	for i := 0; i < N; i++ {
		mach := cmn.Fmt("mach%d", i)
		err := initMachCoreDirectory(baseDir, mach)
		if err != nil {
			cmn.Exit(err.Error())
		}
		// Read priv_validator.json to populate vals
		privValFile := path.Join(baseDir, mach, "priv_validator.json")
		privVal := types.LoadPrivValidator(privValFile)
		genVals[i] = types.GenesisValidator{
			PubKey: privVal.PubKey,
			Amount: 1,
			Name:   mach,
		}
	}

	// Generate genesis doc from generated validators
	genDoc := &types.GenesisDoc{
		GenesisTime: time.Now(),
		ChainID:     "chain-" + cmn.RandStr(6),
		Validators:  genVals,
	}

	// Write genesis file.
	for i := 0; i < N; i++ {
		mach := cmn.Fmt("mach%d", i)
		genDoc.SaveAs(path.Join(baseDir, mach, "genesis.json"))
	}

	// write the chain meta data (ie. validator set name and validators)
	blockchainCfg := &BlockchainInfo{
		Validators: make([]*CoreInfo, len(genVals)),
	}

	for i, v := range genVals {
		blockchainCfg.Validators[i] = &CoreInfo{
			Validator: &Validator{ID: v.Name, PubKey: v.PubKey},
			Index:     i, // XXX: we may want more control here
		}
	}
	err = WriteBlockchainInfo(baseDir, blockchainCfg)
	if err != nil {
		cmn.Exit(err.Error())
	}

	fmt.Println(cmn.Fmt("Successfully initialized %v node directories", N))
}

// Initialize per-machine core directory
func initMachCoreDirectory(base, mach string) error {
	dir := path.Join(base, mach)
	err := cmn.EnsureDir(dir, 0777)
	if err != nil {
		return err
	}

	// Create priv_validator.json file if not present
	ensurePrivValidator(path.Join(dir, "priv_validator.json"))
	return nil

}

func ensurePrivValidator(file string) {
	if cmn.FileExists(file) {
		return
	}
	privValidator := types.GenPrivValidator()
	privValidator.SetFile(file)
	privValidator.Save()
}

//--------------------
// For netmon

func WriteBlockchainInfo(base string, chainCfg *BlockchainInfo) error {
	b := wire.JSONBytes(chainCfg)
	var buf bytes.Buffer
	json.Indent(&buf, b, "", "\t")
	return ioutil.WriteFile(path.Join(base, "chain_config.json"), buf.Bytes(), 0600)
}

// validator (independent of chain)
type Validator struct {
	ID     string        `json:"id"`
	PubKey crypto.PubKey `json:"pub_key"`
	// Chains []string      `json:"chains,omitempty"`
}

// validator set (independent of chains)
type ValidatorSet struct {
	ID         string       `json:"id"`
	Validators []*Validator `json:"validators"`
}

// validator on a chain
type CoreInfo struct {
	Validator *Validator `json:"validator"`
	P2PAddr   string     `json:"p2p_addr"`
	RPCAddr   string     `json:"rpc_addr"`
	Index     int        `json:"index,omitempty"`
}

type BlockchainInfo struct {
	ID         string      `json:"id"`
	ValSetID   string      `json:"val_set_id"`
	Validators []*CoreInfo `json:"validators"`
}
