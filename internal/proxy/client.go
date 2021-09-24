package proxy

import (
	"io"

	abciclient "github.com/tendermint/tendermint/abci/client"
	"github.com/tendermint/tendermint/abci/example/kvstore"
	"github.com/tendermint/tendermint/abci/types"
	e2e "github.com/tendermint/tendermint/test/e2e/app"
)

// DefaultClientCreator returns a default ClientCreator, which will create a
// local client if addr is one of: 'kvstore',
// 'persistent_kvstore', 'e2e', or 'noop', otherwise - a remote client.
//
// The Closer is a noop except for persistent_kvstore applications,
// which will clean up the store.
func DefaultClientCreator(addr, transport, dbDir string) (abciclient.Creator, io.Closer) {
	switch addr {
	case "kvstore":
		return abciclient.NewLocalCreator(kvstore.NewApplication()), noopCloser{}
	case "persistent_kvstore":
		app := kvstore.NewPersistentKVStoreApplication(dbDir)
		return abciclient.NewLocalCreator(app), app
	case "e2e":
		app, err := e2e.NewApplication(e2e.DefaultConfig(dbDir))
		if err != nil {
			panic(err)
		}
		return abciclient.NewLocalCreator(app), noopCloser{}
	case "noop":
		return abciclient.NewLocalCreator(types.NewBaseApplication()), noopCloser{}
	default:
		mustConnect := false // loop retrying
		return abciclient.NewRemoteCreator(addr, transport, mustConnect), noopCloser{}
	}
}

type noopCloser struct{}

func (noopCloser) Close() error { return nil }
