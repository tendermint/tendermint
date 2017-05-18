package commands

import (
	"fmt"
	"path"
	"time"

	"github.com/spf13/cobra"

	cmn "github.com/tendermint/tmlibs/common"
	"github.com/tendermint/tendermint/types"
)

var testnetFilesCmd = &cobra.Command{
	Use:   "testnet",
	Short: "Initialize files for a Tendermint testnet",
	Run:   testnetFiles,
}

//flags
var (
	nValidators int
	dataDir     string
)

func init() {
	testnetFilesCmd.Flags().IntVar(&nValidators, "n", 4,
		"Number of validators to initialize the testnet with")
	testnetFilesCmd.Flags().StringVar(&dataDir, "dir", "mytestnet",
		"Directory to store initialization data for the testnet")

	RootCmd.AddCommand(testnetFilesCmd)
}

func testnetFiles(cmd *cobra.Command, args []string) {

	genVals := make([]types.GenesisValidator, nValidators)

	// Initialize core dir and priv_validator.json's
	for i := 0; i < nValidators; i++ {
		mach := cmn.Fmt("mach%d", i)
		err := initMachCoreDirectory(dataDir, mach)
		if err != nil {
			cmn.Exit(err.Error())
		}
		// Read priv_validator.json to populate vals
		privValFile := path.Join(dataDir, mach, "priv_validator.json")
		privVal := types.LoadPrivValidator(privValFile)
		genVals[i] = types.GenesisValidator{
			PubKey: privVal.PubKey,
			Amount: 1,
			Name:   mach,
		}
	}

	// Generate genesis doc from generated validators
	genDoc := &types.GenesisDoc{
		GenesisTime: time.Now(),
		ChainID:     "chain-" + cmn.RandStr(6),
		Validators:  genVals,
	}

	// Write genesis file.
	for i := 0; i < nValidators; i++ {
		mach := cmn.Fmt("mach%d", i)
		genDoc.SaveAs(path.Join(dataDir, mach, "genesis.json"))
	}

	fmt.Println(cmn.Fmt("Successfully initialized %v node directories", nValidators))
}

// Initialize per-machine core directory
func initMachCoreDirectory(base, mach string) error {
	dir := path.Join(base, mach)
	err := cmn.EnsureDir(dir, 0777)
	if err != nil {
		return err
	}

	// Create priv_validator.json file if not present
	ensurePrivValidator(path.Join(dir, "priv_validator.json"))
	return nil

}

func ensurePrivValidator(file string) {
	if cmn.FileExists(file) {
		return
	}
	privValidator := types.GenPrivValidator()
	privValidator.SetFile(file)
	privValidator.Save()
}
