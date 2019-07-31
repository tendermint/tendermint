package db

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/syndtr/goleveldb/leveldb/opt"
)

func TestGoLevelDBNewGoLevelDB(t *testing.T) {
	name := fmt.Sprintf("test_%x", randStr(12))
	defer cleanupDBDir("", name)

	// Test we can't open the db twice for writing
	wr1, err := NewGoLevelDB(name, "")
	require.Nil(t, err)
	_, err = NewGoLevelDB(name, "")
	require.NotNil(t, err)
	wr1.Close() // Close the db to release the lock

	// Test we can open the db twice for reading only
	ro1, err := NewGoLevelDBWithOpts(name, "", &opt.Options{ReadOnly: true})
	defer ro1.Close()
	require.Nil(t, err)
	ro2, err := NewGoLevelDBWithOpts(name, "", &opt.Options{ReadOnly: true})
	defer ro2.Close()
	require.Nil(t, err)
}

func BenchmarkGoLevelDBRandomReadsWrites(b *testing.B) {
	name := fmt.Sprintf("test_%x", randStr(12))
	db, err := NewGoLevelDB(name, "")
	if err != nil {
		b.Fatal(err)
	}
	defer func() {
		db.Close()
		cleanupDBDir("", name)
	}()

	benchmarkRandomReadsWrites(b, db)
}
