// +build rocksdb

package metadb

import (
	tmdb "github.com/tendermint/tm-db"
	"github.com/tendermint/tm-db/rocksdb"
)

func rocksDBCreator(name, dir string) (tmdb.DB, error) {
	return rocksdb.NewDB(name, dir)
}

func init() { registerDBCreator(RocksDBBackend, rocksDBCreator, true) }
