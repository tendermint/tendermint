package commands

import (
	"github.com/spf13/cobra"

	crypto "github.com/libp2p/go-libp2p-crypto"
	"github.com/tendermint/go-wire/data"
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
	// private validator
	privValFile := config.PrivValidatorFile()
	var privValidator *types.PrivValidatorFS
	if cmn.FileExists(privValFile) {
		privValidator = types.LoadPrivValidatorFS(privValFile)
		logger.Info("Found private validator", "path", privValFile)
	} else {
		privValidator = types.GenPrivValidatorFS(privValFile)
		privValidator.Save()
		logger.Info("Genetated private validator", "path", privValFile)
	}

	genFile := config.GenesisFile()
	if cmn.FileExists(genFile) {
		logger.Info("Found genesis file", "path", genFile)
	} else {
		genDoc := types.GenesisDoc{
			ChainID: cmn.Fmt("test-chain-%v", cmn.RandStr(6)),
		}
		pubKeyBytes, err := crypto.MarshalPublicKey(privValidator.GetPubKey())
		if err != nil {
			panic(err)
		}
		pubKeyStr, err := data.Encoder.Marshal(pubKeyBytes)
		if err != nil {
			panic(err)
		}
		genDoc.Validators = []types.GenesisValidator{{
			PubKey: string(pubKeyStr),
			Power:  10,
		}}
		if err := genDoc.SaveAs(genFile); err != nil {
			panic(err)
		}
		logger.Info("Generated genesis file", "path", genFile)
	}
}
