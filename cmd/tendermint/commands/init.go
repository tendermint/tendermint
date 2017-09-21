package commands

import (
	"os"

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
	if _, err := os.Stat(privValFile); os.IsNotExist(err) {
		privValidator := types.GenPrivValidatorFS(privValFile)
		privValidator.Save()

		genFile := config.GenesisFile()

		if _, err := os.Stat(genFile); os.IsNotExist(err) {
			genDoc := types.GenesisDoc{
				ChainID: cmn.Fmt("test-chain-%v", cmn.RandStr(6)),
			}
			genDoc.Validators = []types.GenesisValidator{types.GenesisValidator{
				PubKey: privValidator.GetPubKey(),
				Power:  10,
			}}

			genDoc.SaveAs(genFile)
		}

		logger.Info("Initialized tendermint", "genesis", config.GenesisFile(), "priv_validator", config.PrivValidatorFile())
	} else {
		logger.Info("Already initialized", "priv_validator", config.PrivValidatorFile())
	}
}
