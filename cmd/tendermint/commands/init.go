package commands

import (
	"os"

	"github.com/spf13/cobra"

	cmn "github.com/tendermint/tmlibs/common"
	"github.com/tendermint/tendermint/types"
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
