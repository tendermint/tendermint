package proxy

import (
	"io"

	abciclient "github.com/tendermint/tendermint/abci/client"
	"github.com/tendermint/tendermint/abci/example/kvstore"
	"github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/libs/log"
	e2e "github.com/tendermint/tendermint/test/e2e/app"
)

// ClientFactory returns a default ClientCreator, which will create a
// local client if addr is one of: 'kvstore',
// 'persistent_kvstore', 'e2e', or 'noop', otherwise - a remote client.
//
// The Closer is a noop except for persistent_kvstore applications,
// which will clean up the store.
func ClientFactory(logger log.Logger, addr, transport, dbDir string) (abciclient.Client, io.Closer, error) {
	switch addr {
	case "kvstore":
		return abciclient.NewLocalClient(logger, kvstore.NewApplication()), noopCloser{}, nil
	case "persistent_kvstore":
		app := kvstore.NewPersistentKVStoreApplication(logger, dbDir)
		return abciclient.NewLocalClient(logger, app), app, nil
	case "e2e":
		app, err := e2e.NewApplication(e2e.DefaultConfig(dbDir))
		if err != nil {
			return nil, noopCloser{}, err
		}
		return abciclient.NewLocalClient(logger, app), noopCloser{}, nil
	case "noop":
		return abciclient.NewLocalClient(logger, types.NewBaseApplication()), noopCloser{}, nil
	default:
		mustConnect := false // loop retrying
		client, err := abciclient.NewClient(logger, addr, transport, mustConnect)
		if err != nil {
			return nil, noopCloser{}, err
		}

		return client, noopCloser{}, nil
	}
}

type noopCloser struct{}

func (noopCloser) Close() error { return nil }
