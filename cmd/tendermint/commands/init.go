package commands

import (
	"github.com/spf13/cobra"

	"github.com/tendermint/tendermint/types"
	priv_val "github.com/tendermint/tendermint/types/priv_validator"
	cmn "github.com/tendermint/tmlibs/common"
)

// InitFilesCmd initialises a fresh Tendermint Core instance.
var InitFilesCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize Tendermint",
	Run:   initFiles,
}

func initFiles(cmd *cobra.Command, args []string) {
	// private validator
	privValFile := config.PrivValidatorFile()
	var privValidator *priv_val.PrivValidatorJSON
	if cmn.FileExists(privValFile) {
		privValidator = priv_val.LoadPrivValidatorJSON(privValFile)
		logger.Info("Found private validator", "path", privValFile)
	} else {
		privValidator = priv_val.GenPrivValidatorJSON(privValFile)
		privValidator.Save()
		logger.Info("Genetated private validator", "path", privValFile)
	}

	// genesis file
	genFile := config.GenesisFile()
	if cmn.FileExists(genFile) {
		logger.Info("Found genesis file", "path", genFile)
	} else {
		genDoc := types.GenesisDoc{
			ChainID: cmn.Fmt("test-chain-%v", cmn.RandStr(6)),
		}
		genDoc.Validators = []types.GenesisValidator{{
			PubKey: privValidator.PubKey(),
			Power:  10,
		}}

		if err := genDoc.SaveAs(genFile); err != nil {
			panic(err)
		}
		logger.Info("Genetated genesis file", "path", genFile)
	}
}
