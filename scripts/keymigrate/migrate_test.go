package keymigrate

import (
	"errors"
	"fmt"
	"math"
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
	return map[string][]byte{
		"Height":            []byte(fmt.Sprintf("H:%d", val)),
		"BlockPart":         []byte(fmt.Sprintf("P:%d", val)),
		"BlockCommit":       []byte(fmt.Sprintf("C:%d", val)),
		"SeenCommit":        []byte(fmt.Sprintf("SC:%d", val)),
		"BlockHeight":       []byte(fmt.Sprintf("BH:%d", val)),
		"Validators":        []byte(fmt.Sprintf("validatorsKey:%d", val)),
		"ConsensusParams":   []byte(fmt.Sprintf("consensusParamsKey:%d", val)),
		"ABCIResponse":      []byte(fmt.Sprintf("abciResponsesKey:%d", val)),
		"State":             []byte("stateKey"),
		"CommittedEvidence": []byte{0x00},
		"PendingEvidence":   []byte{0x01},
		"LightBLock":        []byte(fmt.Sprintf("lb/foo/%020d", val)),
		"Size":              []byte("size"),
	}
}

func getNewPrefixeKeys(t *testing.T, val int) map[string][]byte {
	t.Helper()
	return map[string][]byte{
		"Height":            makeKey(t, int64(0), int64(val)),
		"BlockPart":         makeKey(t, int64(1), int64(val)),
		"BlockCommit":       makeKey(t, int64(2), int64(val)),
		"SeenCommit":        makeKey(t, int64(3), int64(val)),
		"BlockHeight":       makeKey(t, int64(4), int64(val)),
		"Validators":        makeKey(t, int64(5), int64(val)),
		"ConsensusParams":   makeKey(t, int64(6), int64(val)),
		"ABCIResponse":      makeKey(t, int64(7), int64(val)),
		"State":             makeKey(t, int64(8), int64(val)),
		"CommittedEvidence": makeKey(t, int64(9), int64(val)),
		"PendingEvidence":   makeKey(t, int64(10), int64(val)),
		"LightBLock":        makeKey(t, int64(11), int64(val)),
		"Size":              makeKey(t, int64(12)),
	}
}

func getLegacyDatabase(t *testing.T) (int, dbm.DB) {
	db := dbm.NewMemDB()
	batch := db.NewBatch()
	ct := 0

	generated := []map[string][]byte{
		getLegacyPrefixKeys(0),
		getLegacyPrefixKeys(9001),
		getLegacyPrefixKeys(math.MaxInt32),
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
	return ct - (3 * len(generated)), db
}

func TestMigration(t *testing.T) {
	t.Run("Idempotency", func(t *testing.T) {
		// we want to make sure that the key space for new and
		// legacy keys are entirely non-overlapping.

		legacyPrefixes := getLegacyPrefixKeys(42)

		newPrefixes := getNewPrefixeKeys(t, 42)

		require.Equal(t, len(legacyPrefixes), len(newPrefixes))

		t.Run("LegacyEvidence", func(t *testing.T) {
			for kind, le := range legacyPrefixes {
				t.Run(kind, func(t *testing.T) {
					require.True(t, keyIsLegacy(le))
				})
			}
		})
		t.Run("NewEvidence", func(t *testing.T) {
			for kind, ne := range newPrefixes {
				t.Run(kind, func(t *testing.T) {
					require.False(t, keyIsLegacy(ne))
				})
			}
		})
		t.Run("Conversion", func(t *testing.T) {
			for kind, le := range legacyPrefixes {
				t.Run(kind, func(t *testing.T) {
					nk, err := migarateKey(le)
					require.NoError(t, err)
					require.False(t, keyIsLegacy(nk))
				})
			}
		})
	})
	t.Run("Migrations", func(t *testing.T) {
		t.Run("Errors", func(t *testing.T) {
			table := map[string][]byte{
				"Height":          []byte(fmt.Sprintf("H:%f", 4.22222)),
				"BlockPart":       []byte(fmt.Sprintf("P:%f", 4.22222)),
				"BlockCommit":     []byte(fmt.Sprintf("C:%f", 4.22222)),
				"SeenCommit":      []byte(fmt.Sprintf("SC:%f", 4.22222)),
				"BlockHeight":     []byte(fmt.Sprintf("BH:%f", 4.22222)),
				"Validators":      []byte(fmt.Sprintf("validatorsKey:%f", 4.22222)),
				"ConsensusParams": []byte(fmt.Sprintf("consensusParamsKey:%f", 4.22222)),
				"ABCIResponse":    []byte(fmt.Sprintf("abciResponsesKey:%f", 4.22222)),
				"LightBlockShort": []byte(fmt.Sprintf("lb/foo/%010d", 42)),
				"LightBlockLong":  []byte("lb/foo/12345678910.1234567890"),
				"Invalid":         []byte{0x03},
			}
			for kind, key := range table {
				t.Run(kind, func(t *testing.T) {
					out, err := migarateKey(key)
					require.Error(t, err)
					require.Nil(t, out)
				})
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
			t.Run("KeyDisapears", func(t *testing.T) {
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
				if keyIsLegacy(k) {
					legacyKeys++
				}
			}
			require.Equal(t, size, legacyKeys)
		})
		t.Run("KeyIdempotency", func(t *testing.T) {
			for _, key := range getNewPrefixeKeys(t, 84) {
				require.False(t, keyIsLegacy(key))
			}
		})
		t.Run("ChannelConversion", func(t *testing.T) {
			ch := makeKeyChan([]keyID{
				makeKey(t, "abc", int64(2), int64(42)),
				makeKey(t, int64(42)),
			})
			count := 0
			for range ch {
				count++
			}
			require.Equal(t, 2, count)
		})
		t.Run("Migrate", func(t *testing.T) {
			_, db := getLegacyDatabase(t)

			err := Honk(db)
			require.NoError(t, err)
			keys, err := getAllLegacyKeys(db)
			require.NoError(t, err)
			require.Equal(t, 0, len(keys))
		})
	})
}
