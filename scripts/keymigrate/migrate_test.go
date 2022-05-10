package keymigrate

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/google/orderedcode"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

func makeKey(t *testing.T, elems ...interface{}) []byte {
	t.Helper()
	out, err := orderedcode.Append([]byte{}, elems...)
	require.NoError(t, err)
	return out
}

func getLegacyPrefixKeys(val int) map[string][]byte {
	vstr := fmt.Sprintf("%02x", byte(val))
	return map[string][]byte{
		"Height":            []byte(fmt.Sprintf("H:%d", val)),
		"BlockPart":         []byte(fmt.Sprintf("P:%d:%d", val, val)),
		"BlockPartTwo":      []byte(fmt.Sprintf("P:%d:%d", val+2, val+val)),
		"BlockCommit":       []byte(fmt.Sprintf("C:%d", val)),
		"SeenCommit":        []byte(fmt.Sprintf("SC:%d", val)),
		"BlockHeight":       []byte(fmt.Sprintf("BH:%x", val)),
		"Validators":        []byte(fmt.Sprintf("validatorsKey:%d", val)),
		"ConsensusParams":   []byte(fmt.Sprintf("consensusParamsKey:%d", val)),
		"ABCIResponse":      []byte(fmt.Sprintf("abciResponsesKey:%d", val)),
		"State":             []byte("stateKey"),
		"CommittedEvidence": append([]byte{0x00}, []byte(fmt.Sprintf("%0.16X/%X", int64(val), []byte("committed")))...),
		"PendingEvidence":   append([]byte{0x01}, []byte(fmt.Sprintf("%0.16X/%X", int64(val), []byte("pending")))...),
		"LightBLock":        []byte(fmt.Sprintf("lb/foo/%020d", val)),
		"Size":              []byte("size"),
		"UserKey0":          []byte(fmt.Sprintf("foo/bar/%d/%d", val, val)),
		"UserKey1":          []byte(fmt.Sprintf("foo/bar/baz/%d/%d", val, val)),
		"TxHeight":          []byte(fmt.Sprintf("tx.height/%s/%d/%d", fmt.Sprint(val), val, val)),
		"TxHash": append(
			[]byte(strings.Repeat(vstr[:1], 16)),
			[]byte(strings.Repeat(vstr[1:], 16))...,
		),

		// Transaction hashes that could be mistaken for evidence keys.
		"TxHashMimic0": append([]byte{0}, []byte(strings.Repeat(vstr, 16)[:31])...),
		"TxHashMimic1": append([]byte{1}, []byte(strings.Repeat(vstr, 16)[:31])...),
	}
}

func getNewPrefixKeys(t *testing.T, val int) map[string][]byte {
	t.Helper()
	vstr := fmt.Sprintf("%02x", byte(val))
	return map[string][]byte{
		"Height":            makeKey(t, int64(0), int64(val)),
		"BlockPart":         makeKey(t, int64(1), int64(val), int64(val)),
		"BlockPartTwo":      makeKey(t, int64(1), int64(val+2), int64(val+val)),
		"BlockCommit":       makeKey(t, int64(2), int64(val)),
		"SeenCommit":        makeKey(t, int64(3), int64(val)),
		"BlockHeight":       makeKey(t, int64(4), int64(val)),
		"Validators":        makeKey(t, int64(5), int64(val)),
		"ConsensusParams":   makeKey(t, int64(6), int64(val)),
		"ABCIResponse":      makeKey(t, int64(7), int64(val)),
		"State":             makeKey(t, int64(8)),
		"CommittedEvidence": makeKey(t, int64(9), int64(val)),
		"PendingEvidence":   makeKey(t, int64(10), int64(val)),
		"LightBLock":        makeKey(t, int64(11), int64(val)),
		"Size":              makeKey(t, int64(12)),
		"UserKey0":          makeKey(t, "foo", "bar", int64(val), int64(val)),
		"UserKey1":          makeKey(t, "foo", "bar/baz", int64(val), int64(val)),
		"TxHeight":          makeKey(t, "tx.height", fmt.Sprint(val), int64(val), int64(val+2), int64(val+val)),
		"TxHash":            makeKey(t, "tx.hash", strings.Repeat(vstr, 16)),
		"TxHashMimic0":      makeKey(t, "tx.hash", "\x00"+strings.Repeat(vstr, 16)[:31]),
		"TxHashMimic1":      makeKey(t, "tx.hash", "\x01"+strings.Repeat(vstr, 16)[:31]),
	}
}

func getLegacyDatabase(t *testing.T) (int, dbm.DB) {
	db := dbm.NewMemDB()
	batch := db.NewBatch()
	ct := 0

	generated := []map[string][]byte{
		getLegacyPrefixKeys(8),
		getLegacyPrefixKeys(9001),
		getLegacyPrefixKeys(math.MaxInt32 << 1),
		getLegacyPrefixKeys(math.MaxInt64 - 8),
	}

	// populate database
	for _, km := range generated {
		for _, key := range km {
			ct++
			require.NoError(t, batch.Set(key, []byte(fmt.Sprintf(`{"value": %d}`, ct))))
		}
	}
	require.NoError(t, batch.WriteSync())
	require.NoError(t, batch.Close())
	return ct - (2 * len(generated)) + 2, db
}

func TestMigration(t *testing.T) {
	t.Run("Idempotency", func(t *testing.T) {
		// we want to make sure that the key space for new and
		// legacy keys are entirely non-overlapping.

		legacyPrefixes := getLegacyPrefixKeys(42)

		newPrefixes := getNewPrefixKeys(t, 42)

		require.Equal(t, len(legacyPrefixes), len(newPrefixes))

		t.Run("Legacy", func(t *testing.T) {
			for kind, le := range legacyPrefixes {
				require.True(t, checkKeyType(le).isLegacy(), kind)
			}
		})
		t.Run("New", func(t *testing.T) {
			for kind, ne := range newPrefixes {
				require.False(t, checkKeyType(ne).isLegacy(), kind)
			}
		})
		t.Run("Conversion", func(t *testing.T) {
			for kind, le := range legacyPrefixes {
				nk, err := migrateKey(le)
				require.NoError(t, err, kind)
				require.False(t, checkKeyType(nk).isLegacy(), kind)
			}
		})
		t.Run("Hashes", func(t *testing.T) {
			t.Run("NewKeysAreNotHashes", func(t *testing.T) {
				for _, key := range getNewPrefixKeys(t, 9001) {
					require.True(t, len(key) != 32)
				}
			})
			t.Run("ContrivedLegacyKeyDetection", func(t *testing.T) {
				// length 32: should appear to be a hash
				require.Equal(t, txHashKey, checkKeyType([]byte("xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx")))

				// length ≠ 32: should not appear to be a hash
				require.Equal(t, nonLegacyKey, checkKeyType([]byte("xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx--")))
				require.Equal(t, nonLegacyKey, checkKeyType([]byte("xxxxxxxxxxxxxxxxxxxxxxxxxxxxxx")))
			})
		})
	})
	t.Run("Migrations", func(t *testing.T) {
		t.Run("Errors", func(t *testing.T) {
			table := map[string][]byte{
				"Height":          []byte(fmt.Sprintf("H:%f", 4.22222)),
				"BlockPart":       []byte(fmt.Sprintf("P:%f", 4.22222)),
				"BlockPartTwo":    []byte(fmt.Sprintf("P:%d", 42)),
				"BlockPartThree":  []byte(fmt.Sprintf("P:%f:%f", 4.222, 8.444)),
				"BlockPartFour":   []byte(fmt.Sprintf("P:%d:%f", 4222, 8.444)),
				"BlockCommit":     []byte(fmt.Sprintf("C:%f", 4.22222)),
				"SeenCommit":      []byte(fmt.Sprintf("SC:%f", 4.22222)),
				"BlockHeight":     []byte(fmt.Sprintf("BH:%f", 4.22222)),
				"Validators":      []byte(fmt.Sprintf("validatorsKey:%f", 4.22222)),
				"ConsensusParams": []byte(fmt.Sprintf("consensusParamsKey:%f", 4.22222)),
				"ABCIResponse":    []byte(fmt.Sprintf("abciResponsesKey:%f", 4.22222)),
				"LightBlockShort": []byte(fmt.Sprintf("lb/foo/%010d", 42)),
				"LightBlockLong":  []byte("lb/foo/12345678910.1234567890"),
				"Invalid":         {0x03},
				"BadTXHeight0":    []byte(fmt.Sprintf("tx.height/%s/%f/%f", "boop", 4.4, 4.5)),
				"BadTXHeight1":    []byte(fmt.Sprintf("tx.height/%s/%f", "boop", 4.4)),
				"UserKey0":        []byte("foo/bar/1.3/3.4"),
				"UserKey1":        []byte("foo/bar/1/3.4"),
				"UserKey2":        []byte("foo/bar/baz/1/3.4"),
				"UserKey3":        []byte("foo/bar/baz/1.2/4"),
			}
			for kind, key := range table {
				out, err := migrateKey(key)
				require.Error(t, err, kind)
				require.Nil(t, out, kind)
			}
		})
		t.Run("Replacement", func(t *testing.T) {
			t.Run("MissingKey", func(t *testing.T) {
				db := dbm.NewMemDB()
				require.NoError(t, replaceKey(db, keyID("hi"), nil))
			})
			t.Run("ReplacementFails", func(t *testing.T) {
				db := dbm.NewMemDB()
				key := keyID("hi")
				require.NoError(t, db.Set(key, []byte("world")))
				require.Error(t, replaceKey(db, key, func(k keyID) (keyID, error) {
					return nil, errors.New("hi")
				}))
			})
			t.Run("KeyDisappears", func(t *testing.T) {
				db := dbm.NewMemDB()
				key := keyID("hi")
				require.NoError(t, db.Set(key, []byte("world")))
				require.Error(t, replaceKey(db, key, func(k keyID) (keyID, error) {
					require.NoError(t, db.Delete(key))
					return keyID("wat"), nil
				}))

				exists, err := db.Has(key)
				require.NoError(t, err)
				require.False(t, exists)

				exists, err = db.Has(keyID("wat"))
				require.NoError(t, err)
				require.False(t, exists)
			})
		})
	})
	t.Run("Integration", func(t *testing.T) {
		t.Run("KeyDiscovery", func(t *testing.T) {
			size, db := getLegacyDatabase(t)
			keys, err := getAllLegacyKeys(db)
			require.NoError(t, err)
			require.Equal(t, size, len(keys))
			legacyKeys := 0
			for _, k := range keys {
				if checkKeyType(k).isLegacy() {
					legacyKeys++
				}
			}
			require.Equal(t, size, legacyKeys)
		})
		t.Run("KeyIdempotency", func(t *testing.T) {
			for _, key := range getNewPrefixKeys(t, 84) {
				require.False(t, checkKeyType(key).isLegacy())
			}
		})
		t.Run("Migrate", func(t *testing.T) {
			_, db := getLegacyDatabase(t)

			ctx := context.Background()
			err := Migrate(ctx, db)
			require.NoError(t, err)
			keys, err := getAllLegacyKeys(db)
			require.NoError(t, err)
			require.Equal(t, 0, len(keys))

		})
	})
}
