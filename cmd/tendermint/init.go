package main

import (
	"github.com/tendermint/tendermint/types"
)

func init_files() {
	privValidator := types.GenPrivValidator()
	privValidator.SetFile(config.GetString("priv_validator_file"))
	privValidator.Save()

	//TODO: chainID
	genDoc := types.GenesisDoc{
		ChainID: "hi",
	}
	genDoc.Validators = []types.GenesisValidator{types.GenesisValidator{
		PubKey: privValidator.PubKey,
		Amount: 10000,
	}}

	genDoc.SaveAs(config.GetString("genesis_file"))

}
