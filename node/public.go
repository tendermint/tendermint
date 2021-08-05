// Package node provides a high level wrapper around tendermint services.
package node

import (
	"fmt"

	"github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/libs/service"
	"github.com/tendermint/tendermint/privval"
	"github.com/tendermint/tendermint/proxy"
	"github.com/tendermint/tendermint/types"
)

// NewDefault constructs a tendermint node service for use in go
// process that host their own process-local tendermint node. This is
// equivalent to running tendermint in it's own process communicating
// to an external ABCI application.
func NewDefault(conf *config.Config, logger log.Logger) (service.Service, error) {
	return newDefaultNode(conf, logger)
}

// New constructs a tendermint node. The ClientCreator makes it
// possible to construct an ABCI application that runs in the same
// process as the tendermint node.  The final option is a pointer to a
// Genesis document: if the value is nil, the genesis document is read
// from the file specified in the config, and otherwise the node uses
// value of the final argument.
func New(conf *config.Config,
	logger log.Logger,
	cf proxy.ClientCreator,
	gen *types.GenesisDoc,
) (service.Service, error) {
	nodeKey, err := types.LoadOrGenNodeKey(conf.NodeKeyFile())
	if err != nil {
		return nil, fmt.Errorf("failed to load or gen node key %s: %w", conf.NodeKeyFile(), err)
	}

	var genProvider genesisDocProvider
	switch gen {
	case nil:
		genProvider = defaultGenesisDocProviderFunc(conf)
	default:
		genProvider = func() (*types.GenesisDoc, error) { return gen, nil }
	}

	switch conf.Mode {
	case config.ModeFull, config.ModeValidator:
		pval, err := privval.LoadOrGenFilePV(conf.PrivValidator.KeyFile(), conf.PrivValidator.StateFile())
		if err != nil {
			return nil, err
		}

		return makeNode(conf,
			pval,
			nodeKey,
			cf,
			genProvider,
			config.DefaultDBProvider,
			logger)
	case config.ModeSeed:
		return makeSeedNode(conf, config.DefaultDBProvider, nodeKey, genProvider, logger)
	default:
		return nil, fmt.Errorf("%q is not a valid mode", conf.Mode)
	}
}
