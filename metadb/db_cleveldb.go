// +build cleveldb

package metadb

import (
	tmdb "github.com/tendermint/tm-db"
	"github.com/tendermint/tm-db/cleveldb"
)

func clevelDBCreator(name string, dir string) (tmdb.DB, error) {
	return cleveldb.NewDB(name, dir)
}

func init() { registerDBCreator(CLevelDBBackend, clevelDBCreator, false) }
