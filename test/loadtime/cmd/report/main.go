package main

import (
	"encoding/csv"
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/tendermint/tendermint/store"
	"github.com/tendermint/tendermint/test/loadtime/report"
	dbm "github.com/tendermint/tm-db"
)

var (
	db     = flag.String("database-type", "goleveldb", "the type of database holding the blockstore")
	dir    = flag.String("data-dir", "", "path to the directory containing the tendermint databases")
	csvOut = flag.String("csv", "", "dump the extracted latencies as raw csv for use in additional tooling")
)

func main() {
	flag.Parse()
	if *db == "" {
		log.Fatalf("must specify a database-type")
	}
	if *dir == "" {
		log.Fatalf("must specify a data-dir")
	}
	d := strings.TrimPrefix(*dir, "~/")
	if d != *dir {
		h, err := os.UserHomeDir()
		if err != nil {
			panic(err)
		}
		d = h + "/" + d
	}
	_, err := os.Stat(d)
	if err != nil {
		panic(err)
	}
	dbType := dbm.BackendType(*db)
	db, err := dbm.NewDB("blockstore", dbType, d)
	if err != nil {
		panic(err)
	}
	s := store.NewBlockStore(db)
	defer s.Close()
	r, err := report.GenerateFromBlockStore(s)
	if err != nil {
		panic(err)
	}
	if *csvOut != "" {
		cf, err := os.Create(*csvOut)
		if err != nil {
			panic(err)
		}
		w := csv.NewWriter(cf)
		err = w.WriteAll(toRecords(r.All))
		if err != nil {
			panic(err)
		}
		return
	}

	fmt.Printf(""+
		"Total Valid Tx: %d\n"+
		"Total Invalid Tx: %d\n"+
		"Total Negative Latencies: %d\n"+
		"Minimum Latency: %s\n"+
		"Maximum Latency: %s\n"+
		"Average Latency: %s\n"+
		"Standard Deviation: %s\n", len(r.All), r.ErrorCount, r.NegativeCount, r.Min, r.Max, r.Avg, r.StdDev)
}

func toRecords(l []time.Duration) [][]string {
	res := make([][]string, len(l)+1)

	res[0] = make([]string, 1)
	res[0][0] = "duration_ns"
	for i, v := range l {
		res[1+i] = []string{strconv.FormatInt(int64(v), 10)}
	}
	return res
}
