package commands

import (
	"github.com/spf13/cobra"

	"github.com/tendermint/tendermint/types"
	pvm "github.com/tendermint/tendermint/types/priv_validator"
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
	var pv *pvm.FilePV
	if cmn.FileExists(privValFile) {
		pv = pvm.LoadFilePV(privValFile)
		logger.Info("Found private validator", "path", privValFile)
	} else {
		pv = pvm.GenFilePV(privValFile)
		pv.Save()
		logger.Info("Generated private validator", "path", privValFile)
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
			PubKey: pv.GetPubKey(),
			Power:  10,
		}}

		if err := genDoc.SaveAs(genFile); err != nil {
			panic(err)
		}
		logger.Info("Generated genesis file", "path", genFile)
	}
}
