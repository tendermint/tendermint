package commands

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	cfg "github.com/pakula/prism/config"
	cmn "github.com/pakula/prism/libs/common"
	"github.com/pakula/prism/p2p"
	"github.com/pakula/prism/privval"
	"github.com/pakula/prism/types"
	"github.com/pakula/prism/crypto"
	tmtime "github.com/pakula/prism/types/time"
)

var (
	nLeagues       int
	nValidators    int
	nNonValidators int
	outputDir      string
	nodeDirPrefix  string

	populatePersistentPeers bool
	hostnamePrefix          string
	startingIPAddress       string
	p2pPort                 int
)

const (
	nodeDirPerm = 0755
)

func init() {
	TestPrismFilesCmd.Flags().IntVar(&nLeagues, "l", 3,
		"Number of leagues to initialize the testnet with")
	TestPrismFilesCmd.Flags().IntVar(&nValidators, "v", 3,
		"Number of nodes per validators to initialize the testnet with")
	TestPrismFilesCmd.Flags().IntVar(&nNonValidators, "n", 0,
		"Number of non-validators to initialize the testnet with")
	TestPrismFilesCmd.Flags().StringVar(&outputDir, "o", "./mytestnet",
		"Directory to store initialization data for the testnet")
	TestPrismFilesCmd.Flags().StringVar(&nodeDirPrefix, "node-dir-prefix", "node",
		"Prefix the directory name for each node with (node results in node0, node1, ...)")

	TestPrismFilesCmd.Flags().BoolVar(&populatePersistentPeers, "populate-persistent-peers", true,
		"Update config of each node with the list of persistent peers build using either hostname-prefix or starting-ip-address")
	TestPrismFilesCmd.Flags().StringVar(&hostnamePrefix, "hostname-prefix", "node",
		"Hostname prefix (node results in persistent peers list ID0@node0:26656, ID1@node1:26656, ...)")
	TestPrismFilesCmd.Flags().StringVar(&startingIPAddress, "starting-ip-address", "",
		"Starting IP address (192.168.0.1 results in persistent peers list ID0@192.168.0.1:26656, ID1@192.168.0.2:26656, ...)")
	TestPrismFilesCmd.Flags().IntVar(&p2pPort, "p2p-port", 26656,
		"P2P Port")
}

// TestPrismFilesCmd allows initialisation of files for a Tendermint testnet.
var TestPrismFilesCmd = &cobra.Command{
	Use:   "testnet",
	Short: "Initialize files for a Prism testnet",
	Long: `testnet will create "l*v" + "n" number of directories and populate each with
necessary files (private validator, genesis, config, etc.).

Note, strict routability for addresses is turned off in the config file.

Optionally, it will fill in persistent_peers list in config file using either hostnames or IPs.

Example:

	prism testnet --l 3 --v 3 --o ./network --populate-persistent-peers --starting-ip-address 192.168.10.2
	`,
	RunE: testPrismFiles,
}

type nodeInfo struct {
	league int
	nodeId int
	nodeDir string
	isValidator bool
}

var nodes []nodeInfo

func testPrismFiles(cmd *cobra.Command, args []string) error {
	config := cfg.DefaultConfig()
	genVals := make([]types.GenesisValidator, nValidators*nLeagues)
	nodes = make([]nodeInfo, nValidators*nLeagues+nNonValidators)

	for l := 0; l < nLeagues; l ++ {
		for i := 0; i < nValidators; i++ {
			id := l*nValidators + i
			nodeDirName := fmt.Sprintf("%s%d", nodeDirPrefix, id)
			nodeDir := filepath.Join(outputDir, nodeDirName)
			node := nodeInfo{l, id, nodeDir, true}
			nodes[id] = node

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

			initFilesWithConfig(config)

			pvKeyFile := filepath.Join(nodeDir, config.BaseConfig.PrivValidatorKey)
			pvStateFile := filepath.Join(nodeDir, config.BaseConfig.PrivValidatorState)

			pv := privval.LoadFilePV(pvKeyFile, pvStateFile)
			genVals[l*nValidators + i] = types.GenesisValidator{
				League:  l,
				NodeId:  l*nValidators + i,
				Address: pv.GetPubKey().Address(),
				PubKey:  pv.GetPubKey(),
				Power:   1,
				Name:    nodeDirName,
			}
		}
	}

	for i := 0; i < nNonValidators; i++ {
		id := nLeagues*nValidators + i
		nodeDirName := fmt.Sprintf("%s%d", nodeDirPrefix, id)
		nodeDir := filepath.Join(outputDir, nodeDirName)
		nodes[nLeagues*nValidators+i] = nodeInfo{-1, id, nodeDir, false}

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

		initFilesWithConfig(config)
	}

	// Generate genesis doc from generated validators
	genDoc := &types.GenesisDoc{
		GenesisTime: tmtime.Now(),
		ChainID:     "chain-" + cmn.RandStr(6),
		Validators:  genVals,
	}

	// Write genesis file.
	for _, node := range nodes  {
		if err := genDoc.SaveAs(filepath.Join(node.nodeDir, config.BaseConfig.Genesis)); err != nil {
			_ = os.RemoveAll(outputDir)
			return err
		}
	}

	// Generate league topology file
	leaguesDoc := &types.LeaguesDoc{
		Leagues: nLeagues,
		Peers:  fillLeagues(),
	}

	// Write league topology file.
	for _, node := range nodes  {
		if err := leaguesDoc.SaveAs(filepath.Join(node.nodeDir, config.BaseConfig.Leagues)); err != nil {
			_ = os.RemoveAll(outputDir)
			return err
		}
	}

	// Gather persistent peer addresses.
	var (
		persistentPeers string
		err             error
	)
	if populatePersistentPeers {
		persistentPeers, err = persistentPeersString(config)
		if err != nil {
			_ = os.RemoveAll(outputDir)
			return err
		}
	}

	// Overwrite default config.
	for _, node := range nodes {
		config.SetRoot(node.nodeDir)

		config.LogLevel = "*:error"

		config.Consensus.UseLeagues = false
		config.Consensus.League = node.league
		config.Consensus.NodeId = node.nodeId
		config.Consensus.CreateEmptyBlocksInterval = 5 * time.Second

		config.P2P.AddrBookStrict = false
		config.P2P.AllowDuplicateIP = true
		if populatePersistentPeers {
			config.P2P.PersistentPeers = persistentPeers
		}

		cfg.WriteConfigFile(filepath.Join(node.nodeDir, "config", "config.toml"), config)
	}

	fmt.Printf("Successfully initialized %v node directories\n", len(nodes))
	return nil
}

func hostnameOrIP(i int) string {
	if startingIPAddress != "" {
		ip := net.ParseIP(startingIPAddress)
		ip = ip.To4()
		if ip == nil {
			fmt.Printf("%v: non ipv4 address\n", startingIPAddress)
			os.Exit(1)
		}

		for j := 0; j < i; j++ {
			ip[3]++
		}
		return ip.String()
	}

	return fmt.Sprintf("%s%d", hostnamePrefix, i)
}

func fillLeagues() ([]types.LeaguePeer) {
	result := make([]types.LeaguePeer, nLeagues*nValidators)
	for i := range result {
		node := nodes[i]
		result[i] = types.LeaguePeer{
			League:node.league, 
			NodeId:node.nodeId, 
			PubKey:readPubKey(node.nodeDir),
			Hostname: hostnameOrIP(node.nodeId),
		}
	}
	return result
}

func readPubKey(cfgDir string) crypto.PubKey {
	config.SetRoot(cfgDir)
	nodeKey, err := p2p.LoadNodeKey(config.NodeKeyFile())
	if err != nil {
		panic(fmt.Sprintf("Failed to read node key: %s", config.NodeKeyFile()))
	}
	return nodeKey.PubKey()
}

func persistentPeersString(config *cfg.Config) (string, error) {
	persistentPeers := make([]string, len(nodes))
	for i, node := range nodes {
		config.SetRoot(node.nodeDir)
		nodeKey, err := p2p.LoadNodeKey(config.NodeKeyFile())
		if err != nil {
			return "", err
		}
		persistentPeers[i] = p2p.IDAddressString(nodeKey.ID(), fmt.Sprintf("%s:%d", hostnameOrIP(i), p2pPort))
	}
	return strings.Join(persistentPeers, ","), nil
}
