// +build memdb

package metadb

import (
	tmdb "github.com/tendermint/tm-db"
	"github.com/tendermint/tm-db/memdb"
)

func memdbDBCreator(name, dir string) (tmdb.DB, error) {
	return memdb.NewDB(), nil
}

func init() { registerDBCreator(MemDBBackend, memdbDBCreator, false) }
