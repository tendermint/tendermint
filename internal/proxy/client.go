package proxy

import (
	"io"

	abciclient "github.com/tendermint/tendermint/abci/client"
	"github.com/tendermint/tendermint/abci/example/kvstore"
	"github.com/tendermint/tendermint/abci/types"
)

// DefaultClientCreator returns a default ClientCreator, which will create a
// local client if addr is one of: 'kvstore',
// 'persistent_kvstore' or 'noop', otherwise - a remote client.
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
	case "noop":
		return abciclient.NewLocalCreator(types.NewBaseApplication()), noopCloser{}
	default:
		mustConnect := false // loop retrying
		return abciclient.NewRemoteCreator(addr, transport, mustConnect), noopCloser{}
	}
}

type noopCloser struct{}

func (noopCloser) Close() error { return nil }
