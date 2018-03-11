package commands

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	cfg "github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/types"
	cmn "github.com/tendermint/tmlibs/common"
)

//flags
var (
	nValidators int
	dataDir     string
)

func init() {
	TestnetFilesCmd.Flags().IntVar(&nValidators, "n", 4,
		"Number of validators to initialize the testnet with")
	TestnetFilesCmd.Flags().StringVar(&dataDir, "dir", "mytestnet",
		"Directory to store initialization data for the testnet")
}

// TestnetFilesCmd allows initialisation of files for a
// Tendermint testnet.
var TestnetFilesCmd = &cobra.Command{
	Use:   "testnet",
	Short: "Initialize files for a Tendermint testnet",
	Run:   testnetFiles,
}

func testnetFiles(cmd *cobra.Command, args []string) {

	genVals := make([]types.GenesisValidator, nValidators)
	defaultConfig := cfg.DefaultBaseConfig()

	// Initialize core dir and priv_validator.json's
	for i := 0; i < nValidators; i++ {
		mach := cmn.Fmt("mach%d", i)
		err := initMachCoreDirectory(dataDir, mach)
		if err != nil {
			cmn.Exit(err.Error())
		}
		// Read priv_validator.json to populate vals
		privValFile := filepath.Join(dataDir, mach, defaultConfig.PrivValidator)
		privVal := types.LoadPrivValidatorFS(privValFile)
		genVals[i] = types.GenesisValidator{
			PubKey: privVal.GetPubKey(),
			Power:  1,
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
		if err := genDoc.SaveAs(filepath.Join(dataDir, mach, defaultConfig.Genesis)); err != nil {
			panic(err)
		}
	}

	fmt.Println(cmn.Fmt("Successfully initialized %v node directories", nValidators))
}

// Initialize per-machine core directory
func initMachCoreDirectory(base, mach string) error {
	// Create priv_validator.json file if not present
	defaultConfig := cfg.DefaultBaseConfig()
	dir := filepath.Join(base, mach)
	privValPath := filepath.Join(dir, defaultConfig.PrivValidator)
	dir = filepath.Dir(privValPath)
	err := cmn.EnsureDir(dir, 0700)
	if err != nil {
		return err
	}
	ensurePrivValidator(privValPath)
	return nil

}

func ensurePrivValidator(file string) {
	if cmn.FileExists(file) {
		return
	}
	privValidator := types.GenPrivValidatorFS(file)
	privValidator.Save()
}
