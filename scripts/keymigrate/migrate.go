// Package keymigrate translates all legacy formatted keys to their
// new components.
//
// The key migration operation as implemented provides a potential
// model for database migration operations. Crucially, the migration
// as implemented does not depend on any tendermint code.
package keymigrate

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math/rand"
	"runtime"
	"strconv"

	"github.com/creachadair/taskgroup"
	"github.com/google/orderedcode"
	dbm "github.com/tendermint/tm-db"
)

type (
	keyID       []byte
	migrateFunc func(keyID) (keyID, error)
)

func getAllLegacyKeys(db dbm.DB, storeName string) ([]keyID, error) {
	var out []keyID

	iter, err := db.Iterator(nil, nil)
	if err != nil {
		return nil, err
	}

	for ; iter.Valid(); iter.Next() {
		k := iter.Key()

		// make sure it's a key that we'd expect to see in
		// this database, with a legacy format, and skip all
		// other keys, to make it safe to resume the
		// migration.
		kt, err := checkKeyType(k, storeName)
		if err != nil {
			return nil, err
		}

		if !kt.isLegacy() {
			continue
		}

		// Make an explicit copy, since not all tm-db backends do.
		out = append(out, []byte(string(k)))
	}

	if err = iter.Error(); err != nil {
		return nil, err
	}

	if err = iter.Close(); err != nil {
		return nil, err
	}

	return out, nil
}

// keyType is an enumeration for the structural type of a key.
type keyType int

func (t keyType) isLegacy() bool { return t != nonLegacyKey }

const (
	nonLegacyKey keyType = iota // non-legacy key (presumed already converted)
	consensusParamsKey
	abciResponsesKey
	validatorsKey
	stateStoreKey        // state storage record
	blockMetaKey         // H:
	blockPartKey         // P:
	commitKey            // C:
	seenCommitKey        // SC:
	blockHashKey         // BH:
	lightSizeKey         // size
	lightBlockKey        // lb/
	evidenceCommittedKey // \x00
	evidencePendingKey   // \x01
	txHeightKey          // tx.height/... (special case)
	abciEventKey         // name/value/height/index
	txHashKey            // 32-byte transaction hash (unprefixed)
)

var prefixes = []struct {
	prefix []byte
	ktype  keyType
	check  func(keyID) bool
}{
	{[]byte(legacyConsensusParamsPrefix), consensusParamsKey, nil},
	{[]byte(legacyAbciResponsePrefix), abciResponsesKey, nil},
	{[]byte(legacyValidatorPrefix), validatorsKey, nil},
	{[]byte(legacyStateKeyPrefix), stateStoreKey, nil},
	{[]byte(legacyBlockMetaPrefix), blockMetaKey, nil},
	{[]byte(legacyBlockPartPrefix), blockPartKey, nil},
	{[]byte(legacyCommitPrefix), commitKey, nil},
	{[]byte(legacySeenCommitPrefix), seenCommitKey, nil},
	{[]byte(legacyBlockHashPrefix), blockHashKey, nil},
	{[]byte(legacyLightSizePrefix), lightSizeKey, nil},
	{[]byte(legacyLightBlockPrefix), lightBlockKey, nil},
	{[]byte(legacyEvidenceComittedPrefix), evidenceCommittedKey, checkEvidenceKey},
	{[]byte(legacyEvidencePendingPrefix), evidencePendingKey, checkEvidenceKey},
}

const (
	legacyConsensusParamsPrefix  = "consensusParamsKey:"
	legacyAbciResponsePrefix     = "abciResponsesKey:"
	legacyValidatorPrefix        = "validatorsKey:"
	legacyStateKeyPrefix         = "stateKey"
	legacyBlockMetaPrefix        = "H:"
	legacyBlockPartPrefix        = "P:"
	legacyCommitPrefix           = "C:"
	legacySeenCommitPrefix       = "SC:"
	legacyBlockHashPrefix        = "BH:"
	legacyLightSizePrefix        = "size"
	legacyLightBlockPrefix       = "lb/"
	legacyEvidenceComittedPrefix = "\x00"
	legacyEvidencePendingPrefix  = "\x01"
)

type migrationDefinition struct {
	name      string
	storeName string
	prefix    []byte
	ktype     keyType
	check     func(keyID) bool
	transform migrateFunc
}

var migrations = []migrationDefinition{
	{
		name:      "consensus-params",
		storeName: "state",
		prefix:    []byte(legacyConsensusParamsPrefix),
		ktype:     consensusParamsKey,
		transform: func(key keyID) (keyID, error) {
			val, err := strconv.Atoi(string(key[19:]))
			if err != nil {
				return nil, err
			}

			return orderedcode.Append(nil, int64(6), int64(val))
		},
	},
	{
		name:      "abci-responses",
		storeName: "state",
		prefix:    []byte(legacyAbciResponsePrefix),
		ktype:     abciResponsesKey,
		transform: func(key keyID) (keyID, error) {
			val, err := strconv.Atoi(string(key[17:]))
			if err != nil {
				return nil, err
			}

			return orderedcode.Append(nil, int64(7), int64(val))
		},
	},
	{
		name:      "validators",
		storeName: "state",
		prefix:    []byte(legacyValidatorPrefix),
		ktype:     validatorsKey,
		transform: func(key keyID) (keyID, error) {
			val, err := strconv.Atoi(string(key[14:]))
			if err != nil {
				return nil, err
			}

			return orderedcode.Append(nil, int64(5), int64(val))
		},
	},
	{
		name:      "tendermint-state",
		storeName: "state",
		prefix:    []byte(legacyStateKeyPrefix),
		ktype:     stateStoreKey,
		transform: func(key keyID) (keyID, error) {
			return orderedcode.Append(nil, int64(8))
		},
	},
	{
		name:      "block-meta",
		storeName: "blockstore",
		prefix:    []byte(legacyBlockMetaPrefix),
		ktype:     blockMetaKey,
		transform: func(key keyID) (keyID, error) {
			val, err := strconv.Atoi(string(key[2:]))
			if err != nil {
				return nil, err
			}

			return orderedcode.Append(nil, int64(0), int64(val))
		},
	},
	{
		name:      "block-part",
		storeName: "blockstore",
		prefix:    []byte(legacyBlockPartPrefix),
		ktype:     blockPartKey,
		transform: func(key keyID) (keyID, error) {
			parts := bytes.Split(key[2:], []byte(":"))
			if len(parts) != 2 {
				return nil, fmt.Errorf("block parts key has %d rather than 2 components",
					len(parts))
			}
			valOne, err := strconv.Atoi(string(parts[0]))
			if err != nil {
				return nil, err
			}

			valTwo, err := strconv.Atoi(string(parts[1]))
			if err != nil {
				return nil, err
			}

			return orderedcode.Append(nil, int64(1), int64(valOne), int64(valTwo))
		},
	},
	{
		name:      "commit",
		storeName: "blockstore",
		prefix:    []byte(legacyCommitPrefix),
		ktype:     commitKey,
		transform: func(key keyID) (keyID, error) {
			val, err := strconv.Atoi(string(key[2:]))
			if err != nil {
				return nil, err
			}

			return orderedcode.Append(nil, int64(2), int64(val))
		},
	},
	{
		name:      "seen-commit",
		storeName: "blockstore",
		prefix:    []byte(legacySeenCommitPrefix),
		ktype:     seenCommitKey,
		transform: func(key keyID) (keyID, error) {
			val, err := strconv.Atoi(string(key[3:]))
			if err != nil {
				return nil, err
			}

			return orderedcode.Append(nil, int64(3), int64(val))
		},
	},
	{
		name:      "block-hash",
		storeName: "blockstore",
		prefix:    []byte(legacyBlockHashPrefix),
		ktype:     blockHashKey,
		transform: func(key keyID) (keyID, error) {
			hash := string(key[3:])
			if len(hash)%2 == 1 {
				hash = "0" + hash
			}
			val, err := hex.DecodeString(hash)
			if err != nil {
				return nil, err
			}

			return orderedcode.Append(nil, int64(4), string(val))
		},
	},
	{
		name:      "light-size",
		storeName: "light",
		prefix:    []byte(legacyLightSizePrefix),
		ktype:     lightSizeKey,
		transform: func(key keyID) (keyID, error) {
			return orderedcode.Append(nil, int64(12))
		},
	},
	{
		name:      "light-block",
		storeName: "light",
		prefix:    []byte(legacyLightBlockPrefix),
		ktype:     lightBlockKey,
		transform: func(key keyID) (keyID, error) {
			if len(key) < 24 {
				return nil, fmt.Errorf("light block evidence %q in invalid format", string(key))
			}

			val, err := strconv.Atoi(string(key[len(key)-20:]))
			if err != nil {
				return nil, err
			}

			return orderedcode.Append(nil, int64(11), int64(val))
		},
	},
	{
		name:      "evidence-pending",
		storeName: "evidence",
		prefix:    []byte(legacyEvidencePendingPrefix),
		ktype:     evidencePendingKey,
		transform: func(key keyID) (keyID, error) {
			return convertEvidence(key, 10)
		},
	},
	{
		name:      "evidence-committed",
		storeName: "evidence",
		prefix:    []byte(legacyEvidenceComittedPrefix),
		ktype:     evidenceCommittedKey,
		transform: func(key keyID) (keyID, error) {
			return convertEvidence(key, 9)
		},
	},
	{
		name:      "event-tx",
		storeName: "tx_index",
		prefix:    nil,
		ktype:     txHeightKey,
		transform: func(key keyID) (keyID, error) {
			parts := bytes.Split(key, []byte("/"))
			if len(parts) != 4 {
				return nil, fmt.Errorf("key has %d parts rather than 4", len(parts))
			}
			parts = parts[1:] // drop prefix

			elems := make([]interface{}, 0, len(parts)+1)
			elems = append(elems, "tx.height")

			for idx, pt := range parts {
				val, err := strconv.Atoi(string(pt))
				if err != nil {
					return nil, err
				}
				if idx == 0 {
					elems = append(elems, fmt.Sprintf("%d", val))
				} else {
					elems = append(elems, int64(val))
				}
			}

			return orderedcode.Append(nil, elems...)
		},
	},
	{
		name:      "event-abci",
		storeName: "tx_index",
		prefix:    nil,
		ktype:     abciEventKey,
		transform: func(key keyID) (keyID, error) {
			parts := bytes.Split(key, []byte("/"))

			elems := make([]interface{}, 0, 4)
			if len(parts) == 4 {
				elems = append(elems, string(parts[0]), string(parts[1]))

				val, err := strconv.Atoi(string(parts[2]))
				if err != nil {
					return nil, err
				}
				elems = append(elems, int64(val))

				val2, err := strconv.Atoi(string(parts[3]))
				if err != nil {
					return nil, err
				}
				elems = append(elems, int64(val2))
			} else {
				elems = append(elems, string(parts[0]))
				parts = parts[1:]

				val, err := strconv.Atoi(string(parts[len(parts)-1]))
				if err != nil {
					return nil, err
				}

				val2, err := strconv.Atoi(string(parts[len(parts)-2]))
				if err != nil {
					return nil, err
				}

				appKey := bytes.Join(parts[:len(parts)-3], []byte("/"))
				elems = append(elems, string(appKey), int64(val), int64(val2))
			}

			return orderedcode.Append(nil, elems...)
		},
	},
	{
		name:      "event-tx-hash",
		storeName: "tx_index",
		prefix:    nil,
		ktype:     txHashKey,
		transform: func(key keyID) (keyID, error) {
			return orderedcode.Append(nil, "tx.hash", string(key))
		},
	},
}

// checkKeyType classifies a candidate key based on its structure.
func checkKeyType(key keyID, storeName string) (keyType, error) {
	var migrations []migrationDefinition
	for _, m := range migrations {
		if m.storeName != storeName {
			continue
		}
		if m.prefix == nil && storeName == "tx_index" {
			// A legacy event key has the form:
			//
			//    <name> / <value> / <height> / <index>
			//
			// Transaction hashes are stored as a raw binary hash with no prefix.
			//
			// Note, though, that nothing prevents event names or values from containing
			// additional "/" separators, so the parse has to be forgiving.
			parts := bytes.Split(key, []byte("/"))
			if len(parts) >= 4 {
				// Special case for tx.height.
				if len(parts) == 4 && bytes.Equal(parts[0], []byte("tx.height")) {
					return txHeightKey, nil
				}

				// The name cannot be empty, but we don't know where the name ends and
				// the value begins, so insist that there be something.
				var n int
				for _, part := range parts[:len(parts)-2] {
					n += len(part)
				}
				// Check whether the last two fields could be .../height/index.
				if n > 0 && isDecimal(parts[len(parts)-1]) && isDecimal(parts[len(parts)-2]) {
					return abciEventKey, nil
				}
			}

			// If we get here, it's not an event key. Treat it as a hash if it is the
			// right length. Note that it IS possible this could collide with the
			// translation of some other key (though not a hash, since encoded hashes
			// will be longer). The chance of that is small, but there is nothing we can
			// do to detect it.
			if len(key) == 32 {
				return txHashKey, nil
			}
		} else if bytes.HasPrefix(key, m.prefix) {
			if m.check == nil || m.check(key) {
				return m.ktype, nil
			}
			// we have an expected legacy prefix but that
			// didn't pass the check. This probably means
			// the evidence data is currupt (based on the
			// defined migrations) best to error here.
			return -1, fmt.Errorf("in store %q, key %q exists but is not a valid key of type %q", storeName, key, m.ktype)
		}
		// if we get here, the key in question is either
		// migrated or of a different type. We can't break
		// here because there are more than one key type in a
		// specific database, so we have to keep iterating.
	}
	// if we've looked at every migration and not identified a key
	// type, then the key has been migrated *or* we (possibly, but
	// very unlikely have data that is in the wrong place or the
	// sign of corruption.) In either case we should not attempt
	// more migrations at this point

	return nonLegacyKey, nil
}

// isDecimal reports whether buf is a non-empty sequence of Unicode decimal
// digits.
func isDecimal(buf []byte) bool {
	for _, c := range buf {
		if c < '0' || c > '9' {
			return false
		}
	}
	return len(buf) != 0
}

func migrateKey(key keyID, storeName string) (keyID, error) {
	kt, err := checkKeyType(key, storeName)
	if err != nil {
		return nil, err
	}
	for _, migration := range migrations {
		if migration.storeName != storeName {
			continue
		}
		if kt == migration.ktype {
			return migration.transform(key)
		}
	}

	return nil, fmt.Errorf("key %q is in the wrong format", string(key))
}

func convertEvidence(key keyID, newPrefix int64) ([]byte, error) {
	parts := bytes.Split(key[1:], []byte("/"))
	if len(parts) != 2 {
		return nil, fmt.Errorf("evidence key is malformed with %d parts not 2",
			len(parts))
	}

	hb, err := hex.DecodeString(string(parts[0]))
	if err != nil {
		return nil, err
	}

	evidenceHash, err := hex.DecodeString(string(parts[1]))
	if err != nil {
		return nil, err
	}

	return orderedcode.Append(nil, newPrefix, binary.BigEndian.Uint64(hb), string(evidenceHash))
}

// checkEvidenceKey reports whether a candidate key with one of the legacy
// evidence prefixes has the correct structure for a legacy evidence key.
//
// This check is needed because transaction hashes are stored without a prefix,
// so checking the one-byte prefix alone is not enough to distinguish them.
// Legacy evidence keys are suffixed with a string of the format:
//
//     "%0.16X/%X"
//
// where the first element is the height and the second is the hash.  Thus, we
// check
func checkEvidenceKey(key keyID) bool {
	parts := bytes.SplitN(key[1:], []byte("/"), 2)
	if len(parts) != 2 || len(parts[0]) != 16 || !isHex(parts[0]) || !isHex(parts[1]) {
		return false
	}
	return true
}

func isHex(data []byte) bool {
	for _, b := range data {
		if ('0' <= b && b <= '9') || ('a' <= b && b <= 'f') || ('A' <= b && b <= 'F') {
			continue
		}
		return false
	}
	return len(data) != 0
}

func getMigrationFunc(storeName string, key keyID) (*migrationDefinition, error) {
	for idx := range migrations {
		migration := migrations[idx]

		if migration.storeName == storeName {
			if migration.prefix == nil {
				return &migration, nil
			}
			if bytes.HasPrefix(migration.prefix, key) {
				return &migration, nil
			}
		}
	}
	return nil, fmt.Errorf("no migration defined for data store %q and key %q", storeName, key)
}

func replaceKey(db dbm.DB, storeName string, key keyID) error {
	migration, err := getMigrationFunc(storeName, key)
	if err != nil {
		return err
	}
	gooseFn := migration.transform

	exists, err := db.Has(key)
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}

	newKey, err := gooseFn(key)
	if err != nil {
		return err
	}

	val, err := db.Get(key)
	if err != nil {
		return err
	}

	batch := db.NewBatch()

	if err = batch.Set(newKey, val); err != nil {
		return err
	}
	if err = batch.Delete(key); err != nil {
		return err
	}

	// 10% of the time, force a write to disk, but mostly don't,
	// because it's faster.
	if rand.Intn(100)%10 == 0 { // nolint:gosec
		if err = batch.WriteSync(); err != nil {
			return err
		}
	} else {
		if err = batch.Write(); err != nil {
			return err
		}
	}

	if err = batch.Close(); err != nil {
		return err
	}

	return nil
}

// Migrate converts all legacy key formats to new key formats. The
// operation is idempotent, so it's safe to resume a failed
// operation. The operation is somewhat parallelized, relying on the
// concurrency safety of the underlying databases.
//
// Migrate has "continue on error" semantics and will iterate through
// all legacy keys attempt to migrate them, and will collect all
// errors and will return only at the end of the operation.
//
// The context allows for a safe termination of the operation
// (e.g connected to a singal handler,) to abort the operation
// in-between migration operations.
func Migrate(ctx context.Context, storeName string, db dbm.DB) error {
	keys, err := getAllLegacyKeys(db, storeName)
	if err != nil {
		return err
	}

	var errs []string
	g, start := taskgroup.New(func(err error) error {
		errs = append(errs, err.Error())
		return err
	}).Limit(runtime.NumCPU())

	for _, key := range keys {
		key := key
		start(func() error {
			if err := ctx.Err(); err != nil {
				return err
			}
			return replaceKey(db, storeName, key)
		})
	}
	if g.Wait() != nil {
		return fmt.Errorf("encountered errors during migration: %q", errs)
	}
	return nil
}
