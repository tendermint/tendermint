package db

import (
	"fmt"
	"testing"

	"github.com/etcd-io/bbolt"
	"github.com/stretchr/testify/require"

	cmn "github.com/tendermint/tendermint/libs/common"
)

func TestBoltDBNewBoltDB(t *testing.T) {
	name := fmt.Sprintf("test_%x", cmn.RandStr(12))
	defer cleanupDBDir("", name)

	// Test we can't open the db twice for writing
	wr1, err := NewBoltDB(name, "")
	require.Nil(t, err)
	_, err = NewBoltDB(name, "")
	require.NotNil(t, err)
	wr1.Close() // Close the db to release the lock

	// Test we can open the db twice for reading only
	ro1, err := NewBoltDBWithOpts(name, "", &bbolt.Options{ReadOnly: true})
	defer ro1.Close()
	require.Nil(t, err)
	ro2, err := NewBoltDBWithOpts(name, "", &bbolt.Options{ReadOnly: true})
	defer ro2.Close()
	require.Nil(t, err)
}

func TestBoltDBGetSet(t *testing.T) {
	// TODO
}

func BenchmarkBoltDBRandomReadsWrites(b *testing.B) {
	name := fmt.Sprintf("test_%x", cmn.RandStr(12))
	db, err := NewBoltDB(name, "")
	if err != nil {
		b.Fatal(err)
	}
	defer func() {
		db.Close()
		cleanupDBDir("", name)
	}()

	benchmarkRandomReadsWrites(b, db)
}
