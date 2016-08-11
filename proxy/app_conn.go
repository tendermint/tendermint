package proxy

import (
	cfg "github.com/tendermint/go-config"
	tmspcli "github.com/tendermint/tmsp/client"
)

type AppConn interface {
	tmspcli.Client
}

type GetProxyApp func(cfg.Config) (AppConn, error)
