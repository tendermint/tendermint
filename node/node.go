// Package node provides a high level wrapper around tendermint services.
package node

import (
	"github.com/tendermint/tendermint/config"
	tmni "github.com/tendermint/tendermint/internal/node"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/libs/service"
)

// New constructs a tendermint node service for use in go process that
// host their own process-local tendermint node.
func New(conf *config.Config, logger log.Logger) (service.Service, error) {
	return tmni.DefaultNewNode(conf, logger)
}
