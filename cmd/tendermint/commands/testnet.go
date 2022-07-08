package commands

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	cfg "github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/libs/bytes"
	"github.com/tendermint/tendermint/libs/log"
	tmrand "github.com/tendermint/tendermint/libs/rand"
	tmtime "github.com/tendermint/tendermint/libs/time"
	"github.com/tendermint/tendermint/privval"
	"github.com/tendermint/tendermint/types"
)

const (
	nodeDirPerm = 0755
)

// MakeTestnetFilesCommand constructs a command to generate testnet config files.
func MakeTestnetFilesCommand(conf *cfg.Config, logger log.Logger) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "testnet",
		Short: "Initialize files for a Tendermint testnet",
		Long: `testnet will create "v" + "n" number of directories and populate each with
necessary files (private validator, genesis, config, etc.).

Note, strict routability for addresses is turned off in the config file.

Optionally, it will fill in persistent-peers list in config file using either hostnames or IPs.

Example:

	tendermint testnet --v 4 --o ./output --populate-persistent-peers --starting-ip-address 192.168.10.2
	`,
	}
	var (
		nValidators    int
		nNonValidators int
		initialHeight  int64
		configFile     string
		outputDir      string
		nodeDirPrefix  string

		populatePersistentPeers bool
		hostnamePrefix          string
		hostnameSuffix          string
		startingIPAddress       string
		hostnames               []string
		p2pPort                 int
		randomMonikers          bool
		keyType                 string
	)

	cmd.Flags().IntVar(&nValidators, "v", 4,
		"number of validators to initialize the testnet with")
	cmd.Flags().StringVar(&configFile, "config", "",
		"config file to use (note some options may be overwritten)")
	cmd.Flags().IntVar(&nNonValidators, "n", 0,
		"number of non-validators to initialize the testnet with")
	cmd.Flags().StringVar(&outputDir, "o", "./mytestnet",
		"directory to store initialization data for the testnet")
	cmd.Flags().StringVar(&nodeDirPrefix, "node-dir-prefix", "node",
		"prefix the directory name for each node with (node results in node0, node1, ...)")
	cmd.Flags().Int64Var(&initialHeight, "initial-height", 0,
		"initial height of the first block")

	cmd.Flags().BoolVar(&populatePersistentPeers, "populate-persistent-peers", true,
		"update config of each node with the list of persistent peers build using either"+
			" hostname-prefix or"+
			" starting-ip-address")
	cmd.Flags().StringVar(&hostnamePrefix, "hostname-prefix", "node",
		"hostname prefix (\"node\" results in persistent peers list ID0@node0:26656, ID1@node1:26656, ...)")
	cmd.Flags().StringVar(&hostnameSuffix, "hostname-suffix", "",
		"hostname suffix ("+
			"\".xyz.com\""+
			" results in persistent peers list ID0@node0.xyz.com:26656, ID1@node1.xyz.com:26656, ...)")
	cmd.Flags().StringVar(&startingIPAddress, "starting-ip-address", "",
		"starting IP address ("+
			"\"192.168.0.1\""+
			" results in persistent peers list ID0@192.168.0.1:26656, ID1@192.168.0.2:26656, ...)")
	cmd.Flags().StringArrayVar(&hostnames, "hostname", []string{},
		"manually override all hostnames of validators and non-validators (use --hostname multiple times for multiple hosts)")
	cmd.Flags().IntVar(&p2pPort, "p2p-port", 26656,
		"P2P Port")
	cmd.Flags().BoolVar(&randomMonikers, "random-monikers", false,
		"randomize the moniker for each generated node")
	cmd.Flags().StringVar(&keyType, "key", types.ABCIPubKeyTypeEd25519,
		"Key type to generate privval file with. Options: ed25519, secp256k1")

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if len(hostnames) > 0 && len(hostnames) != (nValidators+nNonValidators) {
			return fmt.Errorf(
				"testnet needs precisely %d hostnames (number of validators plus non-validators) if --hostname parameter is used",
				nValidators+nNonValidators,
			)
		}

		// set mode to validator for testnet
		config := cfg.DefaultValidatorConfig()

		// overwrite default config if set and valid
		if configFile != "" {
			viper.SetConfigFile(configFile)
			if err := viper.ReadInConfig(); err != nil {
				return err
			}
			if err := viper.Unmarshal(config); err != nil {
				return err
			}
			if err := config.ValidateBasic(); err != nil {
				return err
			}
		}

		genVals := make([]types.GenesisValidator, nValidators)
		ctx := cmd.Context()
		for i := 0; i < nValidators; i++ {
			nodeDirName := fmt.Sprintf("%s%d", nodeDirPrefix, i)
			nodeDir := filepath.Join(outputDir, nodeDirName)
			config.SetRoot(nodeDir)

			err := os.MkdirAll(filepath.Join(nodeDir, "config"), nodeDirPerm)
			if err != nil {
				_ = os.RemoveAll(outputDir)
				return err
			}
			err = os.MkdirAll(filepath.Join(nodeDir, "data"), nodeDirPerm)
			if err != nil {
				_ = os.RemoveAll(outputDir)
				return err
			}

			if err := initFilesWithConfig(ctx, config, logger, keyType); err != nil {
				return err
			}

			pvKeyFile := filepath.Join(nodeDir, config.PrivValidator.Key)
			pvStateFile := filepath.Join(nodeDir, config.PrivValidator.State)
			pv, err := privval.LoadFilePV(pvKeyFile, pvStateFile)
			if err != nil {
				return err
			}

			ctx, cancel := context.WithTimeout(ctx, ctxTimeout)
			defer cancel()

			pubKey, err := pv.GetPubKey(ctx)
			if err != nil {
				return fmt.Errorf("can't get pubkey: %w", err)
			}
			genVals[i] = types.GenesisValidator{
				Address: pubKey.Address(),
				PubKey:  pubKey,
				Power:   1,
				Name:    nodeDirName,
			}
		}

		for i := 0; i < nNonValidators; i++ {
			nodeDir := filepath.Join(outputDir, fmt.Sprintf("%s%d", nodeDirPrefix, i+nValidators))
			config.SetRoot(nodeDir)

			err := os.MkdirAll(filepath.Join(nodeDir, "config"), nodeDirPerm)
			if err != nil {
				_ = os.RemoveAll(outputDir)
				return err
			}

			err = os.MkdirAll(filepath.Join(nodeDir, "data"), nodeDirPerm)
			if err != nil {
				_ = os.RemoveAll(outputDir)
				return err
			}

			if err := initFilesWithConfig(ctx, conf, logger, keyType); err != nil {
				return err
			}
		}

		// Generate genesis doc from generated validators
		genDoc := &types.GenesisDoc{
			ChainID:         "chain-" + tmrand.Str(6),
			GenesisTime:     tmtime.Now(),
			InitialHeight:   initialHeight,
			Validators:      genVals,
			ConsensusParams: types.DefaultConsensusParams(),
		}
		if keyType == "secp256k1" {
			genDoc.ConsensusParams.Validator = types.ValidatorParams{
				PubKeyTypes: []string{types.ABCIPubKeyTypeSecp256k1},
			}
		}

		// Write genesis file.
		for i := 0; i < nValidators+nNonValidators; i++ {
			nodeDir := filepath.Join(outputDir, fmt.Sprintf("%s%d", nodeDirPrefix, i))
			if err := genDoc.SaveAs(filepath.Join(nodeDir, config.BaseConfig.Genesis)); err != nil {
				_ = os.RemoveAll(outputDir)
				return err
			}
		}

		// Gather persistent peer addresses.
		var (
			persistentPeers = make([]string, 0)
			err             error
		)
		tpargs := testnetPeerArgs{
			numValidators:    nValidators,
			numNonValidators: nNonValidators,
			peerToPeerPort:   p2pPort,
			nodeDirPrefix:    nodeDirPrefix,
			outputDir:        outputDir,
			hostnames:        hostnames,
			startingIPAddr:   startingIPAddress,
			hostnamePrefix:   hostnamePrefix,
			hostnameSuffix:   hostnameSuffix,
			randomMonikers:   randomMonikers,
		}

		if populatePersistentPeers {

			persistentPeers, err = persistentPeersArray(config, tpargs)
			if err != nil {
				_ = os.RemoveAll(outputDir)
				return err
			}
		}

		// Overwrite default config.
		for i := 0; i < nValidators+nNonValidators; i++ {
			nodeDir := filepath.Join(outputDir, fmt.Sprintf("%s%d", nodeDirPrefix, i))
			config.SetRoot(nodeDir)
			if populatePersistentPeers {
				persistentPeersWithoutSelf := make([]string, 0)
				for j := 0; j < len(persistentPeers); j++ {
					if j == i {
						continue
					}
					persistentPeersWithoutSelf = append(persistentPeersWithoutSelf, persistentPeers[j])
				}
				config.P2P.PersistentPeers = strings.Join(persistentPeersWithoutSelf, ",")
			}
			config.Moniker = tpargs.moniker(i)

			if err := cfg.WriteConfigFile(nodeDir, config); err != nil {
				return err
			}
		}

		fmt.Printf("Successfully initialized %v node directories\n", nValidators+nNonValidators)
		return nil
	}

	return cmd
}

type testnetPeerArgs struct {
	numValidators    int
	numNonValidators int
	peerToPeerPort   int
	nodeDirPrefix    string
	outputDir        string
	hostnames        []string
	startingIPAddr   string
	hostnamePrefix   string
	hostnameSuffix   string
	randomMonikers   bool
}

func (args *testnetPeerArgs) hostnameOrIP(i int) (string, error) {
	if len(args.hostnames) > 0 && i < len(args.hostnames) {
		return args.hostnames[i], nil
	}
	if args.startingIPAddr == "" {
		return fmt.Sprintf("%s%d%s", args.hostnamePrefix, i, args.hostnameSuffix), nil
	}
	ip := net.ParseIP(args.startingIPAddr)
	ip = ip.To4()
	if ip == nil {
		return "", fmt.Errorf("%v is non-ipv4 address", args.startingIPAddr)
	}

	for j := 0; j < i; j++ {
		ip[3]++
	}
	return ip.String(), nil

}

// get an array of persistent peers
func persistentPeersArray(config *cfg.Config, args testnetPeerArgs) ([]string, error) {
	peers := make([]string, args.numValidators+args.numNonValidators)
	for i := 0; i < len(peers); i++ {
		nodeDir := filepath.Join(args.outputDir, fmt.Sprintf("%s%d", args.nodeDirPrefix, i))
		config.SetRoot(nodeDir)
		nodeKey, err := config.LoadNodeKeyID()
		if err != nil {
			return nil, err
		}
		addr, err := args.hostnameOrIP(i)
		if err != nil {
			return nil, err
		}

		peers[i] = nodeKey.AddressString(fmt.Sprintf("%s:%d", addr, args.peerToPeerPort))
	}
	return peers, nil
}

func (args *testnetPeerArgs) moniker(i int) string {
	if args.randomMonikers {
		return randomMoniker()
	}
	if len(args.hostnames) > 0 && i < len(args.hostnames) {
		return args.hostnames[i]
	}
	if args.startingIPAddr == "" {
		return fmt.Sprintf("%s%d%s", args.hostnamePrefix, i, args.hostnameSuffix)
	}
	return randomMoniker()
}

func randomMoniker() string {
	return bytes.HexBytes(tmrand.Bytes(8)).String()
}
