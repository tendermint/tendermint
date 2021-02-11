// +build boltdb

package metadb

import (
	tmdb "github.com/tendermint/tm-db"
	"github.com/tendermint/tm-db/boltdb"
)

func boltDBCreator(name, dir string) (tmdb.DB, error) {
	return boltdb.NewDB(name, dir)
}

func init() { registerDBCreator(BoltDBBackend, boltDBCreator, true) }
