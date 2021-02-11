package goleveldb

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/syndtr/goleveldb/leveldb/opt"
	"github.com/tendermint/tm-db/internal/dbtest"
)

func TestGoLevelDBNewDB(t *testing.T) {
	name := fmt.Sprintf("test_%x", dbtest.RandStr(12))
	defer dbtest.CleanupDBDir("", name)

	// Test we can't open the db twice for writing
	wr1, err := NewDB(name, "")
	require.Nil(t, err)
	_, err = NewDB(name, "")
	require.NotNil(t, err)
	wr1.Close() // Close the db to release the lock

	// Test we can open the db twice for reading only
	ro1, err := NewDBWithOpts(name, "", &opt.Options{ReadOnly: true})
	require.Nil(t, err)
	defer ro1.Close()
	ro2, err := NewDBWithOpts(name, "", &opt.Options{ReadOnly: true})
	require.Nil(t, err)
	defer ro2.Close()
}

func BenchmarkGoLevelDBRandomReadsWrites(b *testing.B) {
	name := fmt.Sprintf("test_%x", dbtest.RandStr(12))
	db, err := NewDB(name, "")
	if err != nil {
		b.Fatal(err)
	}
	defer func() {
		db.Close()
		dbtest.CleanupDBDir("", name)
	}()

	dbtest.BenchmarkRandomReadsWrites(b, db)
}
