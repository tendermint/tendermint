package config

import (
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/libs/service"
	db "github.com/tendermint/tm-db"
	"github.com/tendermint/tm-db/metadb"
)

// ServiceProvider takes a config and a logger and returns a ready to go Node.
type ServiceProvider func(*Config, log.Logger) (service.Service, error)

// DBContext specifies config information for loading a new DB.
type DBContext struct {
	ID     string
	Config *Config
}

// DBProvider takes a DBContext and returns an instantiated DB.
type DBProvider func(*DBContext) (db.DB, error)

// DefaultDBProvider returns a database using the DBBackend and DBDir
// specified in the Config.
func DefaultDBProvider(ctx *DBContext) (db.DB, error) {
	dbType := metadb.BackendType(ctx.Config.DBBackend)
	return metadb.NewDB(ctx.ID, dbType, ctx.Config.DBDir())
}
