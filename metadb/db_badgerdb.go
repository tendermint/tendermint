// +build badgerdb

package metadb

import (
	tmdb "github.com/tendermint/tm-db"
	"github.com/tendermint/tm-db/badgerdb"
)

func badgerDBCreator(name, dir string) (tmdb.DB, error) {
	return badgerdb.NewDB(name, dir)
}

func init() { registerDBCreator(BadgerDBBackend, badgerDBCreator, true) }
