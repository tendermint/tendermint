package proxy

import (
	tmspcli "github.com/tendermint/tmsp/client"
)

type AppConn interface {
	tmspcli.Client
}
