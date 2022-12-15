package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/gookit/goutil/dump"
	tmrand "github.com/tendermint/tendermint/libs/rand"
	dbm "github.com/tendermint/tm-db"
)

// This struct is used to keep track of the steps taken during the test.
// Describes the size of the db and the number of records in it at various
// points during a test
type Step struct {
	// The name of the step
	Name string
	// The size of the db in kb
	Size int64
	// The number of records in the db
	Records int
}

// This variable stores the path in the filesystem where the db will be created.
var dbPath string = "./db"

// Number of records we'll insert in the db
var recordsCount int = 10_000

// Defines the size (bytes)  of each key written in the DB
var keySize int = 10

// Defines the size (bytes) of each value written in the DB
var valueSize int = 100

func main() {
	fmt.Println("\n\t\t\t---- starting tests with GoLevelDB")

	// Run one test
	steps := runTest()
	PrintSteps(steps)
}

// Runs one test, including cleanup and stats printing.
// Returns the steps taken during the test.
func runTest() []Step {
	dbPathRemove()

	// Keep track of steps taken during the test
	var steps []Step

	// Instantiates a new db
	db, err := dbm.NewDB("goleveldb_perf_one", dbm.GoLevelDBBackend, dbPath)
	if err != nil {
		panic(fmt.Errorf("error instantiating the db: %w", err))
	}
	defer db.Close()

	steps = append(steps, Step{
		Name:    "initial",
		Size:    dirSize(dbPath),
		Records: dbCount(db),
	})

	// Insert records
	steps = append(steps, step("insert", recordsCount, db))

	// Delete records
	steps = append(steps, step("delete", 9_000, db))

	// Insert records
	steps = append(steps, step("insert", recordsCount, db))

	// Delete records
	steps = append(steps, step("delete", 9_000, db))

	// Delete records
	steps = append(steps, step("delete", 2_000, db))

	return steps
}

// Performs one step as part of a test.
//
// The step is defined by `stepType`: `delete` or `insert`.
// The `count` defines the number of records to delete or insert.
// The `db` is the db to perform the step on.
//
// Returns the step that was performed.
func step(stepType string, count int, db dbm.DB) Step {
	if stepType == "delete" {
		// Handle the deletion of records
		// Iterate over the db and delete the first `count` records
		iter, err := db.Iterator(nil, nil)
		if err != nil {
			panic(fmt.Errorf("error calling Iterator(): %w", err))
		}
		defer iter.Close()

		deleted := 0
		for ; iter.Valid(); iter.Next() {
			if err := db.Delete(iter.Key()); err != nil {
				panic(fmt.Errorf("error during Delete(): %w", err))
			}
			deleted++
			if deleted == count {
				fmt.Printf("\n\t\t\t---- deleted %v records", count)
				break
			}
		}
	} else if stepType == "insert" {
		// Handle the insertion of records
		// Write `count` records to the db
		for i := 0; i < count; i++ {
			if err := db.Set(tmrand.Bytes(keySize), tmrand.Bytes(valueSize)); err != nil {
				panic(fmt.Errorf("error during Set(): %w", err))
			}
		}
		fmt.Printf("\n\t\t\t---- inserted %v records", recordsCount)
	} else {
		panic("invalid step type")
	}

	// Print some stats about the db
	// dumpDbStats(db, dbPath)

	return Step{
		Name:    stepType,
		Size:    dirSize(dbPath),
		Records: dbCount(db),
	}
}

// Handles the removal of all db files.
// Should be called at the beginning of any test.
func dbPathRemove() {
	err := os.RemoveAll(dbPath)
	if err != nil {
		panic(fmt.Errorf("error removing the db directory: %w", err))
	}
}

// Fetches and prints the stats of the db.
func dumpDbStats(db dbm.DB, path string) {
	stats := db.Stats()

	// Add in the stats the directory size_bytes in the stats
	stats["dir_size_bytes"] = fmt.Sprintf("%v", dirSize(path))

	// Add in the stats the total # of bytes `Set`
	stats["set_bytes"] = fmt.Sprintf("%v", recordsCount*(keySize+valueSize))

	// Add to the stats the total # of records present in the db
	stats["records_count"] = fmt.Sprintf("%v", dbCount(db))

	dump.P(stats)
}

// Counts the number of records in the `db`.
func dbCount(db dbm.DB) int {
	iter, err := db.Iterator(nil, nil)
	if err != nil {
		panic(fmt.Errorf("error calling Iterator(): %w", err))
	}
	defer iter.Close()
	iterCount := 0
	for ; iter.Valid(); iter.Next() {
		iterCount++
	}
	return iterCount
}

// Computes the size of the given `path` directory in bytes.
// Panics if any error occurs.
func dirSize(path string) int64 {
	var size int64
	err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			panic(fmt.Errorf("error opening files inside the db directory: %w", err))
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return err
	})
	if err != nil {
		panic(fmt.Errorf("error walking the db directory: %w", err))
	}

	return size / 1024
}

// Prints the content of an array of `Step` structs.
func PrintSteps(steps []Step) {
	fmt.Println("\n\tSteps:")
	fmt.Printf("\t%15s%15s%15s\n", "name", "size (kb)", "records #")
	for _, step := range steps {
		fmt.Printf("\t%15v\n", step)
	}
}
