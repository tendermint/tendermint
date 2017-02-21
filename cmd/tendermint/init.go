package main

import (
	"os"

	cmn "github.com/tendermint/go-common"
	"github.com/tendermint/tendermint/types"
)

func init_files() {
	privValFile := config.GetString("priv_validator_file")
	if _, err := os.Stat(privValFile); os.IsNotExist(err) {
		privValidator := types.GenPrivValidator()
		privValidator.SetFile(privValFile)
		privValidator.Save()

		genFile := config.GetString("genesis_file")

		if _, err := os.Stat(genFile); os.IsNotExist(err) {
			genDoc := types.GenesisDoc{
				ChainID: cmn.Fmt("test-chain-%v", cmn.RandStr(6)),
			}
			genDoc.Validators = []types.GenesisValidator{types.GenesisValidator{
				PubKey: privValidator.PubKey,
				Amount: 10,
			}}

			genDoc.SaveAs(genFile)
		}

		log.Notice("Initialized tendermint", "genesis", config.GetString("genesis_file"), "priv_validator", config.GetString("priv_validator_file"))
	} else {
		log.Notice("Already initialized", "priv_validator", config.GetString("priv_validator_file"))
	}
}
