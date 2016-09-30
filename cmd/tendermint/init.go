package main

import (
	. "github.com/tendermint/go-common"
	"github.com/tendermint/tendermint/types"
)

func init_files() {
	privValidator := types.GenPrivValidator()
	privValidator.SetFile(config.GetString("priv_validator_file"))
	privValidator.Save()

	genDoc := types.GenesisDoc{
		ChainID: Fmt("test-chain-%v", RandStr(6)),
	}
	genDoc.Validators = []types.GenesisValidator{types.GenesisValidator{
		PubKey: privValidator.PubKey,
		Amount: 10,
	}}

	genDoc.SaveAs(config.GetString("genesis_file"))

	log.Notice("Initialized tendermint", "genesis", config.GetString("genesis_file"), "priv_validator", config.GetString("priv_validator_file"))
}
