package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"github.com/tendermint/go-amino"
	crypto "github.com/tendermint/tendermint/crypto"
	cmn "github.com/tendermint/tendermint/libs/common"

	"github.com/tendermint/tendermint/p2p"
	"github.com/tendermint/tendermint/privval"
	"github.com/tendermint/tendermint/types"
)

type GenesisValidator struct {
	PubKey Data   `json:"pub_key"`
	Power  int64  `json:"power"`
	Name   string `json:"name"`
}

type Genesis struct {
	GenesisTime     time.Time              `json:"genesis_time"`
	ChainID         string                 `json:"chain_id"`
	ConsensusParams *types.ConsensusParams `json:"consensus_params,omitempty"`
	Validators      []GenesisValidator     `json:"validators"`
	AppHash         cmn.HexBytes           `json:"app_hash"`
	AppStateJSON    json.RawMessage        `json:"app_state,omitempty"`
	AppOptions      json.RawMessage        `json:"app_options,omitempty"` // DEPRECATED

}

type NodeKey struct {
	PrivKey Data `json:"priv_key"`
}

type PrivVal struct {
	Address    cmn.HexBytes `json:"address"`
	LastHeight int64        `json:"last_height"`
	LastRound  int          `json:"last_round"`
	LastStep   int8         `json:"last_step"`
	PubKey     Data         `json:"pub_key"`
	PrivKey    Data         `json:"priv_key"`
}

type Data struct {
	Type string       `json:"type"`
	Data cmn.HexBytes `json:"data"`
}

func convertNodeKey(cdc *amino.Codec, jsonBytes []byte) ([]byte, error) {
	var nodeKey NodeKey
	err := json.Unmarshal(jsonBytes, &nodeKey)
	if err != nil {
		return nil, err
	}

	var privKey crypto.PrivKeyEd25519
	copy(privKey[:], nodeKey.PrivKey.Data)

	nodeKeyNew := p2p.NodeKey{privKey}

	bz, err := cdc.MarshalJSON(nodeKeyNew)
	if err != nil {
		return nil, err
	}
	return bz, nil
}

func convertPrivVal(cdc *amino.Codec, jsonBytes []byte) ([]byte, error) {
	var privVal PrivVal
	err := json.Unmarshal(jsonBytes, &privVal)
	if err != nil {
		return nil, err
	}

	var privKey crypto.PrivKeyEd25519
	copy(privKey[:], privVal.PrivKey.Data)

	var pubKey crypto.PubKeyEd25519
	copy(pubKey[:], privVal.PubKey.Data)

	privValNew := privval.FilePV{
		Address:    pubKey.Address(),
		PubKey:     pubKey,
		LastHeight: privVal.LastHeight,
		LastRound:  privVal.LastRound,
		LastStep:   privVal.LastStep,
		PrivKey:    privKey,
	}

	bz, err := cdc.MarshalJSON(privValNew)
	if err != nil {
		return nil, err
	}
	return bz, nil
}

func convertGenesis(cdc *amino.Codec, jsonBytes []byte) ([]byte, error) {
	var genesis Genesis
	err := json.Unmarshal(jsonBytes, &genesis)
	if err != nil {
		return nil, err
	}

	genesisNew := types.GenesisDoc{
		GenesisTime:     genesis.GenesisTime,
		ChainID:         genesis.ChainID,
		ConsensusParams: genesis.ConsensusParams,
		// Validators
		AppHash:      genesis.AppHash,
		AppStateJSON: genesis.AppStateJSON,
	}

	if genesis.AppOptions != nil {
		genesisNew.AppStateJSON = genesis.AppOptions
	}

	for _, v := range genesis.Validators {
		var pubKey crypto.PubKeyEd25519
		copy(pubKey[:], v.PubKey.Data)
		genesisNew.Validators = append(
			genesisNew.Validators,
			types.GenesisValidator{
				PubKey: pubKey,
				Power:  v.Power,
				Name:   v.Name,
			},
		)

	}

	bz, err := cdc.MarshalJSON(genesisNew)
	if err != nil {
		return nil, err
	}
	return bz, nil
}

func main() {
	cdc := amino.NewCodec()
	crypto.RegisterAmino(cdc)

	args := os.Args[1:]
	if len(args) != 1 {
		fmt.Println("Please specify a file to convert")
		os.Exit(1)
	}

	filePath := args[0]
	fileName := filepath.Base(filePath)

	fileBytes, err := ioutil.ReadFile(filePath)
	if err != nil {
		panic(err)
	}

	var bz []byte

	switch fileName {
	case "node_key.json":
		bz, err = convertNodeKey(cdc, fileBytes)
	case "priv_validator.json":
		bz, err = convertPrivVal(cdc, fileBytes)
	case "genesis.json":
		bz, err = convertGenesis(cdc, fileBytes)
	default:
		fmt.Println("Expected file name to be in (node_key.json, priv_validator.json, genesis.json)")
		os.Exit(1)
	}

	if err != nil {
		panic(err)
	}
	fmt.Println(string(bz))

}
