// +build goleveldb

package metadb

import (
	tmdb "github.com/tendermint/tm-db"
	"github.com/tendermint/tm-db/goleveldb"
)

func golevelDBCreator(name, dir string) (tmdb.DB, error) {
	return goleveldb.NewDB(name, dir)
}

func init() { registerDBCreator(GoLevelDBBackend, golevelDBCreator, true) }
