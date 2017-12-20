package commands

import (
	"github.com/spf13/cobra"

	"github.com/tendermint/tendermint/types"
	cmn "github.com/tendermint/tmlibs/common"
)

// InitFilesCmd initialises a fresh Tendermint Core instance.
var InitFilesCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize Tendermint",
	Run:   initFiles,
}

func initFiles(cmd *cobra.Command, args []string) {
	privValFile := config.PrivValidatorFile()
	var privValidator *types.PrivValidatorFS
	if cmn.FileExists(privValFile) {
		privValidator = types.LoadPrivValidatorFS(privValFile)
		logger.Info("Already initialized", "priv_validator", privValFile)
	} else {
		privValidator = types.GenPrivValidatorFS(privValFile)
		privValidator.Save()
		logger.Info("Initialized tendermint", "privValidator", privValFile)
	}

	genFile := config.GenesisFile()
	if cmn.FileExists(genFile) {
		logger.Info("Already initialized", "geneisis", genFile)
	} else {
		genDoc := types.GenesisDoc{
			ChainID: cmn.Fmt("test-chain-%v", cmn.RandStr(6)),
		}
		genDoc.Validators = []types.GenesisValidator{{
			PubKey: privValidator.GetPubKey(),
			Power:  10,
		}}

		if err := genDoc.SaveAs(genFile); err != nil {
			panic(err)
		}
		logger.Info("Initialized tendermint", "genesis", genFile)
	}
}
