//go:build rocksdb
// +build rocksdb

package db

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRocksDBBackend(t *testing.T) {
	name := fmt.Sprintf("test_%x", randStr(12))
	dir := os.TempDir()
	db, err := NewDB(name, RocksDBBackend, dir)
	require.NoError(t, err)
	defer cleanupDBDir(dir, name)

	_, ok := db.(*RocksDB)
	assert.True(t, ok)
}

func TestWithRocksDB(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "rocksdb")

	db, err := NewRocksDB(path, "")
	require.NoError(t, err)

	t.Run("RocksDB", func(t *testing.T) { Run(t, db) })
}

func TestRocksDBStats(t *testing.T) {
	name := fmt.Sprintf("test_%x", randStr(12))
	dir := os.TempDir()
	db, err := NewDB(name, RocksDBBackend, dir)
	require.NoError(t, err)
	defer cleanupDBDir(dir, name)

	assert.NotEmpty(t, db.Stats())
}

// TODO: Add tests for rocksdb
