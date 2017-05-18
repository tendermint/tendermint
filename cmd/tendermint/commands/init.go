package commands

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/tendermint/tendermint/types"
	cmn "github.com/tendermint/tmlibs/common"
)

var initFilesCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize Tendermint",
	Run:   initFiles,
}

func init() {
	RootCmd.AddCommand(initFilesCmd)
}

func initFiles(cmd *cobra.Command, args []string) {
	privValFile := config.PrivValidatorFile()
	if _, err := os.Stat(privValFile); os.IsNotExist(err) {
		privValidator := types.GenPrivValidator()
		privValidator.SetFile(privValFile)
		privValidator.Save()

		genFile := config.GenesisFile()

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

		logger.Info("Initialized tendermint", "genesis", config.GenesisFile(), "priv_validator", config.PrivValidatorFile())
	} else {
		logger.Info("Already initialized", "priv_validator", config.PrivValidatorFile())
	}
}
