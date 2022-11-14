package orderbook

import dbm "github.com/tendermint/tm-db"

type AccountStore struct {
	db dbm.DB
}

// iterate over the account database
// Add to the account database
// find an account
