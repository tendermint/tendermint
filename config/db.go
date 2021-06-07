package config

import (
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/libs/service"
	db "github.com/tendermint/tm-db"
)

// ServiceProvider takes a config and a logger and returns a ready to go Node.
type ServiceProvider func(*Config, log.Logger) (service.Service, error)

// DBProivder is an interface for initializing instances of DB
type DBProvider interface {
	Initialize(ID string) (db.DB, error)
}

// DefaultDBProvider is the default provider that is constructed from the FileConfig
type DefaultDBProvider struct {
	backend string
	dir string
}

func (dbp DefaultDBProvider) Initialize(ID string) (db.DB, error) {
	dbType := db.BackendType(dbp.backend)
	return db.NewDB(ID, dbType, dbp.dir)
}



