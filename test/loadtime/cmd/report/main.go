package main

import (
	"fmt"

	"github.com/tendermint/tendermint/store"
	"github.com/tendermint/tendermint/test/loadtime/report"
	dbm "github.com/tendermint/tm-db"
)

func main() {
	// parse out the db location
	db, err := dbm.NewDB("blockstore", dbm.GoLevelDBBackend, "/home/william/.tendermint/data")
	if err != nil {
		panic(err)
	}
	s := store.NewBlockStore(db)
	r, err := report.GenerateFromBlockStore(s)
	if err != nil {
		panic(err)
	}
	fmt.Println(r.ErrorCount)
	fmt.Println(len(r.All))
	fmt.Println(r.Min)
	fmt.Println(r.Max)
	fmt.Println(r.Avg)
	fmt.Println(r.StdDev)
}
