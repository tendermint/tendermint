//go:build boltdb
// +build boltdb

package db

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBoltDBNewBoltDB(t *testing.T) {
	name := fmt.Sprintf("test_%x", randStr(12))
	dir := os.TempDir()
	defer cleanupDBDir(dir, name)

	db, err := NewBoltDB(name, dir)
	require.NoError(t, err)
	db.Close()
}

func TestWithBoltDB(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "boltdb")

	db, err := NewBoltDB(path, "")
	require.NoError(t, err)

	t.Run("BoltDB", func(t *testing.T) { Run(t, db) })
}

func BenchmarkBoltDBRandomReadsWrites(b *testing.B) {
	name := fmt.Sprintf("test_%x", randStr(12))
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
